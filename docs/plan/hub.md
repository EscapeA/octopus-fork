# Octopus 全面增强计划：All API Hub 功能对齐 + 优化

> **目标：** 补齐 Octopus 与 All API Hub 浏览器扩展之间的全部功能差距，涵盖通知渠道、Hub 适配器、可观测性、云备份、用量分析等 10 大领域。
>
> **最后更新：** 2026-05-30

---

## 总览

| Phase | 内容 | 优先级 | 状态 | 完成度 |
|-------|------|--------|------|--------|
| **1** | 通知渠道扩展 (5 个) | P0 | ✅ 已完成 | 100% |
| **2** | Hub 适配器 (4 个) | P0 | ✅ 已完成 | 100% |
| **3** | 可观测性增强 | P1 | ✅ 已完成 | 100% |
| **4** | WebDAV 云备份 | P1 | ✅ 已完成 | 100% |
| **5** | 余额消耗预测 | P1 | ✅ 已完成 | 100% |
| **6** | 批量 Token 导出 | P2 | ✅ 已完成 | 100% |
| **7** | 全局模型映射表 | P2 | ✅ 已完成 | 100% |
| **8** | 自动兑换码 | P2 | ⬚ 待开始 | 0% |
| **9** | 远程站点用量历史 | P2 | ⬚ 待开始 | 0% |
| **10** | 分享快照 | P3 | ⬚ 待开始 | 0% |

---

## Phase 1：通知渠道扩展（P0） ✅ 已完成

> **验证：** Go build ✅ | Go tests ✅ | Frontend lint 0 errors ✅ | Frontend build ✅

### 已修改文件

| 文件 | 变更 |
|------|------|
| `internal/model/alert.go` | 新增 5 个 `AlertNotifChannelType` 常量 + 5 个 Config 结构体 |
| `internal/helper/notify.go` | 新增 5 个 `Send*` 函数 + dispatcher case + `buildAlertText` helper |
| `web/src/api/endpoints/alert.ts` | `NOTIF_CHANNEL_TYPES` 扩展 + 5 个 Config 接口 |
| `web/src/components/modules/alert/forms.ts` | `AlertChannelDraft` 新增字段 + 5 个 parser + 序列化 |
| `web/src/components/modules/alert/index.tsx` | `ChannelTypeIcon`/`ChannelConfigFields`/`getChannelTypeLabel`/`getChannelDescription` 扩展 |
| `web/public/locale/en.json` | 新增 channelTypes + form 字段 i18n |
| `web/public/locale/zh_hans.json` | 新增 channelTypes + form 字段 i18n |
| `web/public/locale/zh_hant.json` | 新增 channelTypes + form 字段 i18n |

### 已实现的通知渠道

| 渠道 | 类型常量 | Config 结构体 | API 端点 |
|------|---------|-------------|----------|
| Telegram | `AlertNotifTelegram` | `TelegramConfig{BotToken, ChatID}` | `POST api.telegram.org/bot{token}/sendMessage` |
| 飞书 | `AlertNotifFeishu` | `FeishuConfig{WebhookKey}` | `POST open.feishu.cn/open-apis/bot/v2/hook/{key}` |
| 钉钉 | `AlertNotifDingTalk` | `DingTalkConfig{WebhookKey, Secret}` | `POST oapi.dingtalk.com/robot/send?access_token={key}` + HMAC-SHA256 签名 |
| 企业微信 | `AlertNotifWeCom` | `WeComConfig{WebhookKey}` | `POST qyapi.weixin.qq.com/cgi-bin/webhook/send?key={key}` |
| ntfy | `AlertNotifNtfy` | `NtfyConfig{TopicURL, AccessToken}` | `POST {topicUrl}` + RFC 2047 编码 Title |

---

## Phase 2：Hub 适配器实现（P0） ✅ 已完成

> **验证：** Go build ✅ | 4 个适配器 `init()` 注册 ✅ | `remotesite.go` 导入更新 ✅

### 新建文件

| 文件 | 适配器 | 认证方式 | 支持的方法 |
|------|--------|---------|-----------|
| `internal/hub/axonhub/adapter.go` | AxonHub | Email/Password → JWT (缓存 + 401 重试) | ListChannels, CreateChannel, UpdateChannel, DeleteChannel (GraphQL) |
| `internal/hub/claudecodehub/adapter.go` | ClaudeCodeHub | 静态 admin token (Bearer) | ListChannels, CreateChannel, UpdateChannel, DeleteChannel (Action API) |
| `internal/hub/sub2api/adapter.go` | Sub2API | Access Token + Refresh Token (3 层刷新) | FetchUserInfo, FetchTokens, CreateToken, FetchAnnouncement |
| `internal/hub/aihubmix/adapter.go` | AIHubMix | 原始 token (非 Bearer) | FetchUserInfo, FetchModels, FetchModelPricing, FetchTokens, CreateToken |

### 修改文件

| 文件 | 变更 |
|------|------|
| `internal/op/remotesite/remotesite.go` | 新增 4 个 `_ "hub/xxx"` 导入 |

### 适配器详情

#### AxonHub (GraphQL)
- 认证：`POST {baseUrl}/admin/auth/signin` → JWT token，缓存 key = `baseURL|email`
- 所有请求：`POST {baseUrl}/admin/graphql`，401/403 自动重试一次
- 维护 `numericId ↔ graphqlId` 双向映射（GraphQL ID 是不透明字符串）
- 分页：cursor-based (`pageInfo.hasNextPage` + `endCursor`)

#### ClaudeCodeHub (Action API)
- 认证：静态 admin token → `Bearer` header
- 所有操作：`POST {baseUrl}/api/actions/providers/{action}`
- "providers" 代替 "channels"，Provider 类型默认 `"openai_compatible"`

#### Sub2API (JWT Refresh)
- 认证 3 层：存储 access token → 主动刷新 (120s 内过期) → 被动刷新 (401)
- Refresh endpoint：`POST /api/v1/auth/refresh` with `{refresh_token}`
- Response envelope：`{code: 0, message, data}`
- 余额 USD → quota 转换：`balance * 500000`
- Password 字段复用存储 refresh_token

#### AIHubMix (专用 REST)
- 认证：原始 token 在 `Authorization` header（**非** Bearer）
- API origin 固定为 `https://aihubmix.com`
- 定价使用直接 USD/百万 token 价格（非比率制）
- 不支持 channel/group 操作

---

## Phase 3：可观测性增强（P1） ✅ 已完成

> **验证：** Go build ✅ | Frontend lint 0 errors ✅ | Frontend build ✅ | i18n 三语 ✅

### 已修改文件

| 文件 | 变更 |
|------|------|
| `internal/model/stats.go` | `StatsMetrics` 新增 13 个字段 (延迟百分位 + FTUT + 直方图桶) + `Add()` 更新 |
| `internal/model/analytics.go` | 新增 `LatencyDistribution` 和 `HistogramBucket` 类型 |
| `internal/utils/telemetry/telemetry.go` | 新增 `p99Locked()` + 通用 `percentileLocked()` + Snapshot 增加 `P99LatencyMs` |
| `internal/relay/metrics.go` | `RelayMetrics.Save()` 新增直方图桶分配 + FTUT 记录 + telemetry P50/P95/P99 |
| `internal/op/analytics/analytics.go` | 新增 `AnalyticsLatencyDistributionGet()` + `loadLatencyDistribution()` + `percentileFromSorted()` |
| `internal/server/handlers/analytics.go` | 新增 `/latency-distribution` 路由 + `getAnalyticsLatencyDistribution` handler |
| `web/src/api/endpoints/analytics.ts` | 新增 `LatencyDistribution`/`HistogramBucket` 接口 + `useAnalyticsLatencyDistribution` hook |
| `web/src/components/modules/analytics/LatencyDistribution.tsx` | **新建** - 延迟分布 UI (MetricCard + 直方图条形图) |
| `web/src/components/modules/analytics/index.tsx` | 新增 `latency` tab |
| `web/public/locale/en.json` | 新增 `analytics.latency.*` 键 |
| `web/public/locale/zh_hans.json` | 新增 `analytics.latency.*` 键 (简体中文) |
| `web/public/locale/zh_hant.json` | 新增 `analytics.latency.*` 键 (繁体中文) |

### 新增 StatsMetrics 字段

```
延迟百分位: LatencyP50, LatencyP95, LatencyP99
FTUT 百分位: FtutAvg, FtutP50, FtutP95, FtutP99
直方图桶:    HistogramLt100 (<100ms), Histogram100to500, Histogram500to1k, Histogram1kto5k, HistogramGt5k (>5s)
```

### 新增 API 端点

```
GET /api/v1/analytics/latency-distribution?range=7d
```

返回 `LatencyDistribution` (P50/P95/P99/Avg for UseTime and FTUT, 5-bucket histogram)。

---

## Phase 4：WebDAV 云备份（P1） ✅ 已完成

> **验证：** Go build ✅ | Frontend build ✅ | i18n 三语 ✅

### 新增/修改文件

| 文件 | 变更 |
|------|------|
| `internal/op/backup/webdav.go` | ✅ 已存在 - WebDAV 客户端 (Test/Upload/Download/List/Delete) |
| `internal/op/backup/backup.go` | ✅ 已存在 - ExportAll/ImportWithMode |
| `internal/op/backup/scheduler.go` | ✅ **新建** - WebDAVBackupConfig 结构体 + Get/Set/Perform/Restore/List/Delete 函数 + cleanupOldBackups |
| `internal/server/handlers/webdav.go` | ✅ **新建** - `init()` 注册 7 个 API 端点 + 完整 CRUD handler |
| `internal/model/setting.go` | ✅ 新增 `SettingKeyWebDAVConfig` 常量 + 默认值 + Validate() |
| `internal/task/init.go` | ✅ 新增 `TaskWebDAVBackup` 常量 + 6 小时定时任务注册 |
| `web/src/api/endpoints/webdav.ts` | ✅ **新建** - WebDAVConfig/WebDAVFile 接口 + 7 个 hooks |
| `web/src/components/modules/setting/WebDAV.tsx` | ✅ **新建** - 完整配置 UI (开关/连接/选项/操作/备份列表) |
| `web/src/components/modules/setting/index.tsx` | ✅ 新增 `SettingWebDAV` 导入 + 卡片注册 |
| `web/public/locale/en.json` | ✅ 新增 `setting.webdav.*` 键 (30+ 字段) |
| `web/public/locale/zh_hans.json` | ✅ 新增 `setting.webdav.*` 键 (简体中文) |
| `web/public/locale/zh_hant.json` | ✅ 新增 `setting.webdav.*` 键 (繁体中文) |

### 新增 API 端点

```
GET    /api/v1/backup/webdav/config       — 获取配置（密码掩码）
POST   /api/v1/backup/webdav/config       — 保存配置
POST   /api/v1/backup/webdav/test         — 测试连接
POST   /api/v1/backup/webdav/backup       — 手动触发备份
POST   /api/v1/backup/webdav/restore      — 从备份恢复
GET    /api/v1/backup/webdav/list         — 列出远程备份
DELETE /api/v1/backup/webdav/delete       — 删除指定备份
```

### 核心功能

- **定时备份**：`TaskWebDAVBackup` 每 6 小时执行 `PerformWebDAVBackup()`
- **旧备份清理**：`cleanupOldBackups()` 保留最新 `max_backups` 个文件
- **配置存储**：JSON 格式存储在 `settings` 表，支持 `interval_hours`/`max_backups`/`include_stats`/`include_logs`
- **密码掩码**：GET 返回 `******`，POST 时保留原密码
- **后台执行**：手动备份使用 `context.Background()` 异步执行

---

## Phase 5：余额消耗预测（P1） ✅ 已完成

> **验证：** Go build ✅ | Frontend build ✅ | i18n 三语 ✅

### 新增/修改文件

| 文件 | 变更 |
|------|------|
| `internal/model/remote_site.go` | ✅ 新增 `BalancePrediction` 结构体（日均消耗/剩余天数/预测归零日/趋势点） |
| `internal/op/remotesite/balance.go` | ✅ 新增 `PredictBalance()` 函数 + `avgFloat64()` 辅助函数 |
| `internal/server/handlers/balance_history.go` | ✅ 新增 `getBalancePrediction()` handler + 路由注册 |
| `web/src/api/endpoints/balance-history.ts` | ✅ 新增 `BalancePrediction` 接口 + `useBalancePrediction()` hook |
| `web/src/components/modules/remote-site/BalanceChart.tsx` | ✅ 重构为合并历史+预测数据 + 预测信息卡片（4 指标） |
| `web/public/locale/en.json` | ✅ 新增 `chart.quota/predicted/dailyBurn/daysRemaining/sevenDayAvg/estimatedZero` |
| `web/public/locale/zh_hans.json` | ✅ 新增简体中文翻译 |
| `web/public/locale/zh_hant.json` | ✅ 新增繁体中文翻译 |

### 新增 API 端点

```
GET /api/v1/balance-history/prediction/:site_id — 获取余额预测数据
```

### 核心算法

- **数据源**：最近 30 天的 `BalanceSnapshot`（通过 `GetBalanceChartData`）
- **日消耗计算**：相邻两天 quota 差值 / 天数间隔，仅统计正消耗
- **均值策略**：计算 7 天和 30 天平均日消耗，使用 7 天均值作为主预测基准（更敏感）
- **剩余天数**：`currentQuota / dailyBurnRate`
- **趋势预测**：从最后历史点向后延伸 30 天，quota 降至 0 后停止
- **可视化**：Recharts `LineChart` 合并历史（实线）+ 预测（虚线）+ 归零日参考线

### 预测信息卡片

| 指标 | 说明 |
|------|------|
| Daily Burn | 当前日均消耗（7 天均值） |
| Days Remaining | 预计剩余天数 |
| 7-Day Avg | 7 天平均日消耗 |
| Est. Zero Date | 预计归零日期（MM-DD） |

---

## Phase 6：批量 Token 导出（P2） ✅ 已完成

> **验证：** Go build ✅ | Frontend build ✅ | i18n 三语 ✅

### 已修改文件

| 文件 | 变更 |
|------|------|
| `internal/op/remotesite/token.go` | 新增 `BatchExportToken` + `BatchExportResult` 结构体 + `BatchExportTokens()` 函数 |
| `internal/server/handlers/remote_site_token.go` | 新增 `exportTokens` handler + `/export/:site_id` 路由 |
| `web/src/api/endpoints/remote-site-token.ts` | 新增 `BatchExportResult` 接口 + `useExportTokens()` hook（原生 fetch + blob 下载） |
| `web/src/components/modules/remote-site/TokenManager.tsx` | 新增"导出"按钮（Upload 图标），调用 `useExportTokens()` |
| `web/public/locale/en.json` | 新增 `exportTokens` + `tokensExported` |
| `web/public/locale/zh_hans.json` | 新增简体中文翻译 |
| `web/public/locale/zh_hant.json` | 新增繁体中文翻译 |

### 新增 API 端点

```
GET /api/v1/remote-site-token/export/:site_id — 导出令牌 JSON 文件（Content-Disposition: attachment）
```

### 核心功能

- **解密导出**：从本地缓存读取令牌，AES-GCM 解密 key 字段后导出明文
- **文件下载**：使用 `Content-Disposition: attachment` 直接触发浏览器下载（与 `exportDB` 模式一致）
- **统计信息**：导出结果包含 `total_tokens`、`active_tokens`、`site_name`、`exported_at` 元数据
- **前端触发**：`useExportTokens()` 使用原生 `fetch` + `blob` + `downloadBlob()` 模式
- **权限**：`PermSitesRead`（只读操作）

---

## Phase 7：全局模型映射表（P2） ✅ 已完成

- [x] 7.1 `internal/model/model_mapping.go` — `ModelMapping` 模型 (Pattern/MatchType/TargetModel/Priority/ScopeGroupID)
- [x] 7.2 `internal/db/db.go` — AutoMigrate 添加 ModelMapping
- [x] 7.3 `internal/op/modelmapping/` — CRUD + 内存缓存 + 匹配引擎 (exact/wildcard/regex)
- [x] 7.4 `internal/relay/relay.go` — 在 `resolveCandidateModelName` 之前集成映射查找
- [x] 7.5 `internal/server/handlers/model_mapping.go` — CRUD API 端点
- [x] 7.6 `web/src/api/endpoints/model-mapping.ts` + `web/src/components/modules/model-mapping/` — 前端
- [x] 7.7 `web/src/route/config.tsx` + `nav-store.ts` — 路由 + 导航注册
- [x] 7.8 i18n 翻译

---

## Phase 8：自动兑换码（P2） ✓ 已完成

- [x] 8.1 `internal/model/redemption.go` — `RedemptionRecord` 模型（状态：success/already_used/invalid/failed）
- [x] 8.2 `internal/op/remotesite/redemption.go` — `RedeemCode()` / `RedeemCodes()` / `ListRedemptionHistory()`
- [x] 8.3 `internal/server/handlers/redemption.go` — `POST /redeem` + `POST /redeem-all` + `GET /history/:site_id`
- [x] 8.4 `internal/db/db.go` — AutoMigrate 添加 `RedemptionRecord`
- [x] 8.5 前端 hooks (`web/src/api/endpoints/redemption.ts`) + UI (`web/src/components/modules/redemption/index.tsx`)
- [x] 8.6 i18n 翻译（en/zh_hans/zh_hant）

**实现要点：**
- SiteAdapter 接口新增 `RedeemCode(ctx, site, code) (*RedeemResult, error)` 方法
- Common/Octopus 适配器实现 `POST /api/user/redemption` 调用
- AxonHub/AIHubMix/ClaudeCodeHub/Sub2API 适配器返回 `nil`（不支持兑换）
- 支持单站点批量兑换和全站点批量兑换两种模式
- 兑换历史记录包含状态、获得额度、执行时间

---

## Phase 9：远程站点用量历史拉取（P2） ✓ 已完成

- [x] 9.1 `internal/model/usage_history.go` — `RemoteUsageRecord` 模型
- [x] 9.2 `internal/op/remotesite/usage_history.go` — 通过 adapter 的 `/api/log/self` 分页拉取 + 指纹去重
- [x] 9.3 `internal/task/init.go` — 注册 6 小时定时任务
- [x] 9.4 `internal/server/handlers/usage_history.go` — 多维查询 API
- [x] 9.5 `internal/db/db.go` — AutoMigrate
- [x] 9.6 前端 hooks + UI (`web/src/components/modules/usage-history/index.tsx`)
- [x] 9.7 i18n 翻译

**实现要点：**
- SiteAdapter 接口新增 `FetchUsageLogs(ctx, site, page, pageSize) ([]RemoteUsageLog, error)` 方法
- Common 适配器实现 `/api/log/self` 分页拉取，支持分页遍历和去重
- 使用 MD5 指纹（siteID + logID + createdAt）确保不重复插入
- 后端提供多维查询 API：按站点/日期范围/模型/令牌筛选
- 支持汇总查询（summary）和小时级聚合（hourly）
- 定时任务每 6 小时自动同步所有启用站点的用量历史
- 前端提供筛选器、统计卡片和历史记录表格

---

## Phase 10：分享快照（P3） ✓ 已完成

- [x] 10.1 `web/package.json` — 新增 `html-to-image` 依赖
- [x] 10.2 `web/src/components/modules/analytics/ShareSnapshot.tsx` — 快照渲染组件 (toPng)
- [x] 10.3 `web/src/components/modules/analytics/index.tsx` — 添加分享按钮
- [x] 10.4 i18n 翻译（en / zh_hans / zh_hant）

---

## 架构原则

- **后端**：`init()` 路由注册、`resp.Success/Error` 响应、GORM CRUD、`hub.Register` 适配器注册
- **前端**：TanStack Query hooks、Radix UI 组件、next-intl 国际化、Recharts 图表
- **通知渠道**：复用 `helper.SendNotification` dispatcher + `model.AlertNotifChannel.Config` JSON blob
- **Hub 适配器**：复用 `hub.SiteAdapter` 接口 + `httpclient.FetchJSON` + `init()` 注册
- **Stats**：扩展现有 `StatsMetrics` 结构体，GORM AutoMigrate 处理新列
- **权限复用**：`PermSettingsRead/Write`、`PermSitesRead/Write`、`PermStatsRead`
- **i18n**：所有新增 UI 文本需 en / zh_hans / zh_hant 三语翻译
