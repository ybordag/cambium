# Cambium — Design Document

**Status:** Initial design
**Version:** 0.1

---

## What it is

Cambium is the HTTP API gateway for the Gardening Agent system. It sits between
the frontend (Verdant) and the domain engine (Rhizome), handling authentication,
request routing, and the public HTTP contract.

The name comes from botany: the cambium is the actively dividing layer between
the inner wood and the outer bark of a plant. It mediates exchange between the
two — exactly what an API gateway does.

---

## Role in the system

```
Verdant (React frontend)
    ↓  HTTP/JSON  /api/v1
Cambium (this repo — Go)
    ↓  verifies JWT → extracts user_id
    ↓  calls Rhizome over gRPC or HTTP
Rhizome (Python — domain engine)
    ↓  threads user_id through graph and tools
    ↓  all DB queries scoped to user_id
Postgres
    ↦  users table  (owned by Cambium)
    ↦  Rhizome domain tables  (scoped by user_id)
```

Cambium and Rhizome are **separate processes**. Cambium does not import Rhizome
as a library — it calls Rhizome over a well-defined internal interface (gRPC or
HTTP). This keeps them independently deployable and language-agnostic.

---

## Language

Go. Reasons:

- Natural fit for HTTP gateway work: goroutines, standard library net/http,
  fast and lightweight
- Strong ecosystem for JWT auth (`golang-jwt`), password hashing (`bcrypt` via
  `golang.org/x/crypto`), and gRPC clients
- Single static binary, fast startup — fits the Spark hardware deployment model
- Good portfolio signal for infrastructure/backend roles
- Fairlead (the inference router) may also be Go, giving consistent language
  across the infrastructure layer

---

## What Cambium owns

- User registration and login (`POST /auth/register`, `POST /auth/login`)
- JWT issuance (short-lived access tokens + long-lived refresh tokens)
- Token refresh (`POST /auth/refresh`)
- JWT verification middleware (runs on every protected route)
- `users` table in Postgres (`id UUID`, `email`, `password_hash`, `created_at`)
- Translation of incoming HTTP requests into Rhizome agent calls
- Stable JSON DTOs for all app-facing resources (see Rhizome Epic 9 for the
  full endpoint and payload inventory)
- Media/file upload handling and asset metadata (for Epic 2)

## What Cambium does not own

- Domain logic of any kind (gardens, plants, tasks, triage — all Rhizome)
- The Rhizome database schema or migrations
- Inference capacity or model routing (Fairlead)
- Frontend presentation logic (Verdant)

---

## Auth design

Custom JWT auth — no third-party auth provider. Reasons:

- Full control over the auth stack and user data
- Everything runs on owned hardware (Spark), no external auth traffic
- Educational value: implements the full backend auth lifecycle

### Flow

1. `POST /auth/register` — hash password with bcrypt, insert user, issue tokens
2. `POST /auth/login` — verify bcrypt hash, issue tokens
3. Access token: short-lived JWT (15 min), carries `user_id` in `sub` claim
4. Refresh token: long-lived (7–30 days), stored in `httpOnly` cookie,
   rotated on each use
5. Every protected route runs a JWT middleware that verifies the signature and
   extracts `user_id`
6. `user_id` is passed to Rhizome in the agent config for every call

### Algorithm

HS256 (shared secret) for the initial implementation. Migrate to RS256
(asymmetric) if Fairlead or other services need to independently verify tokens.

### Libraries

- `github.com/golang-jwt/jwt/v5` — JWT signing and verification
- `golang.org/x/crypto/bcrypt` — password hashing
- Standard library `net/http` or `gin` for routing

---

## Rhizome interface

Cambium calls Rhizome over an internal interface. Two candidate approaches:

- **HTTP/JSON** — simpler to implement initially; Rhizome adds a minimal FastAPI
  layer and Cambium calls it with a standard HTTP client
- **gRPC** — lower overhead, typed contracts via protobuf, better for streaming
  (relevant for streaming LLM responses); more setup

Start with HTTP, migrate to gRPC when streaming responses are needed.

---

## Open questions

- Should the `users` table live in the same Postgres instance as the Rhizome
  tables, or a separate database? (Same instance is simpler; separate is cleaner
  separation of ownership)
- HTTP or gRPC for the Cambium → Rhizome internal interface?
- Should access tokens be stored in `localStorage` or `httpOnly` cookies on the
  frontend? (localStorage is simpler but XSS-vulnerable; httpOnly cookie is more
  secure but adds CSRF complexity)
- Rate limiting strategy for auth endpoints (to prevent brute-force attacks)
