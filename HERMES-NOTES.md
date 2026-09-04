# Octopus (EscapeA/octopus) — HERMES 项目笔记

> 本文件为 Hermes 项目专属笔记，git 不跟踪（已在 .gitignore 忽略）。内容为 Octopus 部署/运维/调试的配置、凭证、状态与方法论细节。

## 仓库

- **上游链**：bestruirui/octopus（上游源仓库）→ lingyuins/octopus（群友 fork，部门级中间层）→ EscapeA/octopus（个人用）
- **PR 对象**：直接提 lingyuins/octopus，被合入上游后使用上游最新版；无 dev/main 集成分支
- **本地路径**：`/home/aries/.hermes/workspace/octopus-fork/`
- **构建**：internal/price/presets.go 已 gitignore（updatePrice.py 重生成，误提交需 reset --soft + rm --cached + force-push）
- **PR 约定**：向 lingyuins 上游提交一律 maintainer_can_modify=true

## 部署 / 服务

- **HCS 119.8.32.187**（tailscale esc-hk 100.69.155.60）
- octopus HTTPS serve：`https://esc-hk.tail1f56db.ts.net:8443` → 127.0.0.1:6677（tailnet only；本机 shell 全局 https_proxy 指向 HCS 3proxy，curl 访问 .ts.net 须 --noproxy '*'）
- 容器：octopus + openlist + cloudpan189-share 自动重启；长耗时产物 scp 本地落盘（ECS /tmp 云侧重启丢失）

## 认证（供 agent 测接口）

- HCS octopus 管理员登录：`keseimiru2` / `octopushk409123..`（POST /api/v1/user/login 拿 JWT，验证接口用 Bearer）
- ⚠️ octopus 用户密码是 bcrypt 哈希，无法反向推出；**不要乱改 DB 密码来验证**（用户明确纠正）
- Arcway↔Octopus 级联：`http://172.17.0.1:18090` key `sk-arcway-octopus-Xk9mPq2Rt7vB4nD6`（无限额）

## 已知 Bug / 调试方法论

### relay.go:1718-1730 顺序 Bug（2026-09-01 实证）
- client disconnected 返回 ScopeAbortAll，外层 retry loop 先熔断 RecordFailure（1720 命中 ScopeAbortAll）后才到 1725 errClientDisconnected 早退 → 每次 disconnect 计一次连续失败，5 次误熔断健康渠道
- 根因：流结束客户端读完即关连接，select 在 clientDone 与上游 EOF 间随机竞态
- relay_logs 特征：error='client disconnected' + tokens=0 + use_time 大 2~137s + 横跨多渠道 = 非上游故障
- 上游 lingyuins master 同有此 bug；修复 = 熔断条件加 `!errors.Is(result.Err, errClientDisconnected)`
- 完整证据链见 `octopus-debugging` skill（user-owned，待 curator adopt 后人工落盘 references/）

### B.A.I (chat.b.ai) 限流（2026-09-01 实测）
- 端点 api.b.ai/v1（直连被墙需代理）；免费模型 GLM-5.3-Flash/DeepSeek-V4-Flash/Qwen3.8-Flash
- 限流三层：
  1. **10 RPM/账号/60s** 滑动窗口（账号级，多 key=多账号=多独立桶）
  2. **rolling 12-hour** 订阅/活动成本窗口（非午夜重置，逐时释放；官方文档原文 "capacity is gradually released over time"）
  3. **activity_cost_limit_reached = 503**（打到成本上限特征错误），约 40-60min 自动恢复（17:10:33 触发→17:59:16 恢复，实测 49min），非硬冻结
- 429 头 x-oneapi-request-id 表明后端 one-api/new-api 网关
- Octopus BAI 渠道 **key_selection_strategy=cost**：恒选 TotalCost 最低 key 近似轮询（144/145 已 45:46 对半）；cost 是优选非硬限（key 429/失败后 relay.go failedKeyIDs 排除，自动换 key 重试 maxKeyRetriesPerRoute，3key 够）
- attempt N/7 逐次失败链只在 `docker logs octopus`（relay_logs 只存聚合 error 丢上游错误体）
- **Octopus key total_cost(~$5) 是本地计费 ≠ B.AI 端成本额度，勿混**

### hero signals
- 已实现 4 项 2×4

### TPS 口径
- 修正（output÷(use_time−ftut)，Item.tsx）已随 v2.6.0 部署 HCS 2026-08-28；上游无此修复，提上游需单独 PR（RecordKeySpeed 总耗时口径 Aries 明确不改）

### reasoning_content 400 / jsoniter / CF UA block / 级联
- 见 `octopus-debugging`、`octopus-operations`、`openai-compat-*` 等 skill（本笔记不再重复）

## 相关 skill
- `octopus-debugging`（调试方法论）、`octopus-operations`（运维手册）、`octopus-model-pricing`（价格/限流）、`octopus-fk-migration-fix`、`octopus-histogram-stats-bug`

## 2026-09-04 SenseNova 套餐监控适配新版额度面板
- 官方 2026-09 改版：旧接口 /lite/console/v1/user/coding-plan/usages 废弃（新版前端 0 引用），新接口 GET /lite/console/v1/tokenplan/pool-usage（无参数，Bearer JWT）
- 响应 {plan, pools[]}，每池 window_5h/window_7d（全字符串，reset_at=Unix秒）+ grant_balance + nearest_grant_expiry
- 映射：five_hour←window_5h、weekly←window_7d、quota←grant_balance（重置=最近授权到期）；多池求和
- commit 43bdf2fa 已部署 HCS 并重启生效（v2.6.1 / Commit 43bdf2fa），线上实测三档正常
- 测试文件 stepfun_plan_test.go 的 SenseNova 用例按真实双池响应构造

## 2026-09-04 SenseNova 分池面板（已完成）
- 后端：pool-usage 替代旧 coding-plan/usages；TokenPlanResult.Pools + PlanProvider.PoolsJSON(text)
- 前端：SenseNovaPoolCard 官方样式（紫=default/橙=dedicated），compact 模式回退三行布局
- 部署：v2.6.1 / 7fd5dbcd；迁移 053 幂等加 pools_json 列（⚠️原 047 与已有迁移撞号致崩溃循环，已改为 053）
- 三个坑已固化到 octopus-operations skill：迁移版本唯一性校验（Python 扫 Version:）、
  SQLite WAL 三件套拷贝、崩溃循环 docker stop→cp→start
