package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// Agent 令牌是给机器身份（脚本 / agent / CI）用的管理面凭证，让自动化不必再用
// 管理员账号密码登录换 JWT。设计要点：
//   - 明文只在生成时返回一次，设置表里只存 SHA-256 摘要——设置列表接口对所有角色
//     可见，存摘要意味着即便被读走也无法反推出可用令牌；
//   - 令牌绑定一个已存在的用户，鉴权按该用户角色走 RBAC，审计日志记该用户名，
//     因此「谁在操作」可区分（机器身份 vs 人类管理员）；
//   - 开关/轮换走设置键，即时生效、无需重启，吊销只需清空摘要。
const (
	// AgentTokenPrefix 让 Agent 令牌在日志/报错里一眼可辨，且与转发用 API Key
	// （sk-octopus-*）互不混淆。
	AgentTokenPrefix       = "ok-agent-"
	agentTokenRandomLength = 48
	agentTokenKeyChars     = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	agentTokenPrefixKeep   = 16 // 展示前缀长度（含 AgentTokenPrefix）
)

// AgentTokenStatus 是可安全外发的令牌状态：不含摘要，也不含明文。
type AgentTokenStatus struct {
	Enabled   bool   `json:"enabled"`
	TokenSet  bool   `json:"token_set"`
	Username  string `json:"username"`
	Prefix    string `json:"prefix"`
	CreatedAt string `json:"created_at"`
}

// GenerateAgentToken 生成一枚新的 Agent 令牌。
func GenerateAgentToken() (string, error) {
	b := make([]byte, agentTokenRandomLength)
	maxI := big.NewInt(int64(len(agentTokenKeyChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, maxI)
		if err != nil {
			return "", fmt.Errorf("generate agent token: %w", err)
		}
		b[i] = agentTokenKeyChars[n.Int64()]
	}
	return AgentTokenPrefix + string(b), nil
}

// HashAgentToken 返回令牌的 SHA-256 十六进制摘要（小写），两端空白被忽略。
func HashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// AgentTokenDisplayPrefix 返回可用于界面识别的令牌前缀（不足以复用为令牌）。
func AgentTokenDisplayPrefix(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= agentTokenPrefixKeep {
		return token
	}
	return token[:agentTokenPrefixKeep]
}

// VerifyAgentToken 校验候选凭证是否为启用中的 Agent 令牌，
// 返回 (是否通过, 绑定的用户名)。
func VerifyAgentToken(candidate string) (bool, string) {
	if !strings.HasPrefix(candidate, AgentTokenPrefix) {
		return false, ""
	}
	if !agentTokenEnabled() {
		return false, ""
	}
	storedHash, err := setting.GetString(model.SettingKeyAgentAPITokenHash)
	if err != nil {
		return false, ""
	}
	storedHash = strings.ToLower(strings.TrimSpace(storedHash))
	if storedHash == "" {
		return false, ""
	}
	username, err := setting.GetString(model.SettingKeyAgentAPITokenUsername)
	if err != nil {
		return false, ""
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return false, ""
	}
	if subtle.ConstantTimeCompare([]byte(HashAgentToken(candidate)), []byte(storedHash)) != 1 {
		return false, ""
	}
	return true, username
}

func agentTokenEnabled() bool {
	value, err := setting.GetString(model.SettingKeyAgentAPITokenEnabled)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

// GetAgentTokenStatus 汇总当前令牌状态（安全外发）。
func GetAgentTokenStatus() AgentTokenStatus {
	hash, _ := setting.GetString(model.SettingKeyAgentAPITokenHash)
	username, _ := setting.GetString(model.SettingKeyAgentAPITokenUsername)
	prefix, _ := setting.GetString(model.SettingKeyAgentAPITokenPrefix)
	createdAt, _ := setting.GetString(model.SettingKeyAgentAPITokenCreatedAt)
	return AgentTokenStatus{
		Enabled:   agentTokenEnabled(),
		TokenSet:  strings.TrimSpace(hash) != "",
		Username:  strings.TrimSpace(username),
		Prefix:    strings.TrimSpace(prefix),
		CreatedAt: strings.TrimSpace(createdAt),
	}
}

// ActivateAgentToken 写入新令牌（只落摘要与展示前缀）、绑定用户名并开启令牌鉴权。
func ActivateAgentToken(token, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("agent token username is required")
	}
	if !strings.HasPrefix(token, AgentTokenPrefix) {
		return fmt.Errorf("agent token must start with %q", AgentTokenPrefix)
	}
	if err := setting.SetString(model.SettingKeyAgentAPITokenHash, HashAgentToken(token)); err != nil {
		return err
	}
	if err := setting.SetString(model.SettingKeyAgentAPITokenUsername, username); err != nil {
		return err
	}
	if err := setting.SetString(model.SettingKeyAgentAPITokenPrefix, AgentTokenDisplayPrefix(token)); err != nil {
		return err
	}
	if err := setting.SetString(model.SettingKeyAgentAPITokenCreatedAt, time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	return setting.SetString(model.SettingKeyAgentAPITokenEnabled, "true")
}

// RevokeAgentToken 清空摘要与展示信息并关闭令牌鉴权（即时生效，无需重启）。
func RevokeAgentToken() error {
	if err := setting.SetString(model.SettingKeyAgentAPITokenEnabled, "false"); err != nil {
		return err
	}
	if err := setting.SetString(model.SettingKeyAgentAPITokenHash, ""); err != nil {
		return err
	}
	if err := setting.SetString(model.SettingKeyAgentAPITokenPrefix, ""); err != nil {
		return err
	}
	return setting.SetString(model.SettingKeyAgentAPITokenCreatedAt, "")
}
