# Request Lifecycle

How a request travels from Verdant through Cambium to Rhizome and back.

---

## Non-streaming data request

Example: `GET /api/v1/tasks/daily`

```
Verdant
  GET /api/v1/tasks/daily
  Authorization: Bearer eyJ...
        │
        ▼
Cambium: RequireAuth middleware
  1. Parse "Bearer eyJ..." from Authorization header
  2. jwt.ParseWithClaims(token, JWT_SECRET)
  3. Verify signature + expiry
  4. Extract user_id from sub claim
  5. Store user_id in request context
        │
        ▼
Cambium: proxyData("tasks/daily") handler
  6. UserIDFromContext(r.Context()) → "abc-123"
  7. rhizome.DataRequest(GET, "tasks/daily", "abc-123", queryParams, nil)
        │
        ▼
Rhizome: GET /internal/data/tasks/daily?user_id=abc-123
  8. current_user_id.set("abc-123")
  9. structured route handler builds `TaskSummaryView[]`
 10. SQLAlchemy query scoped to user_id=abc-123
        │
        ◀ JSON response
        │
Cambium: io.Copy(w, rhizomeBody)
        │
        ◀ JSON response to Verdant
```

Total latency: one DB query in Rhizome, no LLM call.

The data proxy preserves the original HTTP method. `PATCH`, `DELETE`, and
non-agent `POST` requests are forwarded to the matching Rhizome internal data
route instead of being collapsed into one method.

---

## Streaming chat request

Example: `POST /api/v1/chat/stream`

```
Verdant
  POST /api/v1/chat/stream?thread_id=thread-1
  Authorization: Bearer eyJ...
  Body: {"message": "What should I plant this week?"}
        │
        ▼
Cambium: RequireAuth → user_id = "abc-123"
        │
        ▼
Cambium: chatStream handler
  1. UserIDFromContext → user_id
  2. providerKey(user_id)
     → db.GetUserByID()
     → user.preferred_provider = "gemini"
     → auth.DecryptKey(user.encrypted_gemini_key) → "AIza..." (in memory only)
  3. rhizome.StreamAgent({
       user_id: "abc-123",
       thread_id: "thread-1",
       message: "What should I plant this week?",
       provider: "gemini",
       provider_key: "AIza...",
     })
        │
        ▼
Rhizome: POST /internal/agent/stream
  4. current_user_id.set("abc-123")
  5. config["configurable"]["user_id"] = "abc-123"
  6. config["configurable"]["provider"] = "gemini"
  7. config["configurable"]["provider_key"] = "AIza..."
  8. agent.astream_events({"messages": [HumanMessage]}, config)
        │ LangGraph graph runs:
        │   session_context_intake → weather_context_loader
        │   → triage_reasoner → llm_call
        │
        ▼ Gemini API (using user's key)
  9. on_chat_model_stream events
 10. yield: data: {"type":"token","content":"Based on..."}
 11. yield: data: {"type":"token","content":"your zone 9b..."}
        │
        ◀ SSE stream (open connection)
        │
Cambium: proxySSE(w, stream)
  12. w.Header().Set("Content-Type", "text/event-stream")
  13. io.Copy(w, rhizomeBody)   ← forwards each chunk immediately
  14. w.(http.Flusher).Flush()  ← pushes to client without buffering
        │
        ◀ SSE tokens arrive in Verdant one by one
 15. Verdant: reader.read() → decode → append to chat UI
```

---

## AI-trigger request

Example: `POST /api/v1/triage/run`

These follow the same flow as streaming chat but with a pre-built message:

```
Cambium: triggerTriage handler
  1. user_id from JWT
  2. thread_id from request body
  3. providerKey(user_id) → decrypt
  4. rhizome.RunAgent({
       message: "Run daily triage now and summarise the most urgent tasks.",
       ...
     })
```

The agent receives a natural-language instruction rather than a user-typed message. The response goes through the same Rhizome graph and is returned as a complete JSON response (non-streaming).

---

## Notification stream

Example: `GET /api/v1/notifications/stream`

```
Verdant
  GET /api/v1/notifications/stream
  Authorization: Bearer eyJ...
        │
        ▼
Cambium: RequireAuth → user_id = "abc-123"
        │
        ▼
Cambium: notificationStream handler
  1. forwards query params such as since
  2. rhizome.StreamData("notifications/stream", user_id, queryParams)
        │
        ▼
Rhizome: GET /internal/data/notifications/stream?user_id=abc-123
        │
        ◀ long-lived SSE stream
        │
Cambium: proxySSE(w, stream)
        │
        ◀ events arrive in Verdant
```

Live notification delivery is best-effort. Durable recovery comes from
`GET /api/v1/notifications`, `GET /api/v1/alerts`, and pending interaction
routes.

---

## Interaction pause/resume

When the Rhizome agent requires user confirmation (e.g. before applying an irreversible action), the LangGraph graph pauses via `interrupt()` and Rhizome returns an `interaction` event in the SSE stream:

```
data: {"type":"interaction","payload":{"type":"confirmation","title":"Approve frost protection for 3 plants?","actions":["confirm","cancel"]}}
data: (stream ends)
```

Verdant shows the confirmation UI. The user responds:

```
POST /api/v1/chat/resume/stream
{"thread_id":"thread-1","resolution":"confirm"}
```

Cambium resumes the graph via `StreamResume`, which calls `POST /internal/agent/resume/stream` on Rhizome. The graph continues from where it paused and streams the remaining response.

---

## Error handling

| Scenario | Cambium response |
|---|---|
| Missing/invalid JWT | `401 Unauthorized` — middleware rejects before handler runs |
| Rhizome unreachable | `502 Bad Gateway` — `"rhizome unavailable: ..."` |
| Rhizome returns non-200 | `502 Bad Gateway` — includes Rhizome's error body |
| Invalid request body | `400 Bad Request` |
| Route not found | `404 Not Found` (Go stdlib default) |
| Media endpoints | `501 Not Implemented` (Epic 2) |
