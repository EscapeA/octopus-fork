package price

import (
	"strings"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
)

// 价格目录以人民币（¥/M tokens）为唯一口径，来源只有两处，不再有美元价、
// 不再从外部价格源（models.dev）同步：
//  1. llm_infos 表：模型管理页手工定价（price_manual=true）
//  2. model_price_categories：分类规则兜底价（界面维护）
//
// 未命中任何来源 = 未定价，成本按 0 计。原先的内存美元价表（presets.go /
// models.dev 同步）与整词子串兜底（matchFallbackPrice）已随「去美元」一并移除。
// 命中峰谷规则（model_price_schedules）的模型由 EffectiveLLMPrice 直接取规则价，
// 不经过本函数。

// GetLLMPrice 返回模型 modelName 的计费单价（人民币，¥/M tokens）。
func GetLLMPrice(modelName string) *model.LLMPrice {
	modelName = strings.ToLower(modelName)
	p, err := llm.Get(modelName)
	// 渠道同步（LLMPriceAddToDB）会给未知价模型插入四价全 0 的行，DB 命中
	// 不能无条件短路，否则分类兜底恰好对它设计要覆盖的「同步过但没定价」的
	// 模型不生效。0 价行继续走兜底链。
	if err == nil && !isZeroPrice(p) {
		return &p
	}
	// 分类表兜底：按规则匹配的模型价格分类（优先级高于未定价占位行）。
	if cat := llm.PriceCategoryMatch(modelName); cat != nil {
		return cat
	}
	if err == nil {
		// DB 有 0 价行且所有兜底都未命中：维持原语义，返回 DB 行（0 价 = 不计费）。
		return &p
	}
	return nil
}

// isZeroPrice 报告四项价格是否全为 0（渠道同步为未知价模型写入的占位行）。
func isZeroPrice(p model.LLMPrice) bool {
	return p.Input == 0 && p.Output == 0 && p.CacheRead == 0 && p.CacheWrite == 0
}
