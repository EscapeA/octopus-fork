package apitoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/user"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// Agent/机器令牌：给脚本 / agent / CI 用的管理面凭证。
//
// 设计要点：
//   - 明文只在创建时返回一次，库里只存 SHA-256 摘要（设置列表/令牌列表接口对所有角色可见，
//     存摘要意味着即便被读走也无法反推出可用令牌）；
//   - 令牌绑定一个已存在用户，鉴权按该用户角色走 RBAC，审计日志记该用户名
//     （「谁在操作」因此能区分机器身份与人类管理员）；
//   - 支持有效期、吊销、最后使用时间/IP；吊销即时生效，无需重启。
const (
	// TokenPrefix 让机器令牌在日志/报错里一眼可辨，且与转发用 API Key（sk-octopus-*）互不混淆。
	TokenPrefix       = "ok-agent-"
	tokenRandomLength = 48
	tokenKeyChars     = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	tokenPrefixKeep   = 16 // 展示前缀长度（含 TokenPrefix）

	// lastUsedThrottleSeconds 限制「最后使用时间」的写库频率：管理面 QPS 低，
	// 但脚本可能连打请求，逐请求写库没必要（WAL 写放大）。
	lastUsedThrottleSeconds = 60
)

// Generate 生成一枚新的机器令牌。
func Generate() (string, error) {
	b := make([]byte, tokenRandomLength)
	maxI := big.NewInt(int64(len(tokenKeyChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, maxI)
		if err != nil {
			return "", fmt.Errorf("generate api token: %w", err)
		}
		b[i] = tokenKeyChars[n.Int64()]
	}
	return TokenPrefix + string(b), nil
}

// Hash 返回令牌的 SHA-256 十六进制摘要（小写），两端空白被忽略。
func Hash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// DisplayPrefix 返回可用于界面识别的令牌前缀（不足以复用为令牌）。
func DisplayPrefix(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= tokenPrefixKeep {
		return token
	}
	return token[:tokenPrefixKeep]
}

// Create 创建令牌并返回一次性明文。用户名必须对应已存在的用户。
func Create(ctx context.Context, req model.APITokenCreateRequest, creatorID uint) (string, model.APIToken, error) {
	empty := model.APIToken{}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", empty, fmt.Errorf("token name is required")
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return "", empty, fmt.Errorf("bound username is required")
	}
	if req.ExpiresDays < 0 {
		return "", empty, fmt.Errorf("expires_days must be greater than or equal to 0")
	}

	boundUser, err := user.GetByUsername(username, ctx)
	if err != nil {
		return "", empty, fmt.Errorf("bound user does not exist: %s", username)
	}

	token, err := Generate()
	if err != nil {
		return "", empty, err
	}

	expiresAt := int64(0)
	if req.ExpiresDays > 0 {
		expiresAt = time.Now().AddDate(0, 0, req.ExpiresDays).Unix()
	}

	row := model.APIToken{
		Name:      name,
		TokenHash: Hash(token),
		Prefix:    DisplayPrefix(token),
		UserID:    boundUser.ID,
		Username:  boundUser.Username,
		ExpiresAt: expiresAt,
		CreatedBy: creatorID,
	}
	if err := db.GetDB().WithContext(ctx).Create(&row).Error; err != nil {
		return "", empty, fmt.Errorf("failed to create api token: %w", err)
	}
	return token, row, nil
}

// Authenticate 校验候选凭证是否为有效令牌（未吊销、未过期）。
// 通过时返回令牌行，供调用方按绑定用户鉴权。
func Authenticate(candidate string, clientIP string) (model.APIToken, bool) {
	if !strings.HasPrefix(candidate, TokenPrefix) {
		return model.APIToken{}, false
	}
	var row model.APIToken
	if err := db.GetDB().Where("token_hash = ?", Hash(candidate)).First(&row).Error; err != nil {
		return model.APIToken{}, false
	}
	now := time.Now().Unix()
	if row.RevokedAt > 0 {
		return model.APIToken{}, false
	}
	if row.ExpiresAt > 0 && row.ExpiresAt <= now {
		return model.APIToken{}, false
	}
	touchLastUsed(row, clientIP, now)
	return row, true
}

// touchLastUsed 以 1 分钟粒度写回最后使用时间/IP（失败只记日志，不影响鉴权）。
func touchLastUsed(row model.APIToken, clientIP string, now int64) {
	if now-row.LastUsedAt < lastUsedThrottleSeconds && row.LastUsedIP == clientIP {
		return
	}
	if err := db.GetDB().Model(&model.APIToken{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{"last_used_at": now, "last_used_ip": clientIP}).Error; err != nil {
		log.Warnf("update api token last used failed: %v", err)
	}
}

// List 返回全部令牌视图（含绑定用户是否仍存在）。
func List(ctx context.Context) ([]model.APITokenView, error) {
	var rows []model.APIToken
	if err := db.GetDB().WithContext(ctx).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list api tokens: %w", err)
	}

	var users []model.User
	if err := db.GetDB().WithContext(ctx).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	existing := make(map[uint]string, len(users))
	for _, item := range users {
		existing[item.ID] = item.Username
	}

	views := make([]model.APITokenView, 0, len(rows))
	for _, row := range rows {
		username, ok := existing[row.UserID]
		if !ok {
			username = row.Username
		}
		views = append(views, model.APITokenView{
			ID:         row.ID,
			Name:       row.Name,
			Prefix:     row.Prefix,
			UserID:     row.UserID,
			Username:   username,
			UserExists: ok,
			ExpiresAt:  row.ExpiresAt,
			LastUsedAt: row.LastUsedAt,
			LastUsedIP: row.LastUsedIP,
			RevokedAt:  row.RevokedAt,
			CreatedBy:  row.CreatedBy,
			CreatedAt:  row.CreatedAt,
		})
	}
	return views, nil
}

// Revoke 吊销令牌（幂等；保留行以便界面显示吊销时间）。
func Revoke(ctx context.Context, id uint) error {
	result := db.GetDB().WithContext(ctx).Model(&model.APIToken{}).
		Where("id = ? AND revoked_at = 0", id).
		Update("revoked_at", time.Now().Unix())
	if result.Error != nil {
		return fmt.Errorf("failed to revoke api token: %w", result.Error)
	}
	return nil
}

// Delete 硬删除令牌（用于清理已吊销的记录）。
func Delete(ctx context.Context, id uint) error {
	if err := db.GetDB().WithContext(ctx).Where("id = ?", id).Delete(&model.APIToken{}).Error; err != nil {
		return fmt.Errorf("failed to delete api token: %w", err)
	}
	return nil
}
