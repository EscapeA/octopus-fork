package model

import "time"

// PlanProviderType 区分额度查询和套餐查询两类厂商
type PlanProviderType string

const (
	PlanProviderTypeBalance   PlanProviderType = "balance"
	PlanProviderTypeTokenPlan PlanProviderType = "tokenplan"
)

// PlanProviderCategory 厂商具体分类
type PlanProviderCategory string

const (
	// 额度类厂商
	PlanProviderDeepSeek    PlanProviderCategory = "deepseek"
	PlanProviderKimi        PlanProviderCategory = "kimi"
	PlanProviderSiliconFlow PlanProviderCategory = "siliconflow"
	PlanProviderOpenRouter  PlanProviderCategory = "openrouter"
	PlanProviderStepFun     PlanProviderCategory = "stepfun"
	PlanProvider302AI       PlanProviderCategory = "302ai"
	PlanProviderNovita      PlanProviderCategory = "novita"
	PlanProviderOpenAI      PlanProviderCategory = "openai"

	// TokenPlan 类厂商
	PlanProviderMiniMax       PlanProviderCategory = "minimax"
	PlanProviderZhipu         PlanProviderCategory = "zhipu"
	PlanProviderStepFunPlan   PlanProviderCategory = "stepfun_plan"
	PlanProviderSenseNovaPlan PlanProviderCategory = "sensenova_plan"
	PlanProviderMiMoPlan      PlanProviderCategory = "mimo_plan"
)

// PlanProviderCategoryInfo 厂商元信息（非 DB 字段）
type PlanProviderCategoryInfo struct {
	Category    PlanProviderCategory `json:"category"`
	Name        string               `json:"name"`
	Type        PlanProviderType     `json:"type"`
	BaseURL     string               `json:"base_url"`
	Models      string               `json:"models"` // 默认模型列表（逗号分隔）
	Description string               `json:"description"`
	HelpURL     string               `json:"help_url"`
}

// PlanProviderCategories 所有支持的厂商
var PlanProviderCategories = []PlanProviderCategoryInfo{
	{
		Category:    PlanProviderDeepSeek,
		Name:        "DeepSeek",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://api.deepseek.com/v1",
		Models:      "deepseek-chat,deepseek-reasoner",
		Description: "DeepSeek 官方 API 余额查询",
		HelpURL:     "https://platform.deepseek.com/api_keys",
	},
	{
		Category:    PlanProviderKimi,
		Name:        "Kimi (月之暗面)",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://api.moonshot.cn/v1",
		Models:      "moonshot-v1-8k,moonshot-v1-32k,moonshot-v1-128k,kimi-latest",
		Description: "Kimi API 余额查询，支持 Coding Plan",
		HelpURL:     "https://platform.moonshot.cn/console/api-keys",
	},
	{
		Category:    PlanProviderSiliconFlow,
		Name:        "SiliconFlow (硅基流动)",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://api.siliconflow.com/v1",
		Models:      "*",
		Description: "硅基流动 API 余额查询",
		HelpURL:     "https://cloud.siliconflow.cn/account/ak",
	},
	{
		Category:    PlanProviderOpenRouter,
		Name:        "OpenRouter",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://openrouter.ai/api/v1",
		Models:      "*",
		Description: "OpenRouter Credits 余额查询",
		HelpURL:     "https://openrouter.ai/keys",
	},
	{
		Category:    PlanProviderStepFun,
		Name:        "StepFun (阶跃星辰)",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://api.stepfun.ai/v1",
		Models:      "step-3.7-flash,step-3.7,step-3.7-pro",
		Description: "阶跃星辰 API 余额查询",
		HelpURL:     "https://platform.stepfun.com/console/apikey",
	},
	{
		Category:    PlanProvider302AI,
		Name:        "302.ai",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://api.302.ai/v1",
		Models:      "*",
		Description: "302.ai 中转站余额查询",
		HelpURL:     "https://dashboard.302.ai",
	},
	{
		Category:    PlanProviderNovita,
		Name:        "Novita AI",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://api.novita.ai/v1",
		Models:      "*",
		Description: "Novita AI 余额查询（1 unit = 0.0001 USD）",
		HelpURL:     "https://novita.ai/dashboard/keys",
	},
	{
		Category:    PlanProviderOpenAI,
		Name:        "OpenAI",
		Type:        PlanProviderTypeBalance,
		BaseURL:     "https://api.openai.com/v1",
		Models:      "gpt-4.1,gpt-4.1-mini,gpt-4o,gpt-4o-mini,o3,o4-mini",
		Description: "OpenAI 余额查询（部分账户可用 /v1/balances 接口）",
		HelpURL:     "https://platform.openai.com/api-keys",
	},
	{
		Category:    PlanProviderMiniMax,
		Name:        "MiniMax Token Plan",
		Type:        PlanProviderTypeTokenPlan,
		BaseURL:     "https://www.minimaxi.com/v1",
		Models:      "MiniMax-M3,MiniMax-M2.7,MiniMax-M2.7-highspeed",
		Description: "MiniMax Token Plan 套餐用量查询（含日/周额度）",
		HelpURL:     "https://platform.minimaxi.com/user-center/basic-information/interface-key",
	},
	{
		Category:    PlanProviderZhipu,
		Name:        "智谱 GLM Coding Plan",
		Type:        PlanProviderTypeTokenPlan,
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		Models:      "GLM-5.2,GLM-5.1,GLM-5,GLM-5-Turbo",
		Description: "智谱 GLM Coding Plan 套餐用量查询（含日/月/季/年额度）",
		HelpURL:     "https://open.bigmodel.cn/usercenter/apikeys",
	},
	{
		Category:    PlanProviderStepFunPlan,
		Name:        "StepFun 套餐 (阶跃星辰)",
		Type:        PlanProviderTypeTokenPlan,
		BaseURL:     "https://platform.stepfun.com",
		Models:      "step-3.7-flash,step-3.7,step-3.7-pro",
		Description: "StepFun 套餐额度查询（Oasis-Token 必填，约 30 分钟有效期）。可选填 API Key 自动创建/复用转发渠道（接入点 api.stepfun.com/step_plan/v1）",
		HelpURL:     "https://platform.stepfun.com/plan-subscribe",
	},
	{
		Category:    PlanProviderSenseNovaPlan,
		Name:        "SenseNova 套餐 (商汤日日新)",
		Type:        PlanProviderTypeTokenPlan,
		BaseURL:     "https://platform.sensenova.cn",
		Models:      "sensenova-6.7-flash-lite,sensenova-u1-fast,deepseek-v4-flash",
		Description: "SenseNova Coding Plan 套餐用量查询（控制台 Bearer Token 必填，约 3 小时有效期）。可选填 API Key 自动创建/复用转发渠道（接入点 token.sensenova.cn/v1）",
		HelpURL:     "https://platform.sensenova.cn/console",
	},
	{
		Category:    PlanProviderMiMoPlan,
		Name:        "MiMo 套餐 (小米)",
		Type:        PlanProviderTypeTokenPlan,
		BaseURL:     "https://platform.xiaomimimo.com",
		Models:      "MiMo-V2-Pro,MiMo-V2-Flash,MiMo-V2-Edge",
		Description: "小米 MiMo Token Plan 套餐用量查询（Cookie 鉴权，有效期约数天至数周）。在浏览器登录 platform.xiaomimimo.com 后，按 F12 打开开发者工具 → Network → 刷新页面 → 任意请求的 Cookie 字段，复制完整内容（需包含 api-platform_serviceToken、userId、api-platform_slh、api-platform_ph 四个字段）",
		HelpURL:     "https://platform.xiaomimimo.com/console/plan-manage",
	},
}

// PlanProvider 持久化的 Plan Provider 记录
type PlanProvider struct {
	ID            int                  `json:"id" gorm:"primaryKey"`
	Name          string               `json:"name" gorm:"not null"`
	Category      PlanProviderCategory `json:"category" gorm:"type:varchar(32);not null;index"`
	ProviderType  PlanProviderType     `json:"provider_type" gorm:"type:varchar(16);not null;default:'balance'"`
	APIKey        string               `json:"api_key" gorm:"not null"`
	ForwardAPIKey string               `json:"forward_api_key" gorm:"default:''"`
	BaseURL       string               `json:"base_url" gorm:"not null"`
	ChannelID     int                  `json:"channel_id" gorm:"not null;default:0;index"`
	Balance       float64              `json:"balance" gorm:"default:0"`
	BalanceUsed   float64              `json:"balance_used" gorm:"default:0"`
	// TokenPlan 专用
	QuotaTotal    float64    `json:"quota_total" gorm:"default:0"`
	QuotaUsed     float64    `json:"quota_used" gorm:"default:0"`
	QuotaResetAt  *time.Time `json:"quota_reset_at"`
	WeeklyTotal   float64    `json:"weekly_total" gorm:"default:0"`
	WeeklyUsed    float64    `json:"weekly_used" gorm:"default:0"`
	WeeklyResetAt *time.Time `json:"weekly_reset_at"`
	Status        string     `json:"status" gorm:"type:varchar(32);not null;default:'active'"`
	LastRefresh   *time.Time `json:"last_refresh"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// PlanProviderListItem 列表响应
type PlanProviderListItem struct {
	PlanProvider
	Models         string `json:"models"`       // 从 Channel 继承的模型
	ChannelName    string `json:"channel_name"` // 关联渠道名称
	ChannelEnabled bool   `json:"channel_enabled"`
}
