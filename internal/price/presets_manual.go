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

import "github.com/lingyuins/octopus/internal/model"

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
