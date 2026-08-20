package price

import (
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

const (
	// BillingWindowNone 表示模型不套峰谷计费（非 DeepSeek v4 或中转商前缀）。
	BillingWindowNone = ""
	// BillingWindowPeak 高峰：北京时间 [09:00,12:00) ∪ [14:00,18:00)。
	BillingWindowPeak = "peak"
	// BillingWindowOffPeak 空闲：高峰以外时段。
	BillingWindowOffPeak = "offpeak"

	// deepSeekOffPeakMul 空闲倍率（官方：空闲 = 高峰一半）。
	deepSeekOffPeakMul = 0.5
	// deepSeekCNYPerUSD 预设价 CNY→USD 换算汇率（与 presets_manual.go 口径一致）。
	deepSeekCNYPerUSD = 7.15
)

// deepSeekLoc 固定为北京时区（Asia/Shanghai 加载失败时回退 UTC+8）。
// 峰谷窗口判定与时区、统计时区（stats_timezone）及容器 TZ 完全解耦。
var deepSeekLoc = loadDeepSeekLocation()

func loadDeepSeekLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("UTC+8", 8*3600)
}

func deepSeekLocation() *time.Location {
	return deepSeekLoc
}

// DeepSeekFamily 判断模型名是否属于官方 DeepSeek v4 系列（套峰谷计费）。
// 返回 "flash" | "pro" | ""。
//
// 匹配规则：
//   - 无前缀：deepseek-v4-flash* / deepseek-v4-pro*（整词，含 -0731 等变体）
//   - 前缀仅 deepseek/ 与 deepseek-ai/（只看第一段）
//   - 其它中转商前缀（olm/、vova/、opc/ 等）一律不套——中转商价未必跟官方同时段
func DeepSeekFamily(modelName string) string {
	name := strings.ToLower(modelName)
	base := name
	if idx := strings.IndexByte(name, '/'); idx >= 0 {
		prefix := name[:idx]
		if prefix != "deepseek" && prefix != "deepseek-ai" {
			return ""
		}
		base = name[idx+1:]
	}
	// pro 优先于 flash（deepseek-v4-pro 不含 deepseek-v4-flash 子串，顺序仅防御）
	if containsWholeWord(base, "deepseek-v4-pro") {
		return "pro"
	}
	if containsWholeWord(base, "deepseek-v4-flash") {
		return "flash"
	}
	return ""
}

// BillingWindow 返回模型 modelName 在时刻 at 所属的计费窗口。
// 非 DeepSeek v4 返回 ""；否则按北京时间判定高峰/空闲。
// 半开区间 [09:00,12:00) ∪ [14:00,18:00)，秒数忽略（11:59:59 仍高峰，12:00:00 空闲）。
func BillingWindow(modelName string, at time.Time) string {
	if DeepSeekFamily(modelName) == "" {
		return BillingWindowNone
	}
	local := at.In(deepSeekLocation())
	mins := local.Hour()*60 + local.Minute()
	peakMorning := mins >= 9*60 && mins < 12*60
	peakAfternoon := mins >= 14*60 && mins < 18*60
	if peakMorning || peakAfternoon {
		return BillingWindowPeak
	}
	return BillingWindowOffPeak
}

// ScaleLLMPrice 将价格的四个字段统一乘以 mul。
func ScaleLLMPrice(p model.LLMPrice, mul float64) model.LLMPrice {
	return model.LLMPrice{
		Input:      p.Input * mul,
		Output:     p.Output * mul,
		CacheRead:  p.CacheRead * mul,
		CacheWrite: p.CacheWrite * mul,
	}
}

// EffectiveLLMPrice 返回模型 modelName 在时刻 at 的有效计费价：
// 目录价（GetLLMPrice）在空闲窗口乘以 deepSeekOffPeakMul（0.5），高峰/非 DeepSeek 原价。
// 目录存高峰价，空闲由本函数缩放，避免扩展 LLMPrice 八列。
func EffectiveLLMPrice(modelName string, at time.Time) *model.LLMPrice {
	p := GetLLMPrice(modelName)
	if p == nil {
		return nil
	}
	if BillingWindow(modelName, at) == BillingWindowOffPeak {
		scaled := ScaleLLMPrice(*p, deepSeekOffPeakMul)
		return &scaled
	}
	return p
}
