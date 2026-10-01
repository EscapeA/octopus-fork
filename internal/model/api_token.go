package model

import "time"

// APIToken 是机器身份（脚本 / agent / CI）调用管理面 API 的令牌。
//
// 替代「用管理员账号密码登录换 JWT」：只保存 SHA-256 摘要（明文仅在创建时返回一次），
// 绑定一个已存在用户 → 鉴权按该用户角色走 RBAC，审计日志因此能区分人（JWT）与机器（令牌）。
type APIToken struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"not null" json:"name"`
	TokenHash  string    `gorm:"uniqueIndex;not null" json:"-"`
	Prefix     string    `gorm:"not null;default:''" json:"prefix"`
	UserID     uint      `gorm:"not null;index" json:"user_id"`
	Username   string    `gorm:"not null;default:''" json:"username"`
	ExpiresAt  int64     `gorm:"not null;default:0" json:"expires_at"`
	LastUsedAt int64     `gorm:"not null;default:0" json:"last_used_at"`
	LastUsedIP string    `gorm:"not null;default:''" json:"last_used_ip"`
	RevokedAt  int64     `gorm:"not null;default:0" json:"revoked_at"`
	CreatedBy  uint      `gorm:"not null;default:0" json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 固定表名，避免依赖 GORM 对首字母缩略词的单复数推断。
func (APIToken) TableName() string { return "api_tokens" }

// APITokenCreateRequest 创建令牌请求。
// Username 必须是已存在的用户；ExpiresDays=0 表示永不过期。
type APITokenCreateRequest struct {
	Name        string `json:"name"`
	Username    string `json:"username"`
	ExpiresDays int    `json:"expires_days"`
}

// APITokenRevokeRequest 吊销请求。
type APITokenRevokeRequest struct {
	ID uint `json:"id"`
}

// APITokenView 列表展示用视图：不含摘要，也不含明文。
type APITokenView struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Prefix     string    `json:"prefix"`
	UserID     uint      `json:"user_id"`
	Username   string    `json:"username"`
	UserExists bool      `json:"user_exists"`
	ExpiresAt  int64     `json:"expires_at"`
	LastUsedAt int64     `json:"last_used_at"`
	LastUsedIP string    `json:"last_used_ip"`
	RevokedAt  int64     `json:"revoked_at"`
	CreatedBy  uint      `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// APITokenCreateResponse 只在创建时返回一次明文令牌。
type APITokenCreateResponse struct {
	Token     string `json:"token"`
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
}
