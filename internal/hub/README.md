# Hub — 远程站点管理

Hub 将多站点账户管理功能内置到 Octopus 中，使其成为一站式 AI 资产管理平台。

## 架构

```
internal/hub/
├── adapter.go          # SiteAdapter 接口定义 (14 个方法)
├── registry.go         # 按 site_type 注册/获取适配器
├── httpclient.go       # 共享 HTTP 客户端 (FetchJSON 泛型)
├── common/adapter.go   # New API / One API 默认适配器 (兜底)
├── octopus/adapter.go  # Octopus 类型 (JWT 登录)
└── ldoh/adapter.go     # LDOH 公开站点发现

internal/op/remotesite/
├── remotesite.go       # 站点 CRUD + Refresh + DetectSiteType
├── balance.go          # 余额快照 CRUD + 定时捕获
├── checkin.go          # 签到执行 + 历史查询
├── announcement.go     # 公告拉取 + 存储
├── token.go            # 远程 Token 同步 + 导入本地渠道
└── migration.go        # 渠道迁移 (远程↔本地)

internal/op/credential/
└── credential.go       # API 凭据档案 CRUD
```

## 支持的站点类型

| SiteType | 适配器 | 说明 |
|----------|--------|------|
| `new-api` | common | One API / New API 系列（默认兜底） |
| `octopus` | octopus | Octopus 网关（JWT 登录） |
| `veloera` | common | 兼容 New API |
| `done-hub` | common | 兼容 New API |
| `one-hub` | common | 兼容 New API |
| `sub2api` | common | 兼容 New API |
| `anyrouter` | common | 兼容 New API |
| `aihubmix` | common | 兼容 New API |
| `axonhub` | common | 兼容 New API |
| `claude-code-hub` | common | 兼容 New API |
| `unknown` | common | 自动检测失败时的兜底 |

## SiteAdapter 接口

每个站点类型实现 14 个方法：

| 方法 | 功能 |
|------|------|
| `FetchUserInfo` | 获取账户信息（余额、用户名等） |
| `PerformCheckIn` | 执行签到 |
| `FetchCheckInStatus` | 查询今日是否已签到 |
| `FetchModels` | 获取可用模型列表 |
| `FetchModelPricing` | 获取模型定价 |
| `FetchTokens` | 列出 API Token |
| `CreateToken` | 创建 API Token |
| `ListChannels` | 列出渠道 |
| `CreateChannel` | 创建渠道 |
| `UpdateChannel` | 更新渠道 |
| `DeleteChannel` | 删除渠道 |
| `FetchAnnouncement` | 获取公告 |
| `FetchSiteStatus` | 获取站点公开状态 |

## API 路由

所有路由位于 `/api/v1/` 下，需要 JWT 认证。

### 远程站点

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | `/remote-site/list` | `sites:read` | 列出所有站点 |
| POST | `/remote-site/create` | `sites:write` | 添加站点 |
| POST | `/remote-site/update` | `sites:write` | 更新站点 |
| DELETE | `/remote-site/delete/:id` | `sites:write` | 删除站点 |
| POST | `/remote-site/refresh/:id` | `sites:write` | 刷新单个站点 |
| POST | `/remote-site/refresh-all` | `sites:write` | 刷新全部 |
| POST | `/remote-site/detect` | `sites:write` | 自动检测类型 |
| GET | `/remote-site/models/:id` | `sites:read` | 模型列表 |
| GET | `/remote-site/pricing/:id` | `sites:read` | 模型定价 |
| GET | `/remote-site/site-types` | `sites:read` | 已知站点类型 |

### 余额历史

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | `/balance-history/list/:site_id` | `sites:read` | 余额快照列表 |
| GET | `/balance-history/chart/:site_id` | `sites:read` | 图表数据 |
| POST | `/balance-history/capture/:site_id` | `sites:write` | 手动捕获 |

### 签到

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | `/checkin/status/:site_id` | `sites:read` | 今日签到状态 |
| POST | `/checkin/execute/:site_id` | `sites:write` | 执行签到 |
| POST | `/checkin/execute-all` | `sites:write` | 全部签到 |
| GET | `/checkin/history/:site_id` | `sites:read` | 签到历史 |

### API 凭据

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | `/api-credential/list` | `api_keys:read` | 列出凭据 |
| POST | `/api-credential/create` | `api_keys:write` | 创建凭据 |
| POST | `/api-credential/update` | `api_keys:write` | 更新凭据 |
| DELETE | `/api-credential/delete/:id` | `api_keys:write` | 删除凭据 |

### 验证 & CLI 导出

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| POST | `/verification/run` | `api_keys:read` | 运行验证探针 |
| POST | `/verification/run-for/:id` | `api_keys:read` | 按凭据验证 |
| GET | `/verification/probes` | `api_keys:read` | 可用探针列表 |
| POST | `/cli-export/generate` | `api_keys:read` | 生成 CLI 配置 |

### 公告

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | `/announcement/list` | `sites:read` | 全部公告 |
| GET | `/announcement/list/:site_id` | `sites:read` | 按站点查询 |
| POST | `/announcement/refresh/:site_id` | `sites:write` | 刷新公告 |
| POST | `/announcement/refresh-all` | `sites:write` | 刷新全部 |

### 远程 Token

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | `/remote-site-token/list/:site_id` | `sites:read` | Token 列表 |
| POST | `/remote-site-token/sync/:site_id` | `sites:write` | 同步 Token |
| POST | `/remote-site-token/sync-to-channel` | `sites:write` | 导入为本地渠道 |

### 渠道迁移

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| POST | `/channel-migration/migrate` | `sites:write` | 迁移单个渠道 |
| POST | `/channel-migration/migrate-all` | `sites:write` | 迁移全部渠道 |

### 站点发现

| Method | Path | 权限 | 说明 |
|--------|------|------|------|
| GET | `/site-discovery/discover` | `sites:write` | 查询公开目录 |

## 定时任务

在 `internal/task/init.go` 中注册：

| 任务 | 间隔 | 说明 |
|------|------|------|
| `hub_balance_capture` | 6 小时 | 捕获所有启用站点的余额快照 |
| `hub_auto_checkin` | 12 小时 | 所有启用站点自动签到（幂等） |
| `hub_announcement_fetch` | 4 小时 | 拉取所有启用站点的公告 |

## 凭据加密

所有 `access_token`、`password`、`api_key` 字段使用 AES-256-GCM 加密存储。

- 加密密钥：配置 `security.encryption_key` 或环境变量 `OCTOPUS_SECURITY_ENCRYPTION_KEY`
- 密文前缀：`enc:` (base64)
- 无前缀的值视为明文（向后兼容）
- 实现在 `internal/utils/crypto/`

## 导入导出

Hub 数据已纳入数据库备份/恢复体系（`internal/op/backup/`）：

**导出表：** `remote_sites`, `balance_snapshots`, `check_in_records`, `api_credential_profiles`, `site_announcements`, `remote_site_tokens`

**导入策略：**
- Incremental 模式：ON CONFLICT DO NOTHING（跳过已存在的记录）
- Full 模式：先按依赖逆序删除所有表数据，再插入

## 前端模块

| 路由 ID | 路径 | 说明 |
|---------|------|------|
| `hub` | 侧边栏 Hub | 站点管理主页面（卡片网格） |
| `announcement` | 侧边栏 Announcements | 公告聚合列表 |
| `checkin` | 侧边栏 Check-in | 签到状态与历史 |
| `credential` | 侧边栏 Credentials | API 凭据管理、验证、CLI 导出 |

站点详情页内嵌余额历史图表（BalanceChart）和远程 Token 管理弹窗（TokenManager）。

## 测试

```bash
# 运行 Hub 相关测试
go test ./internal/hub/...
go test ./internal/op/remotesite/...
go test ./internal/op/backup/...
go test ./internal/utils/crypto/...

# 前端
cd web && pnpm check
```

## 添加新站点适配器

1. 在 `internal/hub/<sitetype>/` 创建 `adapter.go`
2. 实现 `hub.SiteAdapter` 接口的 14 个方法
3. 在 `init()` 中调用 `hub.Register(model.SiteTypeXXX, &Adapter{})`
4. 在 `internal/model/remote_site.go` 添加 `SiteTypeXXX` 常量
5. 在 `model.AllSiteTypes()` 中注册
6. 前端 `web/src/api/endpoints/remote-site.ts` 的 `SITE_TYPES` 数组中添加

大多数 New API 兼容站点无需自定义适配器，`common` 适配器已作为默认兜底。仅当站点有非标准 API（如自定义签到端点、不同的响应格式）时才需要专用适配器。
