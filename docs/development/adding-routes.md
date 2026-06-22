# Adding Routes

Cambium routes fall into a few repeatable patterns. Choose the pattern first,
then add the route, handler/proxy wiring, Swagger annotations, and tests.

---

## Choose the Ownership Pattern

| Route kind | Use when | Cambium code path | Rhizome path |
|---|---|---|---|
| Native Cambium handler | Auth, account profile, password, provider keys, static files, health | `internal/api/{auth,keys,static,health}.go` | none |
| Rhizome data proxy | Structured CRUD, reads, status changes, alerts, search, activity, calendar, shopping | `proxyData` / `proxyDataWithPathParam` → `dispatchData` | `/internal/data/...` |
| Rhizome agent trigger | Cambium needs the agent to reason from a prebuilt instruction | `internal/api/triggers.go` → `RunAgent` | `/internal/agent` |
| Chat proxy | User-authored message or interaction resume | `proxy.go` chat/resume handlers | `/internal/agent...` |
| SSE data stream | Live data events such as notifications | `StreamData` + `proxySSE` | `/internal/data/.../stream` |

The rule of thumb: Cambium owns identity and public HTTP. Rhizome owns garden
domain behavior. Do not duplicate Rhizome domain rules in Cambium.

---

## Native Cambium Handler

Use this when the route changes Cambium-owned state or behavior.

Examples:

- `POST /auth/register`
- `PATCH /auth/profile`
- `POST /auth/password`
- `PUT /api/v1/auth/keys`
- `GET /health`

Steps:

1. Add or update the handler in the relevant `internal/api/*.go` file.
2. Keep DB work in `internal/db` helpers when it touches `cambium` tables.
3. Use `writeJSON` and `writeError` for responses.
4. Register the route in `internal/api/routes.go`.
5. Wrap protected routes with `RequireAuth`.
6. Add Swagger annotations and request/response structs if needed.
7. Add or update handler tests.

Native handlers should never query the `rhizome` schema.

---

## Rhizome Data Proxy Route

Use this for structured data routes that do not need LLM reasoning.

Examples:

- `GET /api/v1/tasks/daily`
- `PATCH /api/v1/garden/profile`
- `DELETE /api/v1/threads/{id}/context/{subjectType}/{subjectId}`
- `PATCH /api/v1/threads/{id}/session-context`
- `GET /api/v1/search`
- `POST /api/v1/alerts/{id}/dismiss`

Steps:

1. Confirm the Rhizome internal route exists under `/internal/data/...`.
2. Register the Cambium route in `internal/api/routes.go`.
3. Use `ph.proxyData("literal/path")` for routes without path params.
4. Use `ph.proxyDataWithPathParam("base/path", "id")` when the remaining URL
   path should be forwarded after a known prefix.
5. Keep `RequireAuth` on the public route.
6. Add Swagger annotations/types when the route is part of the documented API.
7. Add a proxy test if method/path/query/body forwarding is not already covered
   by an existing equivalent test.
8. Add the route to `TestAllProtectedRoutesReject401`.

Important: `dispatchData` preserves the original HTTP method. Do not route a
structured `PATCH` or `DELETE` through an agent trigger just to avoid proxying.

---

## Rhizome Agent Trigger

Use this when Cambium exposes a button-like public API but the actual work
requires the LangGraph agent to reason.

Examples:

- `POST /api/v1/triage/run`
- `POST /api/v1/weather/tasks/draft`
- `POST /api/v1/incidents/{id}/treatment`
- `POST /api/v1/projects/{id}/tasks/generate`

Steps:

1. Add or update a trigger handler in `internal/api/triggers.go`.
2. Require a `thread_id` when the action belongs in a conversation.
3. Build a clear natural-language instruction for Rhizome.
4. Call `providerKey` so user provider/model preferences are forwarded.
5. Call `rhizome.RunAgent`.
6. Return the agent response with `writeJSON`.
7. Add tests around validation and request construction where practical.
8. Add the route to `TestAllProtectedRoutesReject401`.

Agent triggers should be narrow. If the frontend just needs a list, status
change, or structured mutation, use the data proxy instead.

---

## Chat And Resume Routes

Chat routes are special because they forward user-authored text and may stream
tokens.

Current routes:

- `POST /api/v1/chat`
- `POST /api/v1/chat/stream`
- `POST /api/v1/chat/resume`
- `POST /api/v1/chat/resume/stream`

Rules:

- Require `thread_id` for new chat messages.
- For resume, require both `thread_id` and `resolution`.
- Forward provider, provider key, and model only after decrypting user-owned
  provider state.
- Streaming routes must proxy `text/event-stream` without buffering.
- Interaction events end the current stream; Verdant resumes through the resume
  route.

---

## SSE Data Routes

Use `StreamData` for long-lived Rhizome data streams.

Current example:

- `GET /api/v1/notifications/stream`

Rules:

- Keep the route authenticated.
- Preserve query params such as `since`.
- Set and preserve `Content-Type: text/event-stream`.
- Treat live delivery as best-effort unless the backing Rhizome feature has a
  durable recovery path.
- Provide a synchronous catch-up route when possible, such as
  `GET /api/v1/notifications`.

---

## Swagger Workflow

Swagger docs are generated from handler annotations and structs in
`internal/api/swagger_types.go`.

After changing public API shape:

```bash
make swagger
```

Commit the regenerated files with the route change.

Do not hand-edit generated Swagger JSON/YAML except as a temporary debugging
step.

---

## Test Checklist

For every new protected route:

- route is wrapped in `RequireAuth`
- route is listed in `TestAllProtectedRoutesReject401`
- malformed bodies return `400`
- Cambium-native errors return structured JSON via `writeError`
- Rhizome transport failures return `502`
- method, path, query params, body, and `user_id` are forwarded correctly
- SSE routes preserve event-stream headers and body
- provider keys are forwarded only for agent/chat routes

See [Testing Guide](testing.md) for examples.

---

## Documentation Checklist

Update docs when the route changes public behavior:

- Swagger generated files for exact OpenAPI contract.
- [Using the API](../getting-started/using-the-api.md) if users need an example.
- [Architecture Overview](../architecture/overview.md) if a new route category
  or ownership boundary appears.
- [Request Lifecycle](../architecture/request-lifecycle.md) if the route uses a
  new runtime pattern.
- [Roadmap](../roadmap/overview.md) if the route closes or changes a tracked
  initiative.
