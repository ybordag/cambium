# Code Organization

This guide maps the Cambium codebase for contributors who need to change
routes, auth, proxying, static serving, or tests.

---

## Top-Level Layout

```
cambium/
├── cmd/server/       Process entrypoint and Swagger metadata
├── internal/api/     HTTP router, handlers, middleware, proxy layer
├── internal/auth/    JWT, password hashing, provider-key encryption
├── internal/db/      Postgres connection, migrations, users, refresh tokens
├── internal/rhizome/ HTTP client for Rhizome internal routes
├── k8s/              Deployment manifests
├── docs/             Markdown docs and generated Swagger artifacts
├── go.mod            Go module definition
└── README.md
```

Cambium owns the public HTTP boundary for Verdant. Rhizome owns garden domain
state and agent behavior.

---

## `cmd/server/`

`cmd/server/main.go` is the process entrypoint. It:

- connects to Postgres through `internal/db.Connect`
- runs Cambium schema migrations through `internal/db.Migrate`
- reads `PORT`
- starts `api.NewRouter(pool)`
- carries the top-level Swagger metadata used by `swag init`

---

## `internal/api/`

HTTP concerns live here.

| File | Responsibility |
|---|---|
| `routes.go` | Registers all public routes. More specific routes must stay before catch-all static serving. |
| `middleware.go` | `RequireAuth`, JWT validation, request context population. |
| `context.go` | Typed `user_id` context helpers. |
| `auth.go` | Register, login, refresh, logout, session, profile, password handlers. |
| `keys.go` | Provider key set/list/delete handlers. |
| `proxy.go` | Chat proxying, data proxying, notification streaming, provider-key injection. |
| `triggers.go` | AI-trigger endpoints that synthesize agent instructions. |
| `threads.go` | Cambium-native thread creation and initial-context forwarding. |
| `threadnames.go` | Botanical thread-name generator. |
| `static.go` | Serves built Verdant assets from `STATIC_DIR` with SPA fallback. |
| `swagger_types.go` | Request/response structs for generated Swagger docs. |
| `respond.go` | JSON response and error helpers. |
| `health.go` | `GET /health`. |

API tests live next to the handlers as `*_test.go`.

---

## `internal/auth/`

Pure crypto/auth helpers. These files should not import HTTP or DB packages.

| File | Responsibility |
|---|---|
| `jwt.go` | Issue and verify HS256 access tokens. |
| `password.go` | bcrypt password hashing and verification. |
| `crypto.go` | AES-256-GCM provider-key encryption/decryption. |

---

## `internal/db/`

Postgres access for Cambium-owned tables only.

| File | Responsibility |
|---|---|
| `db.go` | Opens a `pgxpool` from `DATABASE_URL`. |
| `migrations.go` | Idempotent `cambium` schema/table creation. |
| `users.go` | User, profile, and provider-key queries. |
| `tokens.go` | Refresh-token generation, hashing, storage, lookup, and revocation. |

Cambium never queries the `rhizome` schema. It passes trusted `user_id` values
to Rhizome instead.

---

## `internal/rhizome/`

Small HTTP client for Rhizome's internal FastAPI server.

- `RunAgent`, `StreamAgent`, `ResumeAgent`, and `StreamResume` call the agent
  surface.
- `DataRequest` preserves HTTP method and proxies structured data routes.
- `DataGet`, `DataPost`, and `DataDelete` are convenience wrappers.
- `StreamData` proxies long-lived data-surface SSE routes such as
  notifications.

Provider keys are included only in agent requests that may call an LLM.

---

## Change Rules

- Add or change public routes in `internal/api/routes.go`.
- If a route proxies Rhizome structured data, use the method-preserving data
  proxy path.
- If a route performs Cambium-owned auth/account/key behavior, keep it native in
  `internal/api` and `internal/db`.
- If a route requires LLM reasoning, use the agent surface and include provider
  context.
- Regenerate Swagger after handler or request/response changes:

```bash
make swagger
```

- Add or update tests next to the changed package.

For a more complete route workflow, see [Adding Routes](adding-routes.md).
