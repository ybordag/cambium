# Using the API

A practical walkthrough of Cambium's API from registration through AI chat.

All examples use `curl`. Replace `localhost:8080` with your deployment URL.

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

## 6. Chat with the agent

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

## 7. Read garden data

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
