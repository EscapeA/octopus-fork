# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Runtime and build commands

### Backend
- Start the server: `go run main.go start`
- Start with an explicit config file: `go run main.go start --config /path/to/config.json`
- Build: `go build ./...`
- Show embedded build metadata: `go run main.go version`

### Frontend
- Install deps: `cd web && pnpm install`
- Dev server (against local backend): `cd web && NEXT_PUBLIC_API_BASE_URL="http://127.0.0.1:8080" pnpm dev`
- Lint: `cd web && pnpm lint`
- Build static export: `cd web && NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build`

### Tests
- All tests: `go test ./...`
- Single package: `go test ./internal/relay/...`
- Single test: `go test ./internal/transformer/model -run TestEmbeddingInput_MarshalJSON`
- With race detector: `go test -race ./internal/relay/...` (requires CGO)

### Frontend tests
- Full check (lint + i18n + unit + build): `cd web && pnpm check`
- Unit tests only: `cd web && pnpm test:unit`
- i18n completeness: `cd web && pnpm test:i18n`
- Lint: `cd web && pnpm lint`

### Release
- Canonical release build: `bash scripts/build.sh release`
- Single-platform build: `bash scripts/build.sh build linux x86_64`

### Embedded management UI
The Go server serves the web UI only when static assets exist under `static/out/` (embedded by `static/static.go` via `//go:embed out out/**`). Build frontend first, copy `web/out/*` into `static/out/`, then `go build`. If `static/out/_not-found/` exists but is empty, add `.keep` to prevent build errors (the release script handles this).

### Quality gates
Root `package.json` provides Husky hooks (`commit-msg`, `pre-commit`) and lint-staged config. Commit messages are validated by commitlint (`.commitlintrc.json`). Go linting uses golangci-lint (`.golangci.yml`). Pre-commit runs `lint-staged` which formats/checks staged files.

## High-level architecture

### Request surfaces
- `/api/v1/bootstrap/...` — Unauthenticated first-run bootstrap (before any admin exists).
- `/api/v1/...` — Management API with JWT auth and RBAC (`admin` / `editor` / `viewer`). Handlers register routes via `init()` in `internal/server/handlers/`.
- `/v1/...` — Public relay API. OpenAI-compatible and Anthropic-compatible, authenticated with Octopus API keys (`sk-octopus-...`). Accepts `Authorization: Bearer ...` and `x-api-key`.

### Startup sequence
`main.go` → Cobra commands in `cmd/`. `cmd/start.go`: loads config (Viper from `data/config.json`, `OCTOPUS_...` env overrides), initializes DB, runs migrations, warms caches via `op.InitCache()`, restores balancer runtime state, bootstraps initial admin, starts Gin, launches background tasks.

If `auth.jwt_secret` / `OCTOPUS_AUTH_JWT_SECRET` is unset, startup generates an ephemeral JWT secret — admin web sessions invalidate on restart. Initial admin can be bootstrapped via `OCTOPUS_INITIAL_ADMIN_USERNAME` and `OCTOPUS_INITIAL_ADMIN_PASSWORD`.

### Core domain model (`internal/model/`)
- **Channel** — One upstream provider: type, base URLs, keys, model declarations, custom headers, rewriting rules, auto-sync/grouping.
- **Group** — Routing policy: ordered/weighted `GroupItem`s, endpoint type, optional regex matching, optional JSON condition, first-token timeout, sticky-session TTL.
- **APIKey** — Client credential (`sk-octopus-...`): supported model allowlists, expiry, max-cost caps, RPM/TPM limits, optional per-model quotas.
- **User** — Management-console identity with server-enforced role (`admin`, `editor`, `viewer`).
- **AlertRule/AlertNotifChannel/AlertStateRecord/AlertHistory** — Webhook-based alert definitions, notification targets (webhook, Gotify, email), current state, and history.
- **Setting** — Runtime tuning: retry, circuit breaker, auto-strategy, sync intervals, log retention, AI-route service pool, semantic-cache knobs, database migration switching.
- **RemoteSite** — Remote AI relay site managed by the Hub: base URL, site type, credentials (AES-GCM encrypted), health status, quota, exchange rate, pinned/sort order.
- **BalanceSnapshot / CheckInRecord** — Hub balance tracking (daily quota snapshots) and auto check-in records per remote site.
- **APICredentialProfile** — Reusable API credential (base URL + key pair) with health verification and CLI export support.
- **SiteAnnouncement / RemoteSiteToken** — Hub announcement cache and remote API token sync (encrypted keys).

### Relay pipeline
1. `/v1/...` handler picks an inbound protocol adapter (`relay.go` for LLM, `media.go` for media/utility).
2. `internal/relay/relay.go` (or `media_relay.go`) parses the inbound payload. A per-request `retryRequestCache` is created to deduplicate semantic-lookup and embedding computation across retries.
3. API-key middleware injects supported-model filters and quota metadata.
4. Requested model is resolved to a `Group` via `internal/op`.
5. `internal/relay/balancer/` builds candidate iterator using group mode (round robin, random, failover, weighted, auto), sticky sessions, and circuit breaker state.
6. A `Channel` and channel key are selected, with retry/cooldown logic controlled by settings. Failure hints from the retry cache steer candidates away from recently-failed (channel, key, model) tuples. Eligible non-stream requests may be deduplicated via `singleflight`.
7. Outbound adapter in `internal/transformer/outbound/` converts to the target provider format and forwards.
8. Request rewrite engine (`internal/transformer/rewrite/`) normalizes incompatible formats before outbound transformation.
9. Response usage, stats, relay logs, channel-key state, and sticky/circuit-breaker data are recorded.

### Error classification and retry (`internal/relay/type.go`)

| Scope | Trigger | Behavior |
|-------|---------|----------|
| `ScopeNone` | Success, 400 errors | No retry |
| `ScopeSameChannel` | 401/403/429, network errors | Retry with different key on same channel |
| `ScopeNextChannel` | 404/5xx, timeouts, transformer errors | Try next candidate channel |
| `ScopeAbortAll` | Stream already written to client | Stop all retries |

`RetryDecision` carries scope, reason, status code, and error flag. Max retries per candidate (default 3), rate-limit cooldown (default 300s), and max total attempts (default unlimited) are configurable via DB settings.

### Retry cache and inflight dedup (`internal/relay/retry_cache.go`)

Per-request `retryRequestCache` caches semantic-cache lookup inputs and embeddings so they are computed once per relay request, not once per retry attempt. A global `singleflight.Group` deduplicates identical in-flight relay requests (restricted to non-streaming requests without tool-use). A short-lived failure-hint cache records recent 429, 401/403, and network failures by `(channelID, keyID, modelName)`, allowing `retry_helper.go` to skip known-bad candidates during the same request lifecycle.

### Circuit breaker (`internal/relay/balancer/circuit.go`)

Three-state breaker per `(channelID, keyID, modelName)`, stored in a global `sync.Map`:

- **Closed** — Normal operation. Transitions to Open after N consecutive failures (default 5).
- **Open** — All requests rejected. Cooldown = base × 2^(tripCount-1), capped at 10 minutes. Transitions to Half-Open after cooldown expires.
- **Half-Open** — One probe request allowed. Success → Closed; failure → back to Open with incremented trip count.

State is persisted to DB via balancer runtime state save and restored at startup.

### Balancer strategies (`internal/relay/balancer/`)

Five strategies: `RoundRobin`, `Random`, `Failover`, `Weighted`, `Auto`. The Auto strategy uses a ring-buffer sliding window (default 100 samples, 5-minute window) with exploration/exploitation phases. Sticky sessions remember `(apiKeyID, modelName) → (channelID, keyID)` within a configurable TTL.

### Stream session system (`internal/relay/stream_session.go`)

In-memory SSE replay buffer with conversation-level concurrency guards. One active generation per `(apiKeyID, conversationID)`. Clients reconnect with `ResumeFromEventID`. Sessions self-delete 30 minutes after completion.

### Protocol adapters (`internal/transformer/`)

**Inbound** — `TransformRequest(client→internal)`, `TransformResponse(internal→client)`, `TransformStream(internal SSE→client SSE)`.
**Outbound** — `TransformRequest(internal→upstream HTTP)`, `TransformResponse(upstream HTTP→internal)`, `TransformStream(upstream SSE→internal)`.

Inbound adapters are registered by `InboundType` in `internal/transformer/inbound/register.go`; outbound adapters by `OutboundType` in `internal/transformer/outbound/register.go`. Each call to `Get()` creates a new adapter instance (factory pattern, not singleton).

**Inbound adapters**: `openai`, `anthropic`.
**Outbound adapters**: `openai`, `anthropic`, `gemini`, `volcengine`, `mimo`.

The zen/... model prefix steers candidate provider types and upstream model resolution.

### Condition evaluator (`internal/relay/condition/`)
Group-level JSON condition matching logic, used by the relay pipeline to evaluate whether a request matches a group's routing conditions.

### Domain operations (`internal/op/`)
Business logic is split into focused subpackages such as `airoute`, `alert`, `analytics`, `apikey`, `audit`, `backup`, `cacheusage`, `channel`, `credential`, `dbmigration`, `group`, `llm`, `navorder`, `ops`, `ratelimitstore`, `relaylog`, `remotesite`, `setting`, `stats`, and `user`. Prefer updating the relevant subpackage first, then any compatibility wrapper in `internal/op/` if one exists.

### Hub — Remote site management (`internal/hub/`, `internal/op/remotesite/`)
Multi-site account management for upstream AI relay providers. Features: site CRUD, balance tracking, auto check-in, announcement aggregation, remote token sync, channel migration, API credential management, CLI export, and site discovery. Adapter pattern with 15-method `SiteAdapter` interface; common adapter covers New API/One API family as default fallback. Credentials encrypted via AES-256-GCM. See `internal/hub/README.md` for full API reference.

### Media and utility relay
- `internal/server/handlers/media.go` exposes direct relay endpoints for images, audio, video, music, search, rerank, and moderation.
- `internal/relay/media_relay.go` resolves groups by `endpoint_type`, forwards JSON/multipart payloads, streams JSON/SSE/binary responses.
- Image edits/variations and audio transcription are multipart; all others are JSON.

### Auth model
- Admin UI: JWT tokens validated by `internal/server/middleware/auth.go`, created in `internal/server/auth/auth.go`. Permissions in `auth/permissions.go`, RBAC enforced by `middleware/rbac.go`. Role is reloaded from DB each request. Login handler distinguishes transient DB errors (service unavailable) from credential failures (unauthorized).
- Relay: API keys with `sk-octopus-...` prefix. `Authorization: Bearer ...` for OpenAI-style, `x-api-key` for Anthropic-style.

### Internal packages overview

| Package | Purpose |
|---------|---------|
| `internal/client` | Shared HTTP client construction |
| `internal/conf` | Config loading (Viper, env overrides) |
| `internal/db` | DB initialization, auto-migration, dialect detection |
| `internal/helper` | Async channel/fetch/price/ai-route helpers |
| `internal/hub` | Remote site adapter interface, registry, HTTP client, common/octopus/ldoh adapters |
| `internal/model` | Domain types (Channel, Group, APIKey, User, Alert, Setting, RemoteSite, etc.) |
| `internal/op` | Business logic subpackages (see Domain operations) |
| `internal/price` | Model pricing presets |
| `internal/relay` | Core relay pipeline, retry, caching, balancer, stream sessions |
| `internal/server` | HTTP server, router, handlers, middleware, auth, response format |
| `internal/task` | Background periodic jobs |
| `internal/transformer` | Inbound/outbound protocol adapters, rewrite engine, internal model types |
| `internal/update` | Self-update mechanism |
| `internal/utils` | Shared utility functions |

### Background work (`internal/task/init.go`)
Periodic jobs based on DB settings: model price refresh, base URL latency probing, upstream model sync, stats flush, balancer runtime state flush, relay log flush, alert rule evaluation, Hub balance capture (6h), Hub auto check-in (12h), Hub announcement fetch (4h). Channel lifecycle triggers async helpers in `internal/helper/` (HTTP client construction, model listing, price hydration, AI routing generation).

### Migration system (`internal/db/migrate/`)
Explicit versioned migrations (`001` through `009`) with `RegisterBeforeAutoMigration` and `RegisterAfterAutoMigration` hooks. Dialect-specific SQL for SQLite/MySQL/Postgres. Tracks status in `migration_records`, skips already-successful entries. Runtime database type switching (SQLite ↔ MySQL ↔ Postgres) is supported via `internal/op/dbmigration/` with export/import from the Settings UI.

### Response format (`internal/server/resp/`)
Management API uses `{code: int, message: string, data: any}` envelope. `resp.Success(c, data)` and `resp.Error(c, statusCode, err)`.

## Frontend structure
- Next.js 16.0.7 (App Router shell) with `output: "export"` static export. React 19.2.1 with compiler enabled (`babel-plugin-react-compiler`).
- Key libraries: TanStack Query 5 (server state), Zustand 5 (client state), Radix UI (primitives), Tailwind CSS 4, `motion` 12 (animation), `recharts` 2.15 (charts), `next-intl` 4.5 (i18n), `next-themes` (dark mode), `@hello-pangea/dnd` (drag-and-drop), `@lobehub/icons` (provider icons), `sonner` (toasts), `react-day-picker` 9, `dayjs` (dates).
- App shell: `web/src/components/app.tsx`. Lazy-loaded modules registered in `web/src/route/config.tsx`.
- API client: `web/src/api/client.ts` (base URL defaults to `.`, override via `NEXT_PUBLIC_API_BASE_URL`).
- i18n: Locale files in `web/public/locale/{en,zh_hans,zh_hant}.json`. Use `useTranslations('namespace')` from next-intl.
- Modules: Home, Hub (Sites, Check-in, Announcement, Redemption, Usage, Credential tabs), Channel, Group, Model (Model Market), Analytics, Log, Alert, Ops, APIKey, Setting, User.
- Settings includes Semantic Cache controls and Database Migration switcher; page ordering lives inside Appearance card.

## Test patterns
- **Database isolation**: Use `t.TempDir()` + `glebarez/sqlite` (pure-Go SQLite, no CGO). See `internal/db/migrate/003_test.go` for the canonical example. Hub tests use a helper: `dbPath := filepath.Join(t.TempDir(), "test.db"); internaldb.InitDB("sqlite", dbPath, false)`.
- **Mock upstream APIs**: Use `net/http/httptest.NewServer()` — see `internal/helper/group_probe_test.go` and `internal/hub/httpclient_test.go`.
- **Global state reset**: Many packages have global caches/maps. Tests save and restore state using snapshot/restore helpers (e.g., `withCleanRegistry` in `internal/hub/registry_test.go`, `resetKey` in `internal/utils/crypto/crypto_test.go`).
- **Time control**: Tests that depend on timing override time functions.
- **No `TestMain`**: Each test is self-contained with its own state management.
- **Source-level invariants**: Backup tests read their own source file via `runtime.Caller` and use `strings.Contains` to assert table names appear in delete/export order (`internal/op/backup/backup_test.go`).

## Files to inspect first for common tasks

| Area | Key Files |
|------|-----------|
| Startup / wiring | `main.go`, `cmd/start.go` |
| Config | `internal/conf/config.go` |
| DB init / migrations | `internal/db/db.go`, `internal/db/migrate/` |
| DB type switching | `internal/op/dbmigration/`, `internal/model/backup.go` |
| Route registration | `internal/server/server.go`, `internal/server/router/router.go`, `internal/server/handlers/` |
| Relay pipeline | `internal/relay/relay.go`, `internal/relay/media_relay.go`, `internal/relay/type.go` |
| Retry cache / dedup | `internal/relay/retry_cache.go`, `internal/relay/retry_helper.go` |
| Balancer / circuit breaker | `internal/relay/balancer/` |
| Condition evaluator | `internal/relay/condition/evaluator.go` |
| Stream session | `internal/relay/stream_session.go` |
| Protocol adapters | `internal/transformer/inbound/`, `internal/transformer/outbound/` |
| Request rewriting | `internal/transformer/rewrite/` |
| Semantic cache | `internal/relay/semantic_cache.go` |
| Cache-backed operations | `internal/op/cache.go`, `internal/op/` |
| Domain operations | `internal/op/airoute/`, `internal/op/analytics/`, `internal/op/channel/`, `internal/op/group/`, `internal/op/relaylog/`, `internal/op/stats/` |
| RBAC / auth | `internal/server/auth/permissions.go`, `internal/server/middleware/auth.go`, `internal/server/middleware/rbac.go` |
| Middleware | `internal/server/middleware/` (auth, rbac, cors, audit, rate_limit, security, logger, static, validate) |
| Alerting | `internal/server/handlers/alert.go`, `internal/task/alert.go`, `internal/op/alert/` |
| Analytics / Ops / Audit | `internal/server/handlers/analytics.go`, `internal/server/handlers/ops.go`, `internal/server/handlers/audit.go` |
| Async helpers | `internal/helper/` (channel, fetch, price, ai_route) |
| Hub / Remote sites | `internal/hub/`, `internal/op/remotesite/`, `internal/op/credential/`, `internal/server/handlers/remote_site.go` |
| HTTP client | `internal/client/http.go`, `internal/hub/httpclient.go` |
| Price presets | `internal/price/` |
| Self-update | `internal/update/` |
| Response format | `internal/server/resp/` |
| Frontend entry | `web/src/components/app.tsx`, `web/src/route/config.tsx`, `web/src/api/client.ts` |
| Embedded static | `static/static.go` |