# Using the API

A practical walkthrough of Cambium's API from registration through AI chat.

All examples use `curl`. Replace `localhost:8080` with your deployment URL.

---

## Swagger UI — interactive API explorer

Cambium ships with a built-in Swagger UI. Once the server is running, open:

```
http://localhost:8080/docs/index.html
```

### What is Swagger?

**OpenAPI** (the standard, formerly known as Swagger) is a machine-readable specification that describes every endpoint in a REST API — its URL, HTTP method, parameters, request body shape, response codes, and authentication requirements. The spec lives in `docs/swagger.json` and `docs/swagger.yaml`.

**Swagger UI** is an interactive website that renders the OpenAPI spec as a visual API explorer. You can:
- Browse all endpoints grouped by tag (auth, chat, keys, threads, triage, etc.)
- Read request body schemas and response shapes
- Authenticate with your JWT token via the **Authorize** button
- Execute real requests against the running server directly from the browser

### Authenticating in Swagger UI

1. Register or login via `POST /auth/register` or `POST /auth/login` in Swagger UI — copy the `access_token` from the response
2. Click **Authorize** (top right of the page)
3. Enter `Bearer <your_access_token>` in the `BearerAuth` field
4. Click **Authorize** — all subsequent requests will include the token automatically

### Rhizome also has Swagger

Rhizome's internal FastAPI server exposes its own Swagger UI at:

```
http://localhost:8001/docs
```

FastAPI generates this automatically from Python type hints — no extra work needed. This shows the `/internal/agent` and `/internal/data/...` endpoints that Cambium calls. Useful during development to inspect what Cambium is proxying.

### Keeping the spec up to date

The spec is generated from annotations in the Go handler code. After changing any handler signature, adding an endpoint, or modifying a request/response type, regenerate it:

```bash
~/go/bin/swag init -g cmd/server/main.go -o docs
```

Commit the updated `docs/swagger.json` and `docs/swagger.yaml` alongside the code change. The spec is a first-class artifact — it should always reflect the live API.

---

## 1. Register

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"yourpassword"}'
```

Response:

```json
{"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}
```

A `refresh_token` cookie is also set automatically. Store the access token — you'll need it for every authenticated request.

---

## 2. Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"yourpassword"}'
```

Same response shape as register.

---

## 3. Set your provider key

Cambium stores your LLM provider key encrypted at rest. It is never returned to you — only used to inject into Rhizome requests.

```bash
curl -X PUT http://localhost:8080/api/v1/auth/keys \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"provider":"gemini","key":"AIzaSy..."}'
```

Check which providers are configured:

```bash
curl http://localhost:8080/api/v1/auth/keys \
  -H "Authorization: Bearer <access_token>"
# → {"gemini": true, "openai": false, "anthropic": false}
```

Supported providers: `gemini`, `openai`, `anthropic`.

---

## 4. Start a conversation thread

Before chatting, create a thread. Cambium generates a memorable botanical name for you:

```bash
curl -X POST http://localhost:8080/api/v1/threads \
  -H "Authorization: Bearer <access_token>"
# → {"thread_id": "silver-fern-cascade"}
```

Thread IDs are three-word botanical names (`silver-fern-cascade`, `ancient-lotus-dawn`).
Reuse the same `thread_id` to continue a conversation across multiple requests.

List your past conversations:

```bash
curl http://localhost:8080/api/v1/threads \
  -H "Authorization: Bearer <access_token>"
```

Get the full message history for a thread:

```bash
curl http://localhost:8080/api/v1/threads/silver-fern-cascade/messages \
  -H "Authorization: Bearer <access_token>"
```

## 5. Chat with the agent

Every chat request requires the `thread_id` from step 4.

**Non-streaming (wait for complete response):**

```bash
curl -X POST "http://localhost:8080/api/v1/chat?thread_id=my-thread-1" \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"message":"What should I do in my garden today?"}'
```

**Streaming (tokens arrive as they are produced):**

```bash
curl -X POST "http://localhost:8080/api/v1/chat/stream?thread_id=my-thread-1" \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"message":"What should I do in my garden today?"}'
```

SSE event format:

```
data: {"type":"token","content":"Based "}
data: {"type":"token","content":"on your "}
data: {"type":"token","content":"garden profile..."}
data: {"type":"done"}
```

If the agent pauses for confirmation (e.g. before applying a treatment plan), you'll receive an `interaction` event instead of `done`:

```
data: {"type":"interaction","payload":{"type":"confirmation","title":"Apply frost protection?","actions":["confirm","cancel"]}}
```

Resume after confirmation:

```bash
curl -X POST "http://localhost:8080/api/v1/chat/resume?thread_id=my-thread-1" \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"thread_id":"my-thread-1","resolution":"confirm"}'
```

---

## 6. Read garden data

Once the agent has populated your garden (via chat), all domain data is available directly:

```bash
# Tasks for today
curl "http://localhost:8080/api/v1/tasks/daily" \
  -H "Authorization: Bearer <access_token>"

# All your plants
curl "http://localhost:8080/api/v1/garden/plants" \
  -H "Authorization: Bearer <access_token>"

# Active alerts (weather, triage, pests)
curl "http://localhost:8080/api/v1/alerts" \
  -H "Authorization: Bearer <access_token>"

# Project progress
curl "http://localhost:8080/api/v1/projects/<project-id>/progress" \
  -H "Authorization: Bearer <access_token>"

# Unified entity search
curl "http://localhost:8080/api/v1/search?q=tomato&types=plant,task" \
  -H "Authorization: Bearer <access_token>"
```

---

## 7. Listen for notifications

Cambium exposes both a live SSE stream and a catch-up snapshot. The durable
source of truth is still Rhizome's `MonitorAlert` and `InteractionRecord`
tables; the live stream is best-effort.

```bash
curl -N "http://localhost:8080/api/v1/notifications/stream" \
  -H "Authorization: Bearer <access_token>"

curl "http://localhost:8080/api/v1/notifications" \
  -H "Authorization: Bearer <access_token>"
```

---

## 8. Trigger AI operations

Some operations require the agent to reason (rather than just read data). These use a dedicated trigger endpoint and require a `thread_id`:

```bash
# Run daily triage
curl -X POST http://localhost:8080/api/v1/triage/run \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"thread_id":"my-thread-1"}'

# Draft weather task changes
curl -X POST http://localhost:8080/api/v1/weather/tasks/draft \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"thread_id":"my-thread-1"}'

# Draft a treatment plan for an incident
curl -X POST http://localhost:8080/api/v1/incidents/<incident-id>/treatment \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"thread_id":"my-thread-1"}'
```

---

## 9. Refresh tokens

Access tokens expire after 15 minutes. Use the refresh cookie to get a new one:

```bash
curl -X POST http://localhost:8080/auth/refresh \
  -b "refresh_token=<your-refresh-token>"
```

The old refresh token is immediately revoked and a new one is issued (rotation on every use).

---

## 10. Token storage

- **Access token** — store in `localStorage` or memory. Short-lived (15 min). Send as `Authorization: Bearer <token>`.
- **Refresh token** — stored as an `httpOnly` cookie by the browser automatically. Never accessible to JavaScript.
