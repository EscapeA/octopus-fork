# 号池（Account Pool）设计规格

日期：2026-07-27
状态：已批准

## 概述

为 Octopus 新增独立的"号池"子系统，允许用户将上游账号（凭据）集中管理为池，
渠道通过关联池来获取账号，替代现有的 inline key 模式。池调度器提供并发槽位、
EWMA 动态评分、粘性会话、自动冷却等高级调度能力。

参考实现：sub2api 的号池架构（Group + AccountGroup + 分层调度 + Redis 槽位 + EWMA）。

## 设计决策

- **方案 A（已选）**：Pool 作为 Channel 的 Key 提供者。Channel 新增 `pool_id` 字段，
  设了就从池调度器选账号，没设就用现有 inline keys。最小侵入现有 relay 管线。
- 调度能力：并发槽位限制、自动冷却、粘性会话、EWMA 动态评分（全部纳入第一版）。
- 并发控制用内存 atomic counter（与现有 balancer 一致，单机部署足够）。
- EWMA 纯内存（重启冷启动），冷却时间戳持久化到 DB。

## 数据模型

### AccountPool

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| id | int | PK | |
| name | string(128) | unique, not null | 池名称 |
| description | string(512) | | 描述 |
| strategy | string(32) | default "ewma" | 调度策略：ewma / round_robin / random |
| default_concurrency | int | default 1 | 池级默认并发上限（单账号） |
| cooldown_base_sec | int | default 300 | 冷却基础时长（秒） |
| enabled | bool | default true | 是否启用 |
| created_at | time | | |
| updated_at | time | | |

### PoolAccount

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| id | int | PK | |
| pool_id | int | index, not null | 所属池 |
| name | string(128) | | 显示名 |
| credentials | text | | JSON 凭据（token/cookie/service_account） |
| base_url | string(512) | | 上游 base URL（空则继承渠道） |
| status | string(32) | default "active" | active / disabled / error |
| schedulable | bool | default true | 是否参与调度 |
| priority | int | default 0 | 优先级（数字越大越优先） |
| concurrency | int | default 0 | 并发上限（0 = 继承池默认） |
| rate_limit_reset_at | int64 | default 0 | 429 冷却截止（Unix 秒） |
| overload_until | int64 | default 0 | 过载冷却截止（Unix 秒） |
| total_requests | int64 | default 0 | 累计请求数 |
| total_errors | int64 | default 0 | 累计错误数 |
| total_tokens | int64 | default 0 | 累计 token 数 |
| created_at | time | | |
| updated_at | time | | |

### Channel 扩展

Channel struct 新增：

```go
PoolID int `gorm:"default:0;index"` // 0 = inline keys，>0 = 从池选账号
```

ChannelUpdateRequest 新增 `PoolID *int`。

## 池调度器

包路径：`internal/relay/poolscheduler/`

### 核心接口

```go
// SelectAccount 从指定池选择一个可用账号。
// sessionHash 非空时启用粘性；excludeIDs 排除已尝试过的账号。
func SelectAccount(poolID int, sessionHash string, excludeIDs []int) (*model.PoolAccount, error)

// ReportResult 上报请求结果，更新 EWMA 统计。
func ReportResult(poolID, accountID int, success bool, ttftMs float64, outputTokens int64)

// SetCooldown 设置账号冷却截止时间。
func SetCooldown(poolID, accountID int, until time.Time)

// ReleaseSlot 释放并发槽位。
func ReleaseSlot(poolID, accountID int)

// RemovePool / RemoveAccount 清理钩子。
func RemovePool(poolID int)
func RemoveAccount(poolID, accountID int)

// PurgeStale 清理长时间无活动的内存统计（后台任务调用）。
func PurgeStale(idleThreshold time.Duration)
```

### 分层选择算法

| 层 | 逻辑 |
|---|---|
| L1 粘性 | sessionHash 非空 → 查 sticky sync.Map，命中且账号可用 → 直接返回 |
| L2 冷却过滤 | 排除 status != active / !schedulable / rate_limit_reset_at > now / overload_until > now / 在 excludeIDs 中 |
| L3 并发槽位 | 排除当前并发 >= 上限的账号 |
| L4 评分排序 | ewma：错误率升序 + TTFT 升序 + priority 降序；round_robin：atomic 轮转；random：rand |
| L5 绑定 | 写 sticky map + acquire 并发槽位 |

### EWMA 统计（内存）

```go
type accountStats struct {
    mu           sync.Mutex
    errorRate    float64   // α=0.3
    ttftMs       float64   // α=0.3
    lastActivity time.Time
}
// key: "poolID:accountID"
var globalPoolStats sync.Map
```

### 并发槽位（内存）

```go
// key: "poolID:accountID" → *int64 (atomic)
var globalPoolSlots sync.Map
```

AcquireSlot：atomic CAS increment，超过上限返回 false。
ReleaseSlot：atomic decrement。

### 粘性会话

```go
// key: "poolID:sessionHash" → accountID
var globalPoolSticky sync.Map
```

选中账号后写入；下次同 sessionHash 优先复用（账号不可用时自动降级重选）。

## Relay 管线集成

`internal/relay/relay.go` 的 attempt 逻辑中：

```go
if channel.PoolID > 0 {
    account, err := poolscheduler.SelectAccount(channel.PoolID, sessionHash, excludeAccountIDs)
    if err != nil { /* 无可用账号，ScopeNextChannel */ }
    // account.Credentials → 解析为 auth token 注入上游请求
    // account.BaseURL 非空 → 覆盖渠道 base URL
    // defer poolscheduler.ReleaseSlot(poolID, accountID)
} else {
    key := channel.GetChannelKeyExcludingWithCooldown(...)
}
```

请求完成后：

```go
poolscheduler.ReportResult(poolID, accountID, success, ttftMs, outputTokens)
if !success && isRateLimitError {
    cooldown := pool.CooldownBaseSec * 2^consecutiveFailures
    poolscheduler.SetCooldown(poolID, accountID, now.Add(cooldown))
    // 持久化到 DB
}
```

凭据注入方式：PoolAccount.Credentials 为 JSON，约定格式：
```json
{"type": "bearer", "token": "sk-ant-..."}
```
或
```json
{"type": "cookie", "value": "session=..."}
```
构建上游请求时按 type 注入 Authorization header 或 Cookie header。

## 健康管理

| 触发 | 动作 |
|---|---|
| 上游 429 | rate_limit_reset_at = now + cooldownBase × 2^n（指数退避，上限 3600s） |
| 上游 5xx / 超时 | overload_until = now + 60s |
| 连续 5 次失败 | status = "error"，移出调度 |
| 成功请求 | 重置连续失败计数，EWMA 恢复 |
| 手动禁用 | schedulable = false |

冷却/过载时间戳持久化到 DB（重启不丢）。
EWMA 纯内存（重启冷启动，几轮请求后收敛）。
连续失败计数纯内存（重启归零，保守设计）。

## 清理钩子

`internal/relay/init_hooks.go` 新增：

```go
// 池删除 → 清理 stats + slots + sticky
op.OnPoolDeletedHooks = append(op.OnPoolDeletedHooks, poolscheduler.RemovePool)
// 账号删除 → 清理对应条目
op.OnPoolAccountDeletedHooks = append(op.OnPoolAccountDeletedHooks, poolscheduler.RemoveAccount)
```

`internal/task/init.go` 新增定期任务：

```go
poolscheduler.PurgeStale(30 * time.Minute)
```

## 前端

### 导航注册

- `nav-store.ts`：NavItem 联合类型加 `"pool"`，DEFAULT_NAV_ORDER 在 `"channel"` 后插入
- `setting.go`：SettingKeyNavOrder / SettingKeyNavVisible 默认 JSON 加 `"pool"`
- 新迁移：存量 nav_order/nav_visible 补入 `"pool"`（保序、幂等）

### 页面组件

`web/src/components/modules/pool/`：

- `PoolList.tsx`：池列表（名称、策略、账号数、启用状态、操作按钮）
- `PoolForm.tsx`：创建/编辑池对话框
- `PoolDetail.tsx`：池详情（账号表格 + 实时状态）
- `AccountForm.tsx`：添加/编辑账号对话框
- `index.tsx`：页面入口

### 渠道表单

`web/src/components/modules/channel/Form.tsx`：

- 新增"关联号池"下拉选择（可选）
- 选了池后 inline keys 区域隐藏，显示提示"此渠道使用号池 {name} 提供凭据"

### API 客户端

`web/src/api/endpoints/pool.ts`：

```typescript
getPools(): Promise<AccountPool[]>
createPool(data): Promise<AccountPool>
updatePool(id, data): Promise<AccountPool>
deletePool(id): Promise<void>
getPoolAccounts(poolId): Promise<PoolAccount[]>
createPoolAccount(poolId, data): Promise<PoolAccount>
updatePoolAccount(poolId, accountId, data): Promise<PoolAccount>
deletePoolAccount(poolId, accountId): Promise<void>
```

## API 端点

```
GET    /api/v1/pool                        列表（支持 ?enabled=true 过滤）
POST   /api/v1/pool                        创建
PUT    /api/v1/pool/:id                    更新
DELETE /api/v1/pool/:id                    删除（关联渠道的 pool_id 置 0）
GET    /api/v1/pool/:id/account            账号列表
POST   /api/v1/pool/:id/account            添加账号
PUT    /api/v1/pool/:id/account/:aid       更新
DELETE /api/v1/pool/:id/account/:aid       删除
```

RBAC：admin/editor 可写，viewer 只读。
Handler 文件：`internal/server/handlers/pool.go`，init() 注册路由。

## 迁移

新迁移文件 `internal/db/migrate/0NN.go`（编号取当前最大 +1）：

1. AutoMigrate 创建 account_pools + pool_accounts 表
2. Channel 表加 pool_id 列（HasColumn 守卫）
3. nav_order/nav_visible 存量行补入 "pool"（保序去重、幂等）

## 测试策略

- `poolscheduler` 包：单元测试覆盖分层选择、EWMA 衰减、并发槽位、冷却过滤、粘性、清理
- `model`：PoolAccount 状态判断辅助方法
- `handlers`：API CRUD 集成测试（httptest + sqlite temp dir）
- relay 集成：channel.PoolID > 0 时走池调度器路径

## 不做的事（YAGNI）

- 不做 Redis 分布式槽位（单机部署，内存足够）
- 不做池级 RPM 限制（现有 ratelimit 中间件已覆盖 API key 维度）
- 不做跨池账号共享（一个账号只属于一个池）
- 不做 Wait Plan（满载时直接返回错误，由 retry 机制换渠道）
- 不做影子账号/配额维度（sub2api 特有，Octopus 不需要）
