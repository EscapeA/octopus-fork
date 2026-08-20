package price

// 手工维护的价格预设，与 scripts/updatePrice.py 生成的 presets.go 分离，
// 避免重跑脚本时丢失条目。数值按 USD / 1M tokens 存储（与 presets.go 全部条目口径一致）。
//
// 来源：DeepSeek 官网价目表（2026-08-17 起峰谷定价，CNY / 1M tokens，按 ≈7.15 汇率换算为 USD）：
//   目录只存高峰价，空闲由 EffectiveLLMPrice 乘 0.5，不在本表存第二套价。
//   - deepseek-v4-flash 高峰：缓存命中 0.10 元，输入未命中 3 元，输出 9 元
//   - deepseek-v4-pro   高峰：缓存命中 0.30 元，输入未命中 9 元，输出 27 元
//
// 无缓存写（CacheWrite=0）。models.dev 后台刷新收录后会用官方数据覆盖同名键，
// 但 UpdateLLMPrice 结束时会 overlay 高峰预设回盖（见 overlayDeepSeekPeakPresets）。

import (
	"strings"

	"github.com/lingyuins/octopus/internal/model"
)

func init() {
	llmPrice["deepseek-v4-flash"] = model.LLMPrice{
		Input:      3.0 / deepSeekCNYPerUSD,
		Output:     9.0 / deepSeekCNYPerUSD,
		CacheRead:  0.10 / deepSeekCNYPerUSD,
		CacheWrite: 0,
	}
	llmPrice["deepseek-v4-pro"] = model.LLMPrice{
		Input:      9.0 / deepSeekCNYPerUSD,
		Output:     27.0 / deepSeekCNYPerUSD,
		CacheRead:  0.30 / deepSeekCNYPerUSD,
		CacheWrite: 0,
	}
}

// PeakPresetPrice 返回 DeepSeek v4 白名单模型的高峰预设价（USD/1M），
// 非白名单返回 nil。供同步刷新（LLMPriceRefreshExistingModels）对存量 DB 行
// 强制回盖高峰价使用，与 overlayDeepSeekPeakPresets 同源同值。
func PeakPresetPrice(modelName string) *model.LLMPrice {
	modelName = strings.ToLower(modelName)
	var peak model.LLMPrice
	switch modelName {
	case "deepseek-v4-flash":
		peak = model.LLMPrice{
			Input:      3.0 / deepSeekCNYPerUSD,
			Output:     9.0 / deepSeekCNYPerUSD,
			CacheRead:  0.10 / deepSeekCNYPerUSD,
			CacheWrite: 0,
		}
	case "deepseek-v4-pro":
		peak = model.LLMPrice{
			Input:      9.0 / deepSeekCNYPerUSD,
			Output:     27.0 / deepSeekCNYPerUSD,
			CacheRead:  0.30 / deepSeekCNYPerUSD,
			CacheWrite: 0,
		}
	default:
		return nil
	}
	return &peak
}

// overlayDeepSeekPeakPresets 在价格同步（UpdateLLMPrice）后强制回盖 DeepSeek
// v4 高峰预设。models.dev 等外部同步源只有单一价（旧平价或官方未区分峰谷的
// 数值），同步后若不回盖，llmPrice 里 flash/pro 会被打回旧平价，导致
// EffectiveLLMPrice 的「高峰目录价 ×0.5」基准错误。
//
// 必须在持有 llmPriceLock 的前提下写入与 presets_manual 相同的两个键。
func overlayDeepSeekPeakPresets() {
	llmPriceLock.Lock()
	defer llmPriceLock.Unlock()
	llmPrice["deepseek-v4-flash"] = model.LLMPrice{
		Input:      3.0 / deepSeekCNYPerUSD,
		Output:     9.0 / deepSeekCNYPerUSD,
		CacheRead:  0.10 / deepSeekCNYPerUSD,
		CacheWrite: 0,
	}
	llmPrice["deepseek-v4-pro"] = model.LLMPrice{
		Input:      9.0 / deepSeekCNYPerUSD,
		Output:     27.0 / deepSeekCNYPerUSD,
		CacheRead:  0.30 / deepSeekCNYPerUSD,
		CacheWrite: 0,
	}
}
