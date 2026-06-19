# Cambium — Claude Code Memory

## What this project is

Cambium is the HTTP API gateway for the Gardening Agent system, written in Go.
It sits between the Verdant frontend and the Rhizome domain engine, handling:

- User authentication (JWT, bcrypt password hashing, refresh token rotation)
- Encrypted provider key storage (users bring their own Gemini/OpenAI/Anthropic keys)
- Request routing to Rhizome over an internal HTTP interface
- Stable versioned JSON DTOs for the frontend

See `docs/design.md` for the full architecture and design decisions.

## Related repos

- **Rhizome** (Python) — the agent and domain engine. Cambium calls it over HTTP.
- **Verdant** — React frontend. Calls Cambium over `/api/v1`.
- **Fairlead** — inference router (Go or Rust, TBD). Not yet built.

## Tech stack

- **Language:** Go 1.25
- **Routing:** standard library `net/http` (Go 1.22+ enhanced ServeMux with method+path patterns)
- **Database driver:** `github.com/jackc/pgx/v5` (connection pooling via `pgxpool`)
- **JWT:** `github.com/golang-jwt/jwt/v5`
- **Password hashing:** `golang.org/x/crypto/bcrypt` (cost ≥ 12)
- **Key encryption:** AES-256-GCM via standard library `crypto/aes`
- **Database:** Postgres — `cambium` schema (users, refresh_tokens); Rhizome tables in `rhizome` schema on the same instance
- **Internal Rhizome interface:** HTTP initially, gRPC when streaming is needed

## Build and test

```
go build ./...
go test ./...
```

No virtual environment needed — Go resolves dependencies from `go.mod`/`go.sum` automatically.

## Current status

- **Phase 0** ✓ — Postgres 16 in Docker (`rhizome-pg`, port 5432), Rhizome migrated, 325 tests passing
- **Phase 1** ✓ — Go module, `/health`, pgxpool connection, cambium schema migrations
- **Phase 2** ✓ — auth endpoints, JWT middleware, AES-256-GCM key management (lenticel, commit b77a850)
- **Phase 3** in progress (`lenticel` branch) — Rhizome HTTP client, JWT→user_id + provider_key injection, stub data + agent proxy endpoints
- **Phase 4** pending — full API surface

## Project layout

```
cmd/
  server/
    main.go              — entry point: wires DB, runs migrations, starts HTTP server
internal/
  api/
    routes.go            — route registration
    health.go            — GET /health
    middleware.go        — JWT auth middleware (wraps /api/v1 routes)
    context.go           — request context helpers (user_id extraction)
    auth.go              — register, login, refresh, session, logout handlers
    keys.go              — PUT/GET/DELETE /api/v1/auth/keys handlers
  auth/
    jwt.go               — issue + verify HS256 tokens (15-min access, 7-day refresh)
    password.go          — bcrypt hash (cost 12) + verify
    crypto.go            — AES-256-GCM encrypt/decrypt for provider keys
  db/
    db.go                — pgxpool connection from DATABASE_URL
    migrations.go        — idempotent cambium schema + users + refresh_tokens
    users.go             — user queries: insert, get by email, get by id, update keys
    tokens.go            — refresh token queries: insert, get by hash, revoke
  rhizome/               — HTTP client for Rhizome internal API (Phase 3)
docs/
  design.md              — full architecture and design decisions
```

## Environment variables

```
DATABASE_URL            — postgres connection string (required)
JWT_SECRET              — HS256 signing secret, min 32 bytes (required)
CAMBIUM_ENCRYPTION_KEY  — 32-byte AES-256-GCM master key for provider keys (required)
PORT                    — HTTP listen port (default: 8080)
```

## Database design

One Postgres instance shared with Rhizome, two schemas:

```
postgres instance (port 5432)
  ├── cambium   ← users, refresh_tokens (owned by Cambium)
  └── rhizome   ← all domain tables (owned by Rhizome)
```

This avoids running two separate databases on Spark hardware while maintaining schema ownership boundaries.

### `users` table

```sql
CREATE TABLE cambium.users (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                   TEXT UNIQUE NOT NULL,
    password_hash           TEXT NOT NULL,
    preferred_provider      TEXT,        -- 'gemini' | 'openai' | 'anthropic'
    preferred_model         TEXT,        -- optional override e.g. 'gemini-1.5-pro'
    encrypted_gemini_key    TEXT,        -- AES-256-GCM encrypted, nullable
    encrypted_openai_key    TEXT,        -- nullable
    encrypted_anthropic_key TEXT,        -- nullable
    created_at              TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### `refresh_tokens` table

```sql
CREATE TABLE cambium.refresh_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES cambium.users(id),
    token_hash   TEXT NOT NULL,
    expires_at   TIMESTAMP NOT NULL,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMP
);
```

## What Cambium owns

- `POST /auth/register` — hash password, insert user, return tokens
- `POST /auth/login` — verify password hash, return tokens
- `POST /auth/refresh` — rotate refresh token, return new access token
- `GET  /auth/session` — validate current token
- `PUT  /api/v1/auth/keys` — set or update a provider API key (encrypted at rest)
- `GET  /api/v1/auth/keys` — list which providers are configured (never returns actual keys)
- `DELETE /api/v1/auth/keys/{provider}` — remove a provider key
- JWT verification middleware on all `/api/v1` routes
- `cambium` schema tables: `users`, `refresh_tokens`
- Proxy/translation layer for all other `/api/v1` endpoints to Rhizome
- Decrypting the user's provider key and injecting it into every Rhizome request

## What Cambium does not own

- Domain logic (plants, tasks, triage, projects — all Rhizome)
- The Rhizome database schema or migrations
- Inference routing (Fairlead)
- Frontend code (Verdant)

## Auth design

Custom JWT — no third-party provider. See `design.md` for full rationale.

**Access token:** HS256 signed JWT, 15-minute expiry, carries `user_id` in `sub` claim.
Sent in `Authorization: Bearer <token>` header.

**Refresh token:** Long-lived (7–30 days), stored in `httpOnly` cookie, rotated on
each use to prevent reuse after theft. Stored as a hash in `refresh_tokens`.

**Provider keys:** Encrypted with AES-256-GCM using `CAMBIUM_ENCRYPTION_KEY` (a
server-side secret that never leaves Cambium). The nonce is stored alongside the
ciphertext. Keys are never returned to the client — only a boolean indicating
whether each provider is configured.

**Libraries:**
```
github.com/golang-jwt/jwt/v5
golang.org/x/crypto
```

## How Cambium calls Rhizome

Rhizome exposes a small internal FastAPI service (built during Cambium Phase 3).
It presents two surfaces — Cambium routes to the right one based on request type.

### Agent endpoint — AI operations

For requests that require LangGraph reasoning (triage, interactions, care analysis,
incident analysis, complex queries). Two variants — streaming and non-streaming:

```
POST /internal/agent              — non-streaming (returns complete response)
POST /internal/agent/stream       — SSE streaming (tokens arrive as produced)
POST /internal/agent/resume       — resume paused interaction (non-streaming)
POST /internal/agent/resume/stream — resume with SSE streaming
```

Request body (all four endpoints):

```json
{
  "user_id": "abc-123",
  "thread_id": "thread-xyz",
  "message": "What should I do today?",
  "provider": "gemini",
  "provider_key": "<decrypted key>",
  "model": "gemini-1.5-flash"
}
```

SSE event format:

```
data: {"type": "token",       "content": "The garden "}
data: {"type": "interaction", "payload": {...}}
data: {"type": "done"}
```

Cambium proxies SSE streams directly to Verdant with `io.Copy` — no buffering.

### Data endpoint — direct reads and writes

For requests that don't require AI reasoning (list tasks, get project, update task
status, view activity history), Rhizome exposes direct data endpoints that bypass
the LangGraph agent entirely. These are faster and cheaper — no LLM call, just a
SQLAlchemy query.

```
GET  /internal/data/tasks
GET  /internal/data/projects/{id}
POST /internal/data/tasks/{id}/complete
...
```

Cambium determines which surface to call. The rule is simple:
- Endpoints that involve user messages or AI reasoning → `/internal/agent`
- CRUD reads and simple status mutations → `/internal/data/...`

The `provider_key` is only included in agent requests. It is decrypted by Cambium
immediately before the request and never logged or persisted in decrypted form.

### Rhizome instance routing

Rhizome instances are stateless — all domain state and conversation checkpoints live
in Postgres. Cambium can route any request to any available Rhizome instance; no
sticky sessions are needed. See `rhizome/docs/architecture/deployment.md` for the
full deployment topology and scaling model.

## Planned API surface

Auth (public):
```
POST /auth/register
POST /auth/login
POST /auth/logout
POST /auth/refresh
GET  /auth/session
```

Key management (require valid JWT):
```
PUT    /api/v1/auth/keys              { provider, key }
GET    /api/v1/auth/keys
DELETE /api/v1/auth/keys/{provider}
```

Chat (require valid JWT):
```
POST /api/v1/chat               — non-streaming agent turn (request/response)
POST /api/v1/chat/stream        — SSE streaming agent turn (text/event-stream)
POST /api/v1/chat/resume        — resume paused interaction (non-streaming)
POST /api/v1/chat/resume/stream — resume paused interaction with SSE streaming
```

Protected (require valid JWT):
```
POST /api/v1/triage/run
GET  /api/v1/triage/latest
GET  /api/v1/interactions/pending
GET  /api/v1/interactions/{id}
POST /api/v1/interactions/{id}/resolve
GET  /api/v1/tasks
GET  /api/v1/tasks/due
GET  /api/v1/tasks/daily
GET  /api/v1/tasks/{id}
POST /api/v1/tasks/{id}/start
POST /api/v1/tasks/{id}/complete
POST /api/v1/tasks/{id}/skip
POST /api/v1/tasks/{id}/defer
PUT  /api/v1/tasks/{id}
GET  /api/v1/projects
GET  /api/v1/projects/{id}
GET  /api/v1/projects/{id}/progress
GET  /api/v1/projects/{id}/activity
GET  /api/v1/projects/{projectId}/tasks
GET  /api/v1/projects/{projectId}/proposals
GET  /api/v1/projects/{projectId}/proposals/{proposalId}
POST /api/v1/incidents
GET  /api/v1/incidents
GET  /api/v1/incidents/{id}
GET  /api/v1/treatment-plans/{id}
GET  /api/v1/weather/latest
GET  /api/v1/weather/impacts
POST /api/v1/weather/refresh
GET  /api/v1/activity
GET  /api/v1/tasks/{id}/activity
GET  /api/v1/plants/{id}/activity
GET  /api/v1/incidents/{id}/activity
POST /api/v1/media
GET  /api/v1/media/{id}
```

Alerts + monitor (calendula — phases 1–4):
```
GET    /api/v1/alerts                  — active (pending, non-expired) alerts for this user
POST   /api/v1/alerts/{id}/dismiss     — dismiss an alert
POST   /api/v1/weather/monitor         — manually trigger weather_job
POST   /api/v1/triage/monitor          — manually trigger triage_job
POST   /api/v1/tasks/series/run        — manually trigger series materialization
GET    /api/v1/monitor/runs            — recent MonitorRun records (health/debug)
GET    /api/v1/monitor/runs/{id}       — specific run status and summary
```

## Recommended build order

**Phase 0 — Postgres setup** ✓ done
- Postgres 16 running in Docker (`rhizome-pg`, port 5432, named volume `rhizome_pgdata`)
- `cambium` and `rhizome` schemas created
- Rhizome migrated to Postgres — `DATABASE_URL` env var, `PostgresSaver` checkpointer
- All 310 Rhizome tests passing

**Phase 1 — Project skeleton** ✓ done (main, commit 0f06cc8)
- `go mod init github.com/ybordag/cambium` (Go 1.25, pgx/v5)
- `GET /health` returns `{"status":"ok"}`
- pgxpool Postgres connection, cambium schema migrations at startup

**Phase 2 — Auth endpoints + key management** ✓ done (lenticel, commit b77a850)
- `internal/auth/`: jwt.go, password.go, crypto.go
- `internal/db/`: users.go, tokens.go
- `internal/api/`: middleware.go, context.go, auth.go, keys.go, respond.go
- 21 tests passing

**Phase 3 — Rhizome proxy** (lenticel branch)
- `internal/rhizome/client.go` — HTTP client for Rhizome internal FastAPI
- Middleware pipeline: JWT verify → decrypt provider key → build Rhizome request
- Agent proxy: POST /api/v1/chat → /internal/agent
- Data proxy: GET/POST /api/v1/... → /internal/data/...
- Stub endpoints for all planned routes; full wiring as Rhizome FastAPI layer is built

**Phase 3 — Rhizome proxy**
- HTTP client that calls Rhizome's internal FastAPI
- JWT middleware extracts `user_id`, decrypts provider key, passes both in Rhizome request
- Rhizome model factory updated to accept `provider` + `provider_key` from request context
- Stub `/api/v1/triage/latest` and `/api/v1/tasks` that proxy to Rhizome
- End-to-end test: login → set key → call protected route → Rhizome responds

**Phase 4 — Full API surface**
- Implement remaining endpoints from the planned API surface above
- Media upload handling (stubs for now, full implementation in Epic 2)
- API contract tests

## Invariants — never violate

- **Never trust user input for user_id.** The `user_id` in every Rhizome call must
  come from the verified JWT `sub` claim, not from a request body or query parameter.
- **Never store plaintext passwords.** Use `bcrypt.GenerateFromPassword` with cost ≥ 12.
- **Never store plaintext provider keys.** Always encrypt with AES-256-GCM before writing.
  Decrypt immediately before use; never log or persist in decrypted form.
- **Never return provider keys to the client.** Key management GET endpoint returns
  only which providers are configured, not the actual key values.
- **Refresh tokens rotate on every use.** A used refresh token must be invalidated
  immediately — check `revoked_at IS NULL` before accepting.
- **All `/api/v1` routes require a valid JWT.** The middleware must run before any
  handler logic.
- **Tests required for every auth flow.** At minimum: register, login with wrong
  password, login with correct password, refresh, access protected route with valid
  token, access protected route with expired token, set/get/delete provider key.

## Open questions

1. **Rate limiting on auth endpoints** — needed to prevent brute-force; `golang.org/x/time/rate`
   or a middleware library. Decide before Phase 2 ships.
2. **`CAMBIUM_ENCRYPTION_KEY` rotation strategy** — if the master encryption key ever
   needs rotating, all stored keys must be re-encrypted. Design a rotation path before
   storing real user keys.
3. **gRPC migration trigger** — HTTP is fine for request/response. Switch to gRPC when
   streaming LLM responses are needed (likely when Verdant wants token streaming).
