# Cambium — Design Document

**Status:** Initial design — updated with provider key storage and Postgres decisions  
**Version:** 0.2

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
well-defined internal HTTP interface (gRPC when streaming is needed). They share
one Postgres instance under separate schemas.

---

## Language

Go. Reasons:

- Natural fit for HTTP gateway work: goroutines, fast startup, low memory
- Strong standard library for JWT, bcrypt, AES crypto — minimal external dependencies
- Single static binary — fits the Spark hardware deployment model
- Good portfolio signal for infrastructure/backend roles
- Fairlead (inference router) may also be Go

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

Rhizome's data endpoint is built during **Phase 3** alongside the agent endpoint.

### Rhizome instance topology

Rhizome instances are stateless — domain data and LangGraph conversation checkpoints
both live in Postgres. Cambium can route any request to any available Rhizome
instance. No sticky sessions. See `rhizome/docs/architecture/deployment.md` for
the full topology, scaling model, and future Temporal evolution.

### Current: HTTP → Future: gRPC

HTTP is correct for request/response flows. Migrate to gRPC when Verdant needs
token streaming (streaming LLM responses). gRPC supports bidirectional streaming
natively and is a drop-in for the internal interface without changing Cambium's
public API surface.

---

## Build order

### Phase 0 — Postgres setup (in progress)

- ~~Migrate Rhizome: update `db/database.py` to read `DATABASE_URL`, swap `SqliteSaver` → `langgraph-checkpoint-postgres`~~ **done** (geranium commit 6a3c672)
- Stand up Postgres: `docker run --name rhizome-pg -e POSTGRES_PASSWORD=dev -p 5432:5432 -d postgres`
- Create schemas: `CREATE SCHEMA cambium; CREATE SCHEMA rhizome;`
- Set `DATABASE_URL` in Rhizome `.env`, run `init_db()` to create `rhizome` schema tables
- Verify Rhizome tests still pass (test suite uses in-memory SQLite, unaffected)

### Phase 1 — Go project skeleton

- `go mod init github.com/ybordag/cambium`
- Directory structure:
  ```
  cmd/server/       — main.go, entry point
  internal/auth/    — JWT, bcrypt, AES key encryption
  internal/api/     — HTTP handlers, route registration
  internal/rhizome/ — HTTP client that calls Rhizome internal API
  internal/db/      — Postgres connection, user/token queries
  ```
- Basic `net/http` server returning 200 on `/health`
- Postgres connection (pgx or database/sql + lib/pq)
- `cambium` schema migrations (users + refresh_tokens)

### Phase 2 — Auth + key management

- Register, login, refresh, session endpoints
- JWT middleware (handler wrapper, not inline)
- Key management: PUT/GET/DELETE `/api/v1/auth/keys` with AES-256-GCM
- Tests: full register → login → refresh → protected route flow
- Tests: set key, verify configured, delete key, verify unconfigured

### Phase 3 — Rhizome proxy

- HTTP client for Rhizome internal FastAPI
- Middleware pipeline: JWT verify → decrypt provider key → build Rhizome request
- Stub proxy endpoints (`/api/v1/triage/latest`, `/api/v1/tasks/daily`)
- End-to-end: login → set key → call protected route → Rhizome uses user's key

### Phase 4 — Full API surface

- All endpoints from the planned surface in CLAUDE.md
- Media upload stubs (full implementation waits for Epic 2)
- API contract tests

---

## Open questions

1. **Rate limiting** — auth endpoints need brute-force protection. `golang.org/x/time/rate` or a middleware library. Decide before Phase 2 ships.

2. **`CAMBIUM_ENCRYPTION_KEY` rotation** — design the re-encryption script before going to production. See Key rotation section above.

3. **gRPC migration trigger** — switch when Verdant needs token streaming. No action needed now.

---

## Resolved design decisions

| Question | Decision | Rationale |
|---|---|---|
| `users` table location | Same Postgres instance, `cambium` schema | Simpler to operate on Spark hardware; single backup; no cross-service joins needed |
| Refresh token storage | Separate `refresh_tokens` table | Supports multiple devices/sessions; easy to list and revoke all sessions |
| Access token delivery | `Authorization: Bearer` header; frontend stores in `localStorage` | Standard REST pattern; simpler CORS setup; acceptable XSS trade-off for a portfolio project |
| Provider key storage | Encrypted in `users` table (AES-256-GCM) | Keys at rest are never plaintext; keys are never returned to the client |
| Postgres shared vs. separate | Shared instance, separate schemas | One Docker container on Spark; single backup target; Cambium and Rhizome stay independently deployable |
