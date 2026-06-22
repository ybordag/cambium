# Cambium — Design Document

**Status:** Historical design reference — current implementation is documented
in [docs/architecture](architecture/overview.md) and
[docs/roadmap](roadmap/overview.md)
**Version:** 0.3

Read this as an implementation record, not the current source of truth. For
current route ownership and runtime behavior, start with
[Architecture Overview](architecture/overview.md), [API Surface](architecture/api-surface.md),
and [Production Readiness](operations/production-readiness.md).

---

## What it is

Cambium is the HTTP API gateway for the Gardening Agent system. It sits between
the frontend (Verdant) and the domain engine (Rhizome), handling authentication,
encrypted provider key storage, request routing, and the public HTTP contract.

The name comes from botany: the cambium is the actively dividing layer between
the inner wood and the outer bark of a plant — exactly what a gateway does.

---

## Role in the system

```
Verdant (React frontend)
    │  HTTP/JSON  /api/v1  Authorization: Bearer <token>
    ▼
Cambium (Go)
    │  verifies JWT → extracts user_id
    │  decrypts provider key for this user
    │  calls Rhizome over internal HTTP
    ▼
Rhizome (Python — domain engine)
    │  { user_id, provider, provider_key, ... }
    │  threads user_id through graph and tools
    │  uses provider_key for all LLM calls
    ▼
Postgres
    ├── cambium schema  (users, refresh_tokens — owned by Cambium)
    └── rhizome schema  (domain tables — owned by Rhizome)
```

Cambium and Rhizome are **separate processes**. Cambium calls Rhizome over a
well-defined internal HTTP interface. Streaming uses SSE today; gRPC remains a
possible future service-to-service option if the protocol needs bidirectional
streaming. They share one Postgres instance under separate schemas.

---

## Language

Go. Reasons:

- Natural fit for HTTP gateway work: goroutines, fast startup, low memory
- Strong standard library for JWT, bcrypt, AES crypto — minimal external dependencies
- Single static binary — fits the Spark hardware deployment model
- Good portfolio signal for infrastructure/backend roles
- Fairlead is the separate Rust inference-router track

---

## Database design

### Shared Postgres instance

One Postgres instance, two schemas — avoids running parallel databases on Spark
hardware while maintaining clean ownership boundaries:

```
postgres (port 5432)
  ├── cambium   — users, refresh_tokens
  └── rhizome   — all domain tables
```

Cambium never queries the `rhizome` schema. Rhizome never queries `cambium`.
Cross-schema reads are not needed because Cambium injects `user_id` into every
Rhizome request — Rhizome scopes all queries to that ID.

### `cambium.users`

```sql
CREATE TABLE cambium.users (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                   TEXT UNIQUE NOT NULL,
    password_hash           TEXT NOT NULL,           -- bcrypt, cost ≥ 12
    preferred_provider      TEXT,                    -- 'gemini' | 'openai' | 'anthropic'
    preferred_model         TEXT,                    -- optional e.g. 'gemini-1.5-pro'
    encrypted_gemini_key    TEXT,                    -- AES-256-GCM ciphertext, nullable
    encrypted_openai_key    TEXT,                    -- nullable
    encrypted_anthropic_key TEXT,                    -- nullable
    created_at              TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### `cambium.refresh_tokens`

```sql
CREATE TABLE cambium.refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES cambium.users(id),
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMP NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    revoked_at  TIMESTAMP
);
```

---

## Auth design

Custom JWT — no third-party provider. Reasons:

- Full control over auth stack and user data
- Everything runs on owned hardware (Spark) — no external auth traffic
- Educational value: implements the full backend auth lifecycle

### Token flow

1. `POST /auth/register` — hash password with bcrypt (cost 12), insert user, issue tokens
2. `POST /auth/login` — verify bcrypt hash, issue tokens
3. **Access token:** short-lived JWT (15 min), HS256, `user_id` in `sub` claim. Sent as `Authorization: Bearer <token>` header. Frontend stores in `localStorage`.
4. **Refresh token:** long-lived (7–30 days), stored as a hash in `refresh_tokens`, sent as `httpOnly` cookie. Rotated on every use — on refresh, old token is revoked and a new one is issued atomically.

### Account management endpoints

`PATCH /auth/profile` (requires JWT) — update `preferred_provider` and/or `preferred_model` on the user record. At least one field required. Returns updated session object.

`POST /auth/password` (requires JWT) — change password. Body: `{ current_password, new_password }`. Verifies current password via bcrypt before updating. Returns 401 on wrong current password (same generic error as login — prevents enumeration). New password hashed at bcrypt cost 12.

### HS256 now, RS256 later

HS256 (shared secret) for the initial implementation. Migrate to RS256 (asymmetric)
if Fairlead or other services need to independently verify tokens without a shared
secret.

---

## Provider key storage

Users bring their own LLM API keys (Gemini, OpenAI, Anthropic). Cambium stores
them encrypted so Rhizome can use the right key for each user without any user
touching the server's `.env`.

### Encryption

AES-256-GCM using Go's standard library `crypto/aes`. The master key is
`CAMBIUM_ENCRYPTION_KEY` — a 32-byte server-side secret that never leaves
Cambium and is never returned to clients.

```
plaintext key  →  AES-256-GCM(CAMBIUM_ENCRYPTION_KEY)  →  nonce || ciphertext
```

The nonce is prepended to the ciphertext and stored in a single `TEXT` column
(hex or base64 encoded). On read: split nonce, decrypt.

### Key management endpoints

```
PUT    /api/v1/auth/keys         { "provider": "gemini", "key": "AIza..." }
GET    /api/v1/auth/keys         → { "gemini": true, "openai": false, "anthropic": false }
DELETE /api/v1/auth/keys/gemini
```

Keys are **never returned** to the client. The GET endpoint only indicates which
providers are configured. This way a compromised access token cannot leak the
underlying API key.

### Key injection into Rhizome requests

On every proxied request, Cambium:
1. Reads `preferred_provider` from the user row
2. Decrypts the corresponding encrypted key
3. Includes `provider` and `provider_key` in the Rhizome internal request body
4. Never logs or caches the decrypted key

Rhizome's model factory (`agent/core/model.py`) is updated to accept
`provider` + `provider_key` from request context and falls back to env vars
when running locally without Cambium.

### Key rotation

`CAMBIUM_ENCRYPTION_KEY` rotation requires re-encrypting all stored keys.
Design a rotation script before storing real user keys in production. The
rotation pattern: decrypt all keys with the old key, re-encrypt with the new
key, rotate the env var, deploy atomically.

---

## Rhizome internal interface

### Two surfaces

Rhizome exposes two FastAPI routers on its internal interface. Cambium routes to
the right one based on what the request needs:

```
Cambium
  ├── AI operation? → POST /internal/agent   (LangGraph graph execution)
  └── CRUD?         → GET/POST /internal/data/...  (direct SQLAlchemy, no LLM)
```

**Agent endpoint** — for requests that require LangGraph reasoning (triage, interaction
resolution, care analysis, incident triage, complex conversational queries):

```json
POST /internal/agent
{
  "user_id":      "abc-123",
  "thread_id":    "thread-xyz",
  "message":      "What should I do today?",
  "provider":     "gemini",
  "provider_key": "<decrypted — never logged>",
  "model":        "gemini-1.5-flash"
}
```

**Data endpoint** — for simple reads and status mutations that don't need AI
reasoning (list tasks, get project progress, complete a task, view activity history).
These bypass the LangGraph agent entirely — no LLM call, just a DB query:

```
GET  /internal/data/tasks?project_id=...
GET  /internal/data/tasks/daily
GET  /internal/data/projects/{id}
GET  /internal/data/projects/{id}/progress
POST /internal/data/tasks/{id}/complete
POST /internal/data/tasks/{id}/defer
...
```

This split matters for performance and cost. A `GET /api/v1/tasks` request from
Verdant should not spin up a full LangGraph agent turn — it should be a single
SQL query. Only operations that require AI reasoning pay the LLM cost.

Rhizome's agent and data endpoints are implemented. Cambium proxies AI-heavy
operations to the agent surface and structured reads/status mutations to the
data surface.

### Rhizome instance topology

Rhizome instances are stateless — domain data and LangGraph conversation checkpoints
both live in Postgres. Cambium can route any request to any available Rhizome
instance. No sticky sessions. See `rhizome/docs/architecture/deployment.md` for
the full topology, scaling model, and future Temporal evolution.

### Streaming: SSE over HTTP

Token streaming uses **Server-Sent Events (SSE)** over HTTP — not gRPC. SSE is
the right choice because:

- Browsers consume SSE natively (`EventSource` / `fetch` with `ReadableStream`)
- gRPC requires a proxy (Envoy + grpc-web) for browser clients — real operational overhead
- SSE is plain HTTP with `Content-Type: text/event-stream`; no protocol change needed
- OpenAI, Anthropic, and every major AI chat API use SSE for streaming

Rhizome exposes two additional streaming endpoints alongside the standard ones:

```
POST /internal/agent/stream        — SSE stream of tokens for a new message
POST /internal/agent/resume/stream — SSE stream of tokens after interaction resume
```

Each emits typed events:

```
data: {"type": "token",       "content": "The "}
data: {"type": "token",       "content": "garden "}
data: {"type": "interaction", "payload": {...}}   # when graph pauses for user input
data: {"type": "done"}
```

Cambium proxies the SSE stream directly to Verdant with no buffering:

```go
// Forward Rhizome's SSE stream to the client
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
resp, _ := http.Post(rhizomeURL+"/internal/agent/stream", "application/json", body)
defer resp.Body.Close()
io.Copy(w, resp.Body)
w.(http.Flusher).Flush()
```

Verdant reads the stream with `fetch` + `ReadableStream`:

```javascript
const resp = await fetch('/api/v1/chat/stream', { method: 'POST', body: ... })
const reader = resp.body.getReader()
while (true) {
    const { done, value } = await reader.read()
    if (done) break
    const event = JSON.parse(new TextDecoder().decode(value).replace('data: ', ''))
    if (event.type === 'token') appendToChat(event.content)
}
```

**gRPC** remains the future path if Fairlead or other backend services need to
call Rhizome with streaming — purely service-to-service, no browser in the chain.
For the current architecture (browser → Cambium → Rhizome), SSE is correct.

---

## Build order

### Phase 0 — Postgres setup ✓ complete

Postgres 16 running in Docker. `cambium` and `rhizome` schemas created. Rhizome migrated to Postgres.

### Phase 1 — Go project skeleton ✓ complete

Go module, `/health`, pgxpool, `cambium` schema migrations. (commit 0f06cc8)

### Phase 2 — Auth + key management ✓ complete

Register, login, refresh, session, logout. JWT middleware. AES-256-GCM key management. (commit 5ee7575)

### Phase 3 — Rhizome proxy ✓ complete

HTTP client (`DataGet`, `DataPost`, `RunAgent`, `StreamAgent`). SSE streaming proxy. Provider key injection. (commit 1d3bc74)

### Phase 4 — Full API surface ✓ complete

Core route surface wired. AI-trigger handlers (triage, weather, treatment
plans, task generation). Media stubs. Full Swagger docs. (commit 6a916a6)

### Phase 5 — Thread management ✓ complete

Botanical name generator (31×41×36 ≈ 45,700 combinations). `POST/GET/DELETE/GET-messages /api/v1/threads`. (fibril → main)

### Frontend API pass ✓ complete

Expanded route surface. All new Rhizome endpoints wired: task CRUD, task series,
task dependencies, bulk task update, garden detail endpoints, available
resource filters, project beds/containers/expenses/shopping, calendar
annotations, shopping list, activity stats. `TestAllProtectedRoutesReject401`
security sweep expanded to cover protected routes.

### Group B + account ✓ complete

Quick care recording (`POST .../care`) for plants/beds/containers. Incident PATCH/DELETE, manual treatment plan POST/PATCH/DELETE. `PATCH /auth/profile` and `POST /auth/password` as Cambium-native handlers (no Rhizome proxy).

### Unified search + thread context ✓ complete (#16)

`GET /api/v1/search` proxy; `POST/DELETE /api/v1/threads/{id}/context`; `GET/PATCH /api/v1/threads/{id}/session-context` for normalized startup/session context; `initial_context` pass-through on thread creation with Rhizome 400-detail propagation. Fixed a pre-existing bug where `proxyData`/`proxyDataWithPathParam` collapsed every non-GET request to POST before forwarding, silently breaking PATCH/DELETE proxy routes in production — replaced with method-preserving `DataRequest`.

### Notification SSE stream + sync endpoint ✓ complete (#19)

`GET /api/v1/notifications/stream` proxies Rhizome's long-lived SSE notification stream; `GET /api/v1/notifications` proxies the sync snapshot. Required a new client method, `StreamData` (GET with query params, mirrors `openStream` but without a JSON body).

### Static frontend serving ✓ complete (#21)

`internal/api/static.go` serves the built Verdant Pages `dist/` (`STATIC_DIR` env var, default `./dist`) for any path not claimed by a more specific route; unknown paths fall back to `index.html` for client-side routing. Registered as the catch-all `"/"` pattern — Go's `ServeMux` always prefers the most specific match, verified by a router-level precedence test.

These follow-on branches are merged into `main`.

---

## Open questions

1. **Rate limiting** — auth endpoints need brute-force protection before any public exposure. `golang.org/x/time/rate` or a middleware library.

2. **`CAMBIUM_ENCRYPTION_KEY` rotation** — design the re-encryption script before going to production with real user keys.

3. **gRPC migration trigger** — SSE over HTTP is correct for the current browser → Cambium → Rhizome path. Switch if Fairlead needs service-to-service streaming.

4. **Notification SSE event bus — resolved, with a known limitation.** Shipped (#19/#19-rhizome) using per-process in-memory queues (`agent/domain/notifications.py`), not the Postgres `LISTEN/NOTIFY` design originally proposed here. This means live SSE delivery only reaches a connection on the *same* Rhizome instance that handled the triggering job — it will not work across the two k3s pods once multi-instance deployment happens. The gap is partially covered today: anything that matters is also written to `MonitorAlert`/`InteractionRecord` and recovered via `GET /api/v1/notifications` on reconnect/poll, but the in-memory `active_jobs` snapshot (job-in-progress state) has no cross-instance or `since`-filtered recovery path. Revisit `LISTEN/NOTIFY` (or Redis pub/sub) before running more than one Rhizome instance.

---

## Resolved design decisions

| Question | Decision | Rationale |
|---|---|---|
| `users` table location | Same Postgres instance, `cambium` schema | Simpler to operate on Spark hardware; single backup; no cross-service joins needed |
| Refresh token storage | Separate `refresh_tokens` table | Supports multiple devices/sessions; easy to list and revoke all sessions |
| Access token delivery | `Authorization: Bearer` header; frontend stores in `localStorage` | Standard REST pattern; simpler CORS setup; acceptable XSS trade-off for a portfolio project |
| Provider key storage | Encrypted in `users` table (AES-256-GCM) | Keys at rest are never plaintext; keys are never returned to the client |
| Postgres shared vs. separate | Shared instance, separate schemas | One Docker container on Spark; single backup target; Cambium and Rhizome stay independently deployable |
