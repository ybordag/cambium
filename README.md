# Cambium

The HTTP API gateway for the Gardening Agent system. Cambium sits between
Verdant (the React frontend) and Rhizome (the AI domain engine), handling
authentication, encrypted provider key storage, and request routing.

The name comes from botany: the cambium is the actively dividing layer
between the inner wood and outer bark of a plant — exactly what a gateway does.

---

## What it does

- **Auth** — JWT registration, login, token refresh, session validation
- **Provider key management** — users store their own Gemini/OpenAI/Anthropic
  keys; Cambium encrypts them at rest and injects them into every AI request
- **Request routing** — proxies `GET/POST /api/v1/...` to Rhizome's internal
  HTTP interface; routes AI-heavy operations through the LangGraph agent,
  CRUD operations directly to SQLAlchemy
- **SSE streaming** — `POST /api/v1/chat/stream` proxies LLM token streams to
  the browser in real time

## System topology

```
Verdant (React)
    │  HTTPS /api/v1  Authorization: Bearer <token>
    ▼
Cambium  ←  this repo  (Go)
    │  verifies JWT → extracts user_id
    │  decrypts provider key for this user
    │  routes to agent or data surface
    ▼
Rhizome (Python — LangGraph agent + SQLAlchemy)
    │  /internal/agent  — AI operations
    │  /internal/data/  — direct CRUD
    ▼
Postgres
    ├── cambium schema  (users, refresh_tokens)
    └── rhizome schema  (all domain tables)
```

## Quick start

See [Getting Started — Setup](docs/getting-started/setup.md).

## API explorer (Swagger UI)

Once Cambium is running, open **http://localhost:8080/docs/index.html** for an interactive API explorer. All endpoints are documented with request/response schemas and can be tested directly from the browser.

Regenerate the spec after handler changes:
```bash
~/go/bin/swag init -g cmd/server/main.go -o docs
```

## Documentation

- [Setup & running locally](docs/getting-started/setup.md)
- [Using the API](docs/getting-started/using-the-api.md)
- [Architecture overview](docs/architecture/overview.md)
- [Auth flow](docs/architecture/auth-flow.md)
- [Request lifecycle](docs/architecture/request-lifecycle.md)
- [Roadmap](docs/roadmap/overview.md)
- [Full design document](docs/design.md)

## Related repos

| Repo | Role |
|---|---|
| [rhizome](https://github.com/ybordag/rhizome) | AI agent + domain engine (Python, LangGraph) |
| verdant | React frontend |
| fairlead | Inference router (Go/Rust, planned) |

## License

Apache 2.0
