# Octopus Web Console

This directory contains the management UI for Octopus.

## Stack

- Next.js 16
- React 19
- TypeScript
- Tailwind CSS 4
- TanStack Query
- Zustand 5
- Radix UI
- `next-intl`

The app uses App Router as the shell entrypoint, but the actual screen switching inside the console is handled client-side in `src/components/app.tsx`.

## Commands

Install dependencies:

```bash
pnpm install
```

Run the frontend against a local backend:

```bash
NEXT_PUBLIC_API_BASE_URL="http://127.0.0.1:8080" pnpm dev
```

Lint:

```bash
pnpm lint
```

Build the static export used by the embedded management UI:

```bash
NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build
```

## Environment Variables

- `NEXT_PUBLIC_API_BASE_URL`: Optional API base URL. Defaults to relative requests against the current origin.
- `NEXT_PUBLIC_APP_VERSION`: Version string shown in the UI. For release builds, set this to the current git tag or commit.

## Output and Embedding

`pnpm build` produces a static export in `out/`.

The Go server embeds these files from `../static/out/`. A typical local embed flow is:

```bash
pnpm install
NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build
cd ..
mkdir -p static/out
cp -r web/out/* static/out/
```

If `static/out/_not-found/` exists but is empty, add `.keep` before running `go build` or `go run main.go start`.

The top-level `Dockerfile` already builds this frontend and copies the export into `static/out` during image build, so release images contain the matching frontend automatically.

## Key Directories

- `src/components/app.tsx`: Main application shell with login mode switching (user credentials / API key)
- `src/components/modules/home/*`: Runtime/version overview, hero summary, trend chart (multi-metric overlay: cost/count/tokens/success-rate), GitHub-style activity heatmap, ranking panel, and analytics overview cards
- `src/components/modules/remote-site/*`: Hub module — tab-based interface with 5 sub-panels: Sites (multi-account cards with inline balance / sync / check-in status, archive/restore, batch edit, bulk import), Site Channels (SiteChannelSection), Automation (SettingSiteAutomation), Balance (plan balance charts), and TokenPlan (token plan monitoring). Slimmed from the previous 7-tab layout; check-in/announcement/redemption/usage/credential panels are inlined into the Site cards or accessed via the API
- `src/components/modules/site/*`: Site management module — the multi-account site card grid rendered inside the Hub "Sites" tab. Includes `Site` (main grid + site/account dialogs), `CheckinPanel` (summary strip), `AccountEditDialog`, `SiteEditDialog`, `checkin-status`, `site-message`, and `ui-store`. Supports pin / archive / restore, batch enable/disable/delete, and bulk import from AllAPIHub / MetAPI
- `src/components/modules/site-channel/*`: Site Channels section — dedicated view for managing channels associated with remote sites (projected channel bindings)
- `src/components/modules/credential/*`: Shared `CredentialDialog` component reused by the Hub CredentialPanel
- `src/components/modules/model-mapping/*`: Model name mapping rules UI (exact/wildcard/regex pattern-based name rewriting with priority and group scope). Not registered as a top-level nav route; backed by `/api/v1/model-mapping`
- `src/components/modules/proxy-pool/*`: Proxy configuration pool management — CRUD, connectivity testing, reference tree tracking, and jump-to-reference navigation. Accessible from the app shell toolbar
- `src/components/modules/channel/*`: Channel configuration with 9 built-in templates (OpenAI, OpenAI Responses, Anthropic, Gemini, DeepSeek, OpenRouter, SiliconFlow, Volcengine, MiMo), key management, sync, latency, model declarations, proxy mode, and request rewrite profiles
- `src/components/modules/group/*`: Route groups, balancing strategy configuration, zashboard-style collapsible group list, group testing, single-group AI route append flow, AI route progress dialog, endpoint provider configuration, and CC Switch deep link generator (Claude Code, Codex, Gemini, OpenCode, OpenClaw)
- `src/components/modules/model/*`: Model Market UI with dual-tab interface (Market and Capabilities) plus an Available Endpoints view (migrated from the API key page), including summary strip, virtualized cards, multi-dimension filter (capability + provider + normalized-name dedupe), price-edit actions, and capabilities panel with endpoint support and status badges
- `src/components/modules/analytics/*`: Five tabs (Channel × Model [default], Usage Breakdown, Route Health, Latency, Cache) with usage-distribution share chart, latency distribution histogram, provider prompt cache analytics, and share snapshot (PNG export / clipboard copy via html-to-image)
- `src/components/modules/log/*`: Relay request list/detail views, usage/cost display, and error diagnostics
- `src/components/modules/notification/*`: Unified notification center — top-level route aggregating system events, alert firings, and plan notifications. Four groups: Messages (inbox / archived), Alerts (rules / history via `AlertSections`), Delivery (channels / policies / preferences), and Reports (schedules / history). Renders notification text via `notif-text.ts` and streams updates over SSE
- `src/components/modules/plan-provider/*`: Plan provider monitoring section — tracks upstream subscription quota/usage (Codex, MiMo, StepFun, SenseNova, and balance-type providers) and manages auto-created forwarding channels
- `src/components/modules/report/*`: Usage report scheduling (`ReportScheduleManager`) and report history (`ReportHistoryList`), rendered inside the Notification reports group
- `src/components/modules/alert/*`: Alert rule, notification-channel, and history UI. No longer a top-level route — its `AlertSections` component is embedded inside the Notification module's Alerts and Delivery groups
- `src/components/modules/ops/*`: Six tabs (Telemetry, Quota, Health, Maintenance, System, Audit). Telemetry includes hero metrics, P95 latency, throughput RPS, database health, session/quota activity, provider health table (sortable columns + mini bar charts), and provider prompt cache analytics. Maintenance tab consolidates Retry / Circuit Breaker / Response Filter settings (moved out of Settings page)
- `src/components/modules/apikey/*`: API key creation, allowlists, expiry, max-cost, RPM/TPM, per-model quota, and IP/CIDR allowlist controls
- `src/components/modules/apikey-dashboard/*`: API key dashboard views — request stats, token usage, cost, quota, expiration countdown, and supported models when authenticated via API key
- `src/components/modules/setting/*`: Settings cards — Info (version, self-update, version mismatch detection), Appearance (theme, locale, nav order + visibility), AI Route, Auto Strategy, Account (timezone, session), Semantic Cache, Log, System (CORS allowlist, proxy, stats interval), LLM Sync, Backup (export/import/live migration), Redis (optional cache backend, restart to apply), WebDAV Backup, WebAuthn/Passkey, Normalize (model-name normalization rules: router prefixes, functional suffixes, explicit variant→canonical mappings, with an offline AI-assisted workflow), Pool (account-pool scheduling tuning), Local Nodes (status, version, concurrency, last seen; admin-only lifecycle actions and transient connection tokens), and Proxy Pool (opens the shared proxy-pool dialog instead of an inline panel). The authoritative card list lives in `SETTING_ITEM_DEFS` in `src/components/modules/setting/index.tsx`. Supports drag-and-drop card reordering with localStorage persistence. Note: Retry/CircuitBreaker/ResponseFilter were moved to `ops/Maintenance.tsx`; SiteAutomation is rendered inside the Hub Automation tab; PurgeUnavailableModels/RouteGroupDanger moved to the Group Maintenance button
- `src/components/modules/user/*`: Management-console user and role administration
- `src/components/modules/navbar/*`: Top-level navigation state and persisted nav-order/visibility helpers
- `src/components/modules/toolbar/*`: Shared toolbar with per-page search, layout toggle (grid/list), sort options, and context-dependent filters
- `src/components/modules/login/*`: Login form supporting both username/password and API-key authentication modes behind a tabbed interface
- `src/components/modules/logo/*`: Shared animated Octopus logo component used by login and first-run screens
- `src/components/modules/first-run-setup.tsx`: Bootstrap wizard for creating the initial admin account when no admin exists, with animated particle background
- `src/api/`: API client and endpoint hooks (TanStack Query)
- `src/route/config.tsx`: UI route registration (lazy-loaded top-level modules: home, hub, channel, group, model, analytics, log, notification, ops, apikey, setting, user)
- `src/stores/`: Zustand state stores
- `src/lib/`: Utilities, i18n, logger, time zone helpers (10 time zones), service worker management
- `public/locale/`: Localized text resources (en, zh_hans, zh_hant)

## Notes

- The settings module supports drag-and-drop reordering of its 14 card sections, with order persisted to localStorage. A "Reset to Default" button restores the original order.
- Group mode labels and endpoint type display values are shared with backend behavior and should be updated together when adding new strategies or capabilities.
- Local nodes live in Settings, not a new top-level route. `GET /api/v1/nodes` returns `{nodes, can_write}`; only admins can write or rebind channels. Create (`POST /api/v1/nodes`) and rotate (`POST /api/v1/nodes/:id/rotate-token`) return one-time `{node, token}`; update uses `PUT /api/v1/nodes/:id`, delete uses `DELETE /api/v1/nodes/:id`. The admin-only `POST /api/v1/nodes/:id/test` checks the fixed `/v1/models` endpoint through the tunnel, with a short timeout and bounded response; no inference or arbitrary destination is allowed. It returns `{passed, code, latency_ms, status_code, model_count}`. Node and channel test panels show explicit failures and safe diagnostics. Configure the plugin's local port (default 3425), save Request connection, fully quit and restart Magpie before testing. Tokens stay in dialog state, never query/mutation data or browser storage. Closing/leaving the panel clears them. The template contains only a Magpie-key placeholder; use local `OCTOPUS_MAGPIE_KEY`. Channel payloads carry `node_id` for create/edit/discovery/test; node mode strips HTTP URLs/keys and disables pools/proxies. HTTP drafts keep their keys when toggling transport.
- API types in `src/api/` are hand-maintained — there is no OpenAPI/code generation, so backend DTO changes must be mirrored manually.
- i18n keys must be added to all three locale files (`public/locale/en.json`, `zh_hans.json`, `zh_hant.json`); `pnpm test:i18n` enforces key parity in both directions.
- User-visible time formatting in new code goes through `@/lib/time` (`Intl.DateTimeFormat` + the user-configurable time zone from `src/stores/setting.ts`, independent from the server-side stats timezone). Legacy components may still use local formatting, so coverage across the UI is partial.
- The build is a static export (`output: "export"`, no SSR, `images.unoptimized`); non-dev builds use a relative `assetPrefix` so the Go server can host the UI under any prefix.
- The `Model Market` views are backed by `/api/v1/model/market`, `/api/v1/model/capabilities`, and `/api/v1/model/price-category/*` + `/api/v1/model/price-schedule/*`. The market catalog unions price rows with channel-declared names, and `/api/v1/model/market?dedupe=true|false` selects canonical aggregation or exact raw-name cards. Canonical cards expose `primary_name` and `variant_names` for traceable actions. The market summary-strip metrics come from the market endpoint's full-dataset `summary` and do not change with search or filters. Scheduled price refresh lives in the Settings `LLM Sync` card.
- Normalize settings use `/api/v1/model/normalize/rules`, `/api/v1/model/normalize/generate`, `/api/v1/model/normalize/progress/:id`, `/api/v1/model/normalize/stream/:id`, and `/api/v1/model/normalize/coverage`. AI output is a complete replacement candidate shown in the editor; it is saved only through the existing confirmation flow.
- The `analytics` overview query remains available, but the primary UI summary cards for those metrics live on the Home page.
- Ops `Audit` only covers selected management write routes, not public relay traffic.
- Semantic cache settings are split into configured state and runtime-enabled state: enabling the switch alone is not enough; the embedding base URL and embedding model must also be configured before runtime metrics turn green. Cache/coalescing excludes multimodal, tool, and stateful continuation requests; output parameters are isolated, TTL is sliding idle expiry, and replay uses the client's Chat/Responses protocol.
- Key RPM/TPM limits apply globally across models in addition to model sublimits. Known upstream usage settles token reservations (including debt); media without usage reports only consumes RPM. A configured Redis quota backend fails closed with 503 when unavailable instead of granting independent memory quotas.
- Group lists carry category, condition, reasoning-buffer strategy, and test-state fields. Card/list editors use the same reasoning-buffer patch helper; member probes are connectivity checks, not full routing authorization or protocol acceptance.
- Top-level page order and visibility are edited inside the `Appearance` card, persisted through the `nav_order` and `nav_visible` settings, and normalized against `DEFAULT_NAV_ORDER`, so missing routes are appended automatically and unknown routes are dropped.
- The Hub (`remote-site`) module is a tab-based page with 5 tabs: Sites, Site Channels, Automation, Balance, and TokenPlan. The Sites tab renders the `site/` module's multi-account card grid with inline balance / sync / check-in status, archive/restore, batch edit, and bulk import. The Automation tab renders the `SiteAutomation` settings component. The Balance and TokenPlan tabs render plan-provider monitoring. The previously separate announcement / checkin / redemption / usage-history / credential panels were inlined into the Site cards or are accessed via the API.
- The `model-mapping` module provides pattern-based model name rewriting rules but is not exposed as a top-level navigation route. It is accessible from the app shell toolbar.
- The `proxy-pool` module provides named proxy configuration management with CRUD, connectivity testing, and reference tracking. It is accessible from the app shell toolbar.
- The login screen supports two authentication modes (user credentials and API key) behind a tabbed interface, and the first-run bootstrap wizard appears automatically when no admin account exists. Both the login screen and user management support WebAuthn/Passkey registration and authentication.
- The API key dashboard shows a dedicated view when authenticated via API key (instead of username/password), displaying request stats, token usage, cost, quota, and expiration info.
- The Settings `Backup` card includes a live database migration feature beyond simple export/import, with connection testing, per-table row counts, and post-migration restart reminder.
- The Settings `WebDAV` card provides automated cloud backup via WebDAV with configurable schedule, remote file management, and one-click restore.
- The Hub `Automation` tab (formerly the Settings `Site Automation` card) configures auto-sync and auto-checkin intervals for remote sites.
- The `channel` module includes per-channel proxy mode (direct/system/pool/inherit), request rewrite profiles (preserve/openai_chat_compat/codex with codex header, tool role, and system message strategies), param_override JSON for per-channel parameter injection into outbound requests, and a per-channel `skip_model_test` toggle (issue #98) to exclude the channel from group/model availability probes.
- The `group` module includes endpoint provider configuration (openai/deepseek/mimo/siliconflow/newapi) for stripping incompatible reasoning fields, outbound format with `chat_only`/`responses_only` modes that disable cross-format adapter fallback, supports a zashboard-style collapsible group list view, and includes a "Maintenance" dropdown button (Purge Unavailable Models, Delete All Route Groups) relocated from the Settings page.
- The Home page includes a GitHub-style activity heatmap visualizing request activity across the past year, and a multi-metric overlay chart (cost/count/tokens/success-rate selectable simultaneously).
- The Analytics page includes a Share button that generates a PNG snapshot of the current analytics state for download or clipboard copy.
- User-configurable time zone (10 zones) affects all date/time display in the UI, independent from server-side stats timezone.
- Viewer accounts see masked domains (`***`) for Hub-related management data across sites, remote sites, credentials, channels, and URL settings.
- AI Route settings automatically fetch model suggestions from the configured OpenAI-compatible service for both the main service and service-pool rows; model names can still be entered manually. Main-service selection or blur saves directly, while service-pool rows still require **Save Service Pool**. Single-group AI routing requires an explicit current group and has no default target group setting.
- When backend API surfaces or top-level modules change, update both `web/README.md` and the root README files so the embedded-console docs remain consistent.
- The Settings page now has 14 cards (down from 18, plus the Normalize card for model-name normalization rules and the Redis card for the optional cache backend). Response Filter, Log Level, and WebAuthn/Passkey remain; Retry/CircuitBreaker moved to Ops Maintenance; Site Automation moved to Hub Automation; PurgeUnavailableModels/RouteGroupDanger moved to Group Maintenance button.
- The `notification` module is the unified notification center and a top-level route; the former standalone `alert` route was removed and its UI now lives inside the Notification module's Alerts and Delivery groups.
