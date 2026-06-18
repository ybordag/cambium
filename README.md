# cambium

The HTTP API gateway for the Gardening Agent system. Cambium sits between the
frontend (Verdant) and the domain engine (Rhizome), handling authentication,
request routing, and the public HTTP contract.

**Status:** Design phase — no runnable code yet.

---

## What it does

Cambium is the boundary between the outside world and the Rhizome agent. Every
request from Verdant passes through Cambium, which:

- Verifies the caller's identity via JWT
- Extracts the user ID and threads it into every Rhizome call
- Exposes a stable versioned HTTP/JSON API under `/api/v1`
- Owns user registration, login, and token refresh
- Handles media/file uploads for image analysis workflows

---

## System topology

```
Verdant (React)
    ↓  HTTP/JSON  /api/v1
Cambium  ←  this repo
    ↓  JWT verified, user_id extracted
    ↓  internal HTTP or gRPC
Rhizome (Python — agent and domain engine)
    ↓  all queries scoped to user_id
Postgres
Fairlead (inference router)
```

Cambium and Rhizome run as **separate processes**. Cambium calls Rhizome over a
well-defined internal interface — it does not import Rhizome as a library.

---

## Tech stack

- **Language:** Go
- **Auth:** Custom JWT — `golang-jwt/jwt`, `golang.org/x/crypto/bcrypt`
- **Routing:** `net/http` (standard library) or `gin`
- **Database:** Postgres — owns the `users` table; Rhizome domain tables live
  in the same instance but are owned by Rhizome
- **Internal interface to Rhizome:** HTTP initially, gRPC when streaming is needed

---

## Auth approach

Cambium implements its own JWT auth stack — no third-party auth provider. This
keeps all user data and auth traffic on owned hardware (Spark cluster) and
gives full control over the token lifecycle.

Flow:

1. `POST /auth/register` — bcrypt hash, insert user, return tokens
2. `POST /auth/login` — verify bcrypt hash, return tokens
3. `POST /auth/refresh` — rotate refresh token, return new access token
4. Access token (15 min) carries `user_id` in the `sub` claim
5. Refresh token (7–30 days) travels in an `httpOnly` cookie
6. Every protected route runs a JWT middleware that extracts `user_id` and
   passes it to Rhizome

---

## Planned API surface

Auth:

```
POST  /auth/register
POST  /auth/login
POST  /auth/logout
POST  /auth/refresh
GET   /auth/session
```

Core operations (proxied to Rhizome):

```
POST  /api/v1/triage/run
GET   /api/v1/triage/latest
GET   /api/v1/interactions/pending
GET   /api/v1/interactions/{id}
POST  /api/v1/interactions/{id}/resolve
GET   /api/v1/tasks
GET   /api/v1/tasks/{id}
POST  /api/v1/tasks/{id}/complete
POST  /api/v1/tasks/{id}/defer
POST  /api/v1/incidents
GET   /api/v1/incidents/{id}
GET   /api/v1/treatment-plans/{id}
GET   /api/v1/weather/latest
POST  /api/v1/media
GET   /api/v1/media/{id}
```

See [`design.md`](design.md) for the full architecture and open design questions.

---

## Related repos

| Repo | Role |
|---|---|
| [rhizome](https://github.com/ybordag/rhizome) | Agent and domain engine (Python) |
| verdant | Frontend (React) |
| fairlead | Inference router |

---

## License

Apache 2.0
