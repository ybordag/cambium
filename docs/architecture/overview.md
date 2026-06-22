# Architecture Overview

How Cambium fits in the system and what each component does.

---

## System topology

```
                    ┌──────────────────────────┐
                    │  Verdant (React)          │
                    │  Browser / mobile         │
                    └────────────┬─────────────┘
                                 │ HTTPS /api/v1
                                 │ Authorization: Bearer <token>
                    ┌────────────▼─────────────┐
                    │  Cambium  ←  this repo   │
                    │  Go HTTP gateway          │
                    │  auth · keys · routing    │
                    └───────┬──────────┬────────┘
               /internal/agent    /internal/data/...
                    ┌──────▼──────────▼────────┐
                    │  Rhizome (Python)         │
                    │  LangGraph agent          │
                    │  SQLAlchemy + FastAPI     │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │  Postgres                 │
                    │  cambium schema (auth)    │
                    │  rhizome schema (domain)  │
                    └───────────────────────────┘
```

Cambium and Rhizome are separate processes. They share one Postgres instance under separate schemas. Cambium never queries the `rhizome` schema; Rhizome never queries `cambium`.

---

## Components

### `cmd/server/main.go`

Entry point. Connects to Postgres, runs schema migrations, starts the HTTP server. Wires together the DB pool and the router.

### `internal/api/`

HTTP handlers organized by concern:

| File | Responsibility |
|---|---|
| `routes.go` | Route registration — maps every URL to a handler |
| `middleware.go` | `RequireAuth` — verifies JWT, injects `user_id` into context |
| `context.go` | `UserIDFromContext` — typed context key for `user_id` |
| `auth.go` | Register, login, refresh, session, logout handlers |
| `keys.go` | Provider key set/list/delete handlers |
| `proxy.go` | Generic data proxy — `proxyData`, `proxyDataWithPathParam`, `proxySSE` |
| `triggers.go` | AI-trigger handlers — triage, weather draft, treatment draft, task generation |
| `threads.go` | Cambium-native thread creation and botanical name generation |
| `threadnames.go` | Botanical thread name vocabulary/generator |
| `static.go` | Serves built Verdant assets from `STATIC_DIR` with SPA fallback |
| `health.go` | `GET /health` |
| `respond.go` | `writeJSON`, `writeError` helpers |

### `internal/auth/`

Pure crypto functions — no HTTP, no DB:

| File | Responsibility |
|---|---|
| `jwt.go` | `IssueAccessToken`, `VerifyAccessToken` (HS256, 15-min expiry) |
| `password.go` | `HashPassword`, `CheckPassword` (bcrypt cost 12) |
| `crypto.go` | `EncryptKey`, `DecryptKey` (AES-256-GCM, random nonce, base64) |

### `internal/db/`

Postgres queries — no HTTP, no crypto:

| File | Responsibility |
|---|---|
| `db.go` | `Connect` — pgxpool from `DATABASE_URL` |
| `migrations.go` | `Migrate` — idempotent `CREATE TABLE IF NOT EXISTS` for cambium schema |
| `users.go` | `InsertUser`, `GetUserByEmail`, `GetUserByID`, `SetProviderKey`, `ClearProviderKey` |
| `tokens.go` | `InsertRefreshToken`, `GetRefreshToken`, `RevokeRefreshToken`, `HashToken` |

### `internal/rhizome/`

HTTP client for Rhizome's internal API:

| Method | Calls |
|---|---|
| `RunAgent` | `POST /internal/agent` — waits for complete response |
| `StreamAgent` | `POST /internal/agent/stream` — returns raw SSE body |
| `ResumeAgent` | `POST /internal/agent/resume` |
| `StreamResume` | `POST /internal/agent/resume/stream` |
| `DataGet` | `GET /internal/data/{path}?user_id=...` |
| `DataPost` | `POST /internal/data/{path}?user_id=...` |
| `DataRequest` | Method-preserving proxy for `GET`, `POST`, `PATCH`, and `DELETE` data routes |
| `StreamData` | `GET /internal/data/{path}` SSE stream, used by notifications |

---

## Two Rhizome surfaces

Every Cambium request that reaches Rhizome goes to one of two surfaces:

```
Cambium
  ├── AI operation?  → POST /internal/agent   (LangGraph graph, LLM call)
  └── CRUD?          → GET/POST /internal/data/...  (SQLAlchemy, no LLM)
```

**Agent surface** — used for operations that require LLM reasoning:
- Chat messages
- Daily triage
- Weather impact drafting
- Treatment plan drafting
- Task generation

**Data surface** — used for all reads and status mutations:
- Plant, bed, container CRUD
- Task lifecycle (start, complete, skip, defer)
- Project and brief management
- Activity history
- Alert reads and dismissals
- Notifications and monitor run snapshots
- Unified search and thread pinned context
- Calendar annotations and shopping lists

This split keeps LLM costs low — a `GET /api/v1/tasks/daily` costs zero tokens.

## Public API Surface

Cambium exposes the public `/api/v1` contract consumed by Verdant. The current
surface includes:

- chat and chat resume, streaming and non-streaming
- provider key management
- garden profile, beds, containers, plants, batches, search, and locations
- projects, briefs, proposals, project tasks, project resources, expenses, and
  shopping
- task CRUD, lifecycle actions, dependencies, series, daily/due/blocked lists,
  and materialization triggers
- triage, weather, incidents, treatment plans, interactions, alerts, monitor
  runs, notifications, activity, calendar annotations, and unified search
- thread creation, history, delete, and pinned context management
- media stubs, currently returning `501 Not Implemented`

The generated Swagger UI at `/docs/index.html` is the exhaustive endpoint list.

---

## Database ownership

```
Postgres (port 5432)
  ├── cambium schema   — owned by Cambium
  │     users          (id, email, password_hash, provider keys, preferences)
  │     refresh_tokens (id, user_id, token_hash, expires_at, revoked_at)
  │
  └── rhizome schema   — owned by Rhizome
        garden_profile, plant, bed, container, task, project, ...
```

Cambium and Rhizome share one Postgres instance on local dev and on the Spark cluster. They operate on independent schemas and never cross-query each other. `user_id` (a UUID from `cambium.users`) is passed in every Rhizome internal request — Rhizome uses it to scope all domain queries but never joins against the `cambium` schema.

---

## Provider key flow

```
User submits key:
  PUT /api/v1/auth/keys {"provider":"gemini","key":"AIza..."}
    → auth.EncryptKey(key)              ← AES-256-GCM with CAMBIUM_ENCRYPTION_KEY
    → db.SetProviderKey(encryptedKey)   ← stored in users.encrypted_gemini_key

On every AI request:
  proxyHandler.providerKey()
    → db.GetUserByID()                  ← read encrypted key from users table
    → auth.DecryptKey(encryptedKey)     ← plaintext only in memory, never logged
    → AgentRequest{ProviderKey: key}    ← sent to Rhizome internal API
    → key goes out of scope             ← GC'd, never persisted in decrypted form
```

Keys are never returned to the client. `GET /api/v1/auth/keys` returns only booleans.

---

## Static Frontend Serving

Cambium can serve the built Verdant Pages app directly. `STATIC_DIR` points at
the frontend `dist/` directory and defaults to `./dist`. Any path not claimed by
`/api/v1/*`, `/auth/*`, `/health`, or `/docs/*` falls back to `index.html`, so
client-side routing works after a browser refresh.
