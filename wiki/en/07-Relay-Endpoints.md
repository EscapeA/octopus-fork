# Relay Endpoints, Proxy Pool & Model Mapping

### 🌐 Public Relay Endpoints

The public relay API supports both OpenAI-style and Anthropic-style clients:

- OpenAI-style clients: `Authorization: Bearer sk-octopus-...`
- Anthropic-style clients: `x-api-key: sk-octopus-...`

| Category | Paths | Notes |
|----------|-------|-------|
| OpenAI-compatible LLM | `/v1/chat/completions`, `/v1/responses`, `/v1/embeddings`, `/v1/models` | JSON request / response |
| Anthropic-compatible LLM | `/v1/messages` | Anthropic-style request / response |
| JSON media / utility | `/v1/images/generations`, `/v1/audio/speech`, `/v1/videos/generations`, `/v1/music/generations`, `/v1/search`, `/v1/rerank`, `/v1/moderations` | Uses the same group / retry / circuit-breaker pipeline |
| Multipart media | `/v1/images/edits`, `/v1/images/variations`, `/v1/audio/transcriptions` | Multipart upload forwarding |

JSON media endpoints can also proxy upstream SSE streams when the provider supports `stream=true`.

Key RPM/TPM limits apply across models together with per-model sublimits, including configurations with only model limits. Known upstream usage settles TPM reservations and may create token debt. Media without usage consumes RPM without invented token counts. A configured Redis quota backend returns 503 when unavailable or admission is uncertain instead of granting separate memory quotas. Cache/replay requests consume ingress RPM but do not duplicate upstream TPM or cost.

Recovery cursors return 409 for missing, trimmed, or replay-disabled sessions; retrying a failed session without a cursor starts a new execution. Recovery rechecks current categories and conditions. Media retains provider-specific rewriting and response forwarding, supports API key/Bearer/cookie accounts, and rejects OAuth media protocols requiring unsupported adapters. Attempt timeout includes the response body; a media stream's first-token timeout means its first response data byte.

Semantic cache is currently evaluated for non-streaming and streaming OpenAI Chat and OpenAI Responses text requests (streaming cache hits replay from the SSE session buffer). Anthropic, embeddings, and media / utility requests bypass the cache and continue through the normal relay flow.

**Zen Direct Model Routing:**

Requests with model name prefixed `zen/<model>` bypass group model mapping and route directly to the upstream model. Octopus performs smart channel-type detection based on the model name (e.g., Claude → Anthropic, Gemini → Gemini, GPT → OpenAI).

**Response ID Affinity:**

Gateway affinity for `previous_response_id` is not implemented. Passthrough preserves legitimate upstream fields, but does not guarantee the same upstream account on follow-up requests. Stateful continuation requires a route pinned to a compatible upstream.

**Model Mapping:**

Unscoped rules match once before group resolution and prefer a group named by the target. If absent, the original group may be used with an upstream-model override. Group-scoped rules match the original client model after group selection and override upstream models without selecting another group. The first matching rule wins; mappings are not recursive. Allowlists and model subquotas use the client model, while cooldown and health use the upstream model.

---

### 🌍 Proxy Pool

A shared proxy configuration pool accessible from the app shell toolbar:

- Named proxy configurations with URL, scheme (SOCKS5 / HTTP / HTTPS), enable/disable, and remarks
- 4 proxy modes: `direct`, `system`, `pool`, `inherit`
- Proxy connectivity testing against a configurable test URL
- **Reference tree** showing which sites, site accounts, managed channels, and channels use each proxy
- Jump-to-reference navigation that deep-links to the referencing entity
- Deletion protection when a proxy has active references

---

### 🔁 Model Mapping

Rules support unscoped name routing before group selection and scoped upstream overrides afterward; see the mapping order above:

- **Match types**: Exact, Wildcard (glob), and Regex
- **Target model**: the rewritten model name
- **Priority ordering**: rules are evaluated in priority order
- **Group scope**: optionally apply only to a specific group
- **Enable/disable toggle** per rule

---

| [← Home](../Home.md) | [← Previous](06-Model-Market.md) | [Next →](08-Analytics.md) |