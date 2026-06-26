# API Surface

Cambium exposes the public HTTP API that Verdant consumes. Exact schemas live
in the generated Swagger UI at `/docs/index.html`; this page explains the route
groups, ownership, and runtime path.

---

## Route Ownership

| Public surface | Owner | Runtime path |
|---|---|---|
| `/health` | Cambium | native handler |
| `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout` | Cambium | native auth handlers |
| `/auth/session`, `/auth/profile`, `/auth/password` | Cambium | JWT middleware + native account handlers |
| `/api/v1/auth/keys` | Cambium | JWT middleware + encrypted provider-key handlers |
| `/api/v1/chat...` | Rhizome agent via Cambium | Cambium decrypts provider context, calls `/internal/agent...` |
| Most `/api/v1/...` domain routes | Rhizome data API via Cambium | Cambium injects `user_id`, proxies to `/internal/data/...` |
| `/api/v1/*/draft`, `/triage/run`, task generation triggers | Rhizome agent via Cambium | Cambium builds an agent instruction, calls `/internal/agent` |
| `/api/v1/notifications/stream` | Rhizome data SSE via Cambium | Cambium calls `/internal/data/notifications/stream` and proxies SSE |
| `/api/v1/media` | Stub | currently returns `501 Not Implemented` |
| `/docs/` | Cambium | Swagger UI |
| every other path | Cambium | static Verdant SPA fallback from `STATIC_DIR` |

Cambium never queries Rhizome domain tables directly. It authenticates the user,
extracts trusted `user_id`, and forwards that value to Rhizome.

---

## Public Route Groups

### Auth And Account

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /auth/session`
- `PATCH /auth/profile`
- `POST /auth/password`
- `PUT /api/v1/auth/keys`
- `GET /api/v1/auth/keys`
- `DELETE /api/v1/auth/keys/{provider}`

Cambium owns these routes completely. Provider keys are encrypted at rest and
never returned to the client.

### Chat And Interaction Resume

- `POST /api/v1/chat`
- `POST /api/v1/chat/stream`
- `POST /api/v1/chat/resume`
- `POST /api/v1/chat/resume/stream`

These routes call Rhizome's LangGraph agent surface. Streaming variants proxy
SSE token and interaction events without buffering.

### Garden

- `/api/v1/garden/profile`
- `/api/v1/garden/beds...`
- `/api/v1/garden/containers...`
- `/api/v1/garden/plants...`
- `/api/v1/garden/batches...`
- `/api/v1/garden/search`
- `/api/v1/garden/locations/{location}`

These are structured Rhizome data routes. Batch plant routes must remain before
plant `{id}` wildcard routes in `routes.go`.

### Unified Search

- `GET /api/v1/search`

Searches across supported Rhizome entity types for app context-pin and lookup
workflows. This is distinct from `/api/v1/garden/search`, which is the
garden-specific search surface.

### Projects

- `/api/v1/projects`
- `/api/v1/projects/{id}`
- project progress, brief, proposals, proposal acceptance
- project tasks, bulk task update, task generation trigger
- project series, beds, containers, plants
- project activity, expenses, expense summaries, shopping

Most routes proxy Rhizome data. `POST /api/v1/projects/{id}/tasks/generate`
uses an agent trigger because generation relies on Rhizome's planning/task
logic.

### Tasks

- `/api/v1/tasks`
- daily, due, blocked, materialize
- task series create/update/delete/run
- task detail/update/delete
- lifecycle actions: start, complete, skip, defer
- dependencies and blocker explanation
- task activity

Task status and lifecycle routes are structured Rhizome data routes, not chat
commands.

### Triage And Weather

- `POST /api/v1/triage/run`
- `GET /api/v1/triage/latest`
- `POST /api/v1/triage/monitor`
- `GET /api/v1/weather/latest`
- `POST /api/v1/weather/refresh`
- `GET /api/v1/weather/tasks/impacted`
- `POST /api/v1/weather/tasks/draft`
- `PATCH /api/v1/weather/changesets/{id}/approve`
- `POST /api/v1/weather/monitor`

Drafting weather task changes and running triage can use the agent surface.
Reads, approvals, refreshes, and monitor triggers use structured data routes
where Rhizome owns the behavior.

### Incidents And Treatment Plans

- `/api/v1/incidents`
- incident detail/update/delete/resolve
- treatment draft trigger
- manual treatment plan create
- current incident treatment plan
- incident activity
- treatment plan update/delete/approve

Treatment drafting is an agent trigger. Manual treatment and approval routes
are structured Rhizome data routes.

### Interactions, Notifications, Alerts, Monitor Runs

- `GET /api/v1/interactions/pending`
- `GET /api/v1/interactions/recent`
- `GET /api/v1/interactions/{id}`
- `POST /api/v1/interactions/{id}/resolve`
- `GET /api/v1/notifications/stream`
- `GET /api/v1/notifications`
- `GET /api/v1/alerts`
- `POST /api/v1/alerts/{id}/dismiss`
- `GET /api/v1/monitor/runs`
- `GET /api/v1/monitor/runs/{id}`

Notifications are best-effort live SSE plus a synchronous catch-up route.
Durable alert and interaction state lives in Rhizome.

### Threads

- `POST /api/v1/threads`
- `GET /api/v1/threads`
- `GET /api/v1/threads/{id}`
- `GET /api/v1/threads/{id}/messages`
- `DELETE /api/v1/threads/{id}`
- `GET /api/v1/threads/{id}/session-context`
- `PATCH /api/v1/threads/{id}/session-context`
- `POST /api/v1/threads/{id}/context`
- `DELETE /api/v1/threads/{id}/context/{subjectType}/{subjectId}`

Cambium creates botanical thread IDs and forwards the new thread to Rhizome.
Listing, history, deletion, pinned context, and session context are structured
Rhizome data routes. Verdant should use `GET/PATCH
/api/v1/threads/{id}/session-context` for the normalized startup/session
context contract: `time_text`, `energy_text`, `focus_text`, `focus_context`,
`source`, and `updated_at`. Cambium does not infer or validate focus objects;
it forwards `{ subject_type, subject_id }` refs to Rhizome with the
authenticated user id, and Rhizome validates ownership and resolves display
labels. Returned `focus_context` entries may include `label`, but client PATCH
requests should not send labels. Rhizome thread metadata may also include
`session_context`, but that field is raw stored JSON and not the
`SessionContextView` shape.

When Verdant has a selected garden object, it should create or select the
thread, PATCH `focus_text` plus `focus_context` refs through Cambium, then start
the chat stream. Prose-only mentions such as a batch name do not carry the
stable object id Rhizome needs for reliable focus resolution.

### Calendar, Shopping, Activity

- `/api/v1/calendar/annotations`
- `/api/v1/shopping`
- `/api/v1/activity`
- `/api/v1/activity/stats`

These are structured Rhizome data routes for app surfaces.

### Media

- `POST /api/v1/media`
- `GET /api/v1/media/{id}`

These routes are intentionally present as stubs and return
`501 Not Implemented` until the media/vision work lands.

---

## Route Precedence Notes

Go's `ServeMux` chooses the most specific matching pattern. Still, keep
literal routes before wildcard routes in `routes.go` for readability and to
avoid accidental ambiguity. Examples:

- `/api/v1/garden/plants/batch/remove` before `/api/v1/garden/plants/{id}`
- `/api/v1/tasks/daily` before `/api/v1/tasks/{id}`
- `/api/v1/tasks/series/{id}` before generic task ID routes

The static frontend handler is registered last as `"/"` and should stay there.

---

## When To Use This Page

Use this page to understand the API shape and runtime owner. Use:

- Swagger UI for exact request/response schema.
- [Adding Routes](../development/adding-routes.md) for implementation workflow.
- [Request Lifecycle](request-lifecycle.md) for step-by-step runtime behavior.
