# Cambium — Claude Code Memory

## What this project is

Cambium is the HTTP API gateway for the Gardening Agent system, written in Go.
It sits between the Verdant frontend and the Rhizome domain engine, handling:

- User authentication (JWT, bcrypt password hashing)
- Request routing to Rhizome over an internal HTTP interface
- Stable versioned JSON DTOs for the frontend
- Media/file upload handling (future)

See `design.md` for the full architecture and open design questions.

## Related repos

- **Rhizome** (Python) — the agent and domain engine. Cambium calls it over HTTP.
- **Verdant** — React frontend. Calls Cambium over `/api/v1`.
- **Fairlead** — inference router (Go or Rust, TBD). Not yet built.

## Tech stack

- **Language:** Go (1.21+)
- **Routing:** standard library `net/http` or `gin`
- **JWT:** `github.com/golang-jwt/jwt/v5`
- **Password hashing:** `golang.org/x/crypto/bcrypt`
- **Database:** Postgres — `users` table owned by Cambium; Rhizome tables in
  the same instance
- **Internal Rhizome interface:** HTTP initially, gRPC when streaming is needed

## Build and test

```
go build ./...
go test ./...
```

No code yet — this is the initial setup phase.

## What Cambium owns

- `POST /auth/register` — hash password, insert user, return tokens
- `POST /auth/login` — verify password hash, return tokens
- `POST /auth/refresh` — rotate refresh token, return new access token
- `GET  /auth/session` — validate current token
- JWT verification middleware on all `/api/v1` routes
- `users` table: `id UUID, email TEXT UNIQUE, password_hash TEXT, created_at TIMESTAMP`
- Proxy/translation layer for all `/api/v1` endpoints to Rhizome

## What Cambium does not own

- Domain logic (plants, tasks, triage, projects — all Rhizome)
- The Rhizome database schema or migrations
- Inference routing (Fairlead)
- Frontend code (Verdant)

## Auth design

Custom JWT — no third-party provider. See `design.md` for the full rationale.

**Access token:** HS256 signed JWT, 15-minute expiry, carries `user_id` in
`sub` claim. Sent in `Authorization: Bearer <token>` header.

**Refresh token:** Long-lived (7–30 days), stored in `httpOnly` cookie, rotated
on each use to prevent reuse after theft.

**Libraries:**
```
github.com/golang-jwt/jwt/v5
golang.org/x/crypto
```

## How Cambium calls Rhizome

Rhizome exposes a small internal FastAPI service (not yet built). Cambium calls
it over HTTP, passing `user_id` in the request so Rhizome can scope all DB
queries correctly.

When Rhizome adds the FastAPI layer, the internal contract will be:
```
POST /internal/chat
  Body: { user_id: int, thread_id: str, message: str }
  Returns: { response: str, pending_interaction: {...} | null }
```

## Planned API surface

Auth (public):
```
POST /auth/register
POST /auth/login
POST /auth/logout
POST /auth/refresh
GET  /auth/session
```

Protected (require valid JWT):
```
POST /api/v1/triage/run
GET  /api/v1/triage/latest
GET  /api/v1/interactions/pending
GET  /api/v1/interactions/{id}
POST /api/v1/interactions/{id}/resolve
GET  /api/v1/tasks
GET  /api/v1/tasks/{id}
POST /api/v1/tasks/{id}/complete
POST /api/v1/tasks/{id}/defer
POST /api/v1/incidents
GET  /api/v1/incidents/{id}
GET  /api/v1/treatment-plans/{id}
GET  /api/v1/weather/latest
POST /api/v1/media
GET  /api/v1/media/{id}
```

## Recommended build order

**Phase 1 — Project skeleton**
- `go mod init github.com/ybordag/cambium`
- Directory structure: `cmd/server/`, `internal/auth/`, `internal/api/`,
  `internal/rhizome/`, `internal/db/`
- Basic `net/http` server that returns 200 on `/health`
- Postgres connection via `pgx` or `database/sql` + `lib/pq`
- `users` table migration (use `golang-migrate` or write raw SQL)

**Phase 2 — Auth endpoints**
- `POST /auth/register`: bcrypt hash, insert user, issue tokens
- `POST /auth/login`: bcrypt verify, issue tokens
- `POST /auth/refresh`: rotate refresh token
- JWT middleware as a handler wrapper
- Tests for the full register → login → refresh → protected route flow

**Phase 3 — Rhizome proxy**
- HTTP client that calls Rhizome's internal API
- JWT middleware extracts `user_id`, passes it in Rhizome request
- Stub `/api/v1/triage/latest` and `/api/v1/tasks` that proxy to Rhizome
- End-to-end test: login → call protected route → Rhizome responds

**Phase 4 — Full API surface**
- Implement remaining endpoints from the planned API surface above
- Media upload handling
- API contract tests

## Invariants — never violate

- **Never trust user input for user_id.** The `user_id` in every Rhizome call
  must come from the verified JWT `sub` claim, not from a request body or query
  parameter.
- **Never store plaintext passwords.** Use `bcrypt.GenerateFromPassword` with a
  cost factor of at least 12.
- **Refresh tokens rotate on every use.** A used refresh token must be
  invalidated immediately — store them in the `users` table or a separate
  `refresh_tokens` table and check for reuse.
- **All `/api/v1` routes require a valid JWT.** The middleware must run before
  any handler logic.
- **Tests required for every auth flow.** At minimum: register, login with wrong
  password, login with correct password, refresh, access protected route with
  valid token, access protected route with expired token.

## Open questions (resolve before starting Phase 2)

1. **`users` table location:** same Postgres instance as Rhizome, or separate
   database? (Same instance is simpler; separate enforces ownership boundaries)
2. **Refresh token storage:** store in `users.refresh_token` (simplest) or a
   separate `refresh_tokens` table (supports multiple devices)?
3. **Access token storage on frontend:** `localStorage` (simpler, XSS risk) or
   `httpOnly` cookie (more secure, adds CSRF complexity)?
4. **Rate limiting on auth endpoints:** needed to prevent brute-force; `golang.org/x/time/rate` or a middleware library?
