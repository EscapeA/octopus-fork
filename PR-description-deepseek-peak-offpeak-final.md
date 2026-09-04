## 动机

DeepSeek V4 自 2026-08-17 起实行**峰谷定价**（北京时间 09:00-12:00 / 14:00-18:00 为高峰，其余空闲半价），官方价目见 [api-docs.deepseek.com/quick_start/pricing](https://api-docs.deepseek.com/quick_start/pricing)。

当前上游仓库只有静态平价（`presets_manual.go` 中 deepseek-v4-flash `$0.14/$0.28`、deepseek-v4-pro `$0.42/$0.84`），**没有任何峰谷计费能力**：所有模型无论何时请求都按同一价格计费，与官方峰谷口径不一致，也无法在管理端调整价格或时段。

本 PR 为仓库新增完整的**峰谷计费能力**：价格、高峰时段、空闲倍率均可在管理端配置，命中规则的模型按时段计费（高峰全价、空闲 × 倍率），并在日志与模型列表中展示计费窗口。

## 改动内容

### 后端

**新增 `model_price_schedules` 表**（`internal/model/price_schedule.go`，AutoMigrate 自动建表）：

| 字段 | 说明 |
|---|---|
| name | 规则名（唯一，小写） |
| rule_type / rule_value | 模型匹配规则：`exact` / `prefix` / `contains`（忽略大小写，与价格分类同语义） |
| input / output / cache_read / cache_write | 高峰价（USD/1M tokens） |
| off_peak_mul | 空闲倍率（官方 0.5） |
| window1_start/end, window2_start/end | 高峰窗口（北京时间分钟 0-1440，半开区间，start==end 视为无效） |
| sort_order / enabled | 优先级（越小越先匹配）/ 启停 |

**新增规则驱动计费**（`internal/price/deepseek_schedule.go`）：
- `BillingWindow(model, at)`：按规则窗口判定 `peak` / `offpeak` / `""`（未命中规则不套峰谷）
- `EffectiveLLMPrice(model, at)`：命中规则 → 高峰窗口返回规则高峰价，空闲窗口返回高峰价 × off_peak_mul；未命中 → 回落 `GetLLMPrice` 原价（行为不变）
- 固定北京时区判定，与容器 TZ / stats_timezone 解耦

**relay 计费接入**（`internal/relay/metrics.go`）：
- 计费改走 `EffectiveLLMPrice(actualModel, m.StartTime)`
- relay_logs 新增 `billing_window` 列（`peak`/`offpeak`/空），日志列表/详情展示「高峰/空闲」徽章
- `relaylog.go` Select 列同步补 `billing_window`，保证缓存路径与 DB 路径一致（不出现「最新 N 条有徽章、被顶出后消失」）

**新增管理端 API**（`/api/v1/model/price-schedule/*`，已入审计白名单）：
- `GET /list`、`POST /create`、`POST /update`、`POST /delete`

**启动 seed**（`internal/op/llm/price_schedule.go`）：表空时自动插入 DeepSeek 官方美元高峰价两条默认规则，升级后计费行为平滑过渡，之后完全由管理端接管（可改可删）：

| 规则 | 高峰输入(cache miss) | 高峰输出 | 高峰缓存命中 | 倍率 | 窗口 |
|---|---|---|---|---|---|
| deepseek-v4-flash (prefix) | $0.44 | $1.32 | $0.014 | 0.5 | 09:00-12:00 / 14:00-18:00 |
| deepseek-v4-pro (prefix) | $1.32 | $3.96 | $0.044 | 0.5 | 同上 |

（官方 OFF-PEAK 恰为 PEAK × 0.5，与 off_peak_mul=0.5 语义一致）

**移除静态平价预设**：`internal/price/presets_manual.go` 删除（平价 flash/pro 被默认规则替代），同步刷新不再回盖 DeepSeek 价（价格由规则表运行时计算，不经 DB）。

**模型列表**：`GET /api/v1/model/list` 新增 `billing_schedule` 字段，命中规则的模型标 `deepseek_v4`，前端卡片显示峰谷徽章。

### 前端

- **新增「峰谷计费」卡片**（`web/src/components/modules/model/PeakScheduleSection.tsx`）：挂在「模型广场 → 价格分类」页，价格分类卡片下方，同款卡片样式
  - 表格：名称 / 规则类型 / 规则值 / 高峰四价 / 空闲倍率 / 高峰时段 / 优先级 / 启停
  - 添加/编辑 Dialog：规则匹配、高峰四价、空闲倍率、双高峰时段（time input，北京时间）、优先级、启用开关
  - 删除带确认
- **模型卡片峰谷徽章**（Item / MobileModelItem / ItemOverlays）：`billing_schedule` 标注「峰谷」
- **日志卡片**（log/Item.tsx）：`billing_window` 显示「高峰」/「空闲」徽章
- **locale**：en / zh_hans / zh_hant 新增 `peakSchedule` 词条（42 键）

## 影响矩阵

| 场景 | 改动前（上游现状） | 改动后 |
|---|---|---|
| 非 DeepSeek 模型（gpt-4o 等） | GetLLMPrice 原价 | 不变（未命中规则，回落原价） |
| deepseek-v4-flash / pro | 平价 $0.14/$0.28、$0.42/$0.84 | seed 默认峰谷规则，按时段计费 |
| 中转商前缀（olm/deepseek-v4-pro 等） | 平价 | 默认不套（prefix 规则不命中），需要时可配 contains 规则 |
| relay_logs | 无窗口字段 | 新增 billing_window 列（AutoMigrate 自动加） |
| 模型列表 | 无标注 | billing_schedule 字段 + 峰谷徽章 |

## 验证记录

- `go build ./...` 通过
- `go test ./internal/price/... ./internal/op/llm/... ./internal/helper/... ./internal/relay/... ./internal/server/...` 全绿
  - 覆盖：seed 幂等与默认值、BillingWindow 峰值/空闲/边界、UTC 输入、未命中回落、自定义规则改窗口/倍率即时生效、CRUD 与校验
- 前端 `pnpm build` 通过
- 部署验证（生产环境）：
  - seed 自动插入 2 条规则 ✓
  - CRUD API create→update→delete 全通 ✓
  - 模型列表 5 个 deepseek 变体标注峰谷 ✓
  - 重启后规则持久 ✓

## 风险与兼容性

- **升级迁移**：表空时 seed 自动兜底，计费不会突变（默认值=官方高峰价）；已存在规则的表不会重复 seed。规则表生效后以管理端配置为准
- **DB 兼容**：新增表 + relay_logs 新列均为 AutoMigrate 自动处理，SQLite / MySQL / Postgres 通用
- **性能**：规则匹配走内存缓存（`atomic.Pointer` 快照，与 priceCategoryCache 同构），计费热路径不查 DB；增删改时刷新缓存
- **时区**：窗口固定北京时间，与 stats_timezone、容器 TZ 解耦（与统计口径一致，均以北京为准）
- **匹配语义**：默认规则用 prefix 匹配（`deepseek-v4-flash` 前缀），中转商前缀（olm/ 等）默认不套峰谷——避免中转商价格与官方时段不一致导致的计费偏差；需要时可自行添加 contains 规则

## 预览图

预览图
<img width="1075" height="781" alt="Screenshot_2026-08-22-20-19-55-474_com android chrome" src="https://github.com/user-attachments/assets/c419e512-59ff-4f0f-9666-7d10443ced64" />
<img width="1078" height="698" alt="Screenshot_2026-08-22-20-27-54-477_com android chrome" src="https://github.com/user-attachments/assets/ec4e7b6b-c5f3-4cf4-ae22-24f5dd47322c" />
<img width="889" height="863" alt="Screenshot_2026-08-22-20-27-20-327_com android chrome" src="https://github.com/user-attachments/assets/c3feb35b-6b09-4869-a27b-d0a0c2efe9e3" />
