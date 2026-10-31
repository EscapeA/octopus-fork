// 本 fork 对这些上游功能只保留代码、不对外开放入口。
//
// 目的：跟上游合并时不再因为「整包删除」而产生冲突（删除的文件上游每次改动都会
// 变成 delete/modify 冲突），同时保证能力不会被误触发。
//
// 恢复方式：把对应项改成 true，并重新放开前端入口（setting/index.tsx 的
// SETTING_ITEM_DEFS、分组卡片/AIRouteButton、分析中心 evaluation 页签）。
package featureflags

const (
	// AIRouteEnabled 控制「AI 自动路由」。开启后会把渠道名、模型名与 API Key
	// 发给配置里的外部分析服务，默认关闭。
	AIRouteEnabled = false

	// SemanticCacheEnabled 控制「语义缓存」。上游自身默认也是关闭，这里强制关闭。
	SemanticCacheEnabled = false
)
