# Deferred Tests

Tests consciously deferred with rationale and re-enable criteria.

---

## Auth package

### Expired access token rejection
**What:** Issue a token with a past expiry date, verify it is rejected with 401.
**Why deferred:** Requires either mocking `time.Now()` in the JWT package or waiting 15 minutes. Neither is worth the setup cost right now.
**Re-enable when:** We add a clock abstraction to `internal/auth/jwt.go` (e.g. an injectable `time.Now` func), or add a short-lived token duration for testing.

### Refresh token reuse (rotation enforcement)
**What:** Use a refresh token once (gets rotated), then attempt to use the original token again — should return 401.
**Why deferred:** The logout test proves revocation works; the rotation path is exercised but the double-use case specifically is not.
**Re-enable when:** Adding security hardening tests before public or multi-user production exposure.

### Refresh token expiry
**What:** A refresh token past its `expires_at` should be rejected even if not revoked.
**Why deferred:** Same time-mocking problem as expired access token.
**Re-enable when:** Clock abstraction is added.

---

## DB package

### Direct SQL function tests
**What:** Unit tests for `InsertUser`, `GetUserByEmail`, `GetUserByID`, `SetProviderKey`, `ClearProviderKey`, `InsertRefreshToken`, `GetRefreshToken`, `RevokeRefreshToken`.
**Why deferred:** All are covered indirectly through the API integration tests. Direct tests add value mainly for debugging query regressions.
**Re-enable when:** A DB query regresses and we need faster isolation.

---

## API integration tests

### Concurrent refresh
**What:** Two simultaneous requests using the same refresh token — only one should succeed, the other should get 401.
**Why deferred:** Requires goroutine coordination in tests; low risk at current scale.
**Re-enable when:** Moving to production with multiple Cambium instances.

### Malformed JWT variants
**What:** Tokens with wrong algorithm header (`alg: none`), missing claims, corrupted signature.
**Why deferred:** `golang-jwt/jwt/v5` handles these internally; trust the library for now.
**Re-enable when:** Doing a security audit before public launch.

### Rate limiting on auth endpoints
**What:** More than N failed login attempts within a time window should return 429.
**Why deferred:** Rate limiting is listed as an open question in `docs/design.md` — not yet implemented.
**Re-enable when:** Rate limiting is added before public exposure.

### Key encryption round-trip through DB
**What:** Store an encrypted key via `PUT /api/v1/auth/keys`, then verify the raw value in the DB is not the plaintext and can be decrypted back to the original.
**Why deferred:** Requires reaching into the DB in a test — slightly invasive. The crypto unit tests prove the encryption is correct; the handler tests prove the key is stored.
**Re-enable when:** Adding a Cambium security audit test suite.

---

## Proxy and E2E tests

### Proxy route response validation (authenticated)
**What:** For each data proxy route (e.g. `GET /api/v1/tasks/daily`, `GET /api/v1/garden/plants`), verify that an authenticated request with a running Rhizome returns a meaningful response body rather than a 502.
**Why deferred:** Requires Rhizome to be running at `RHIZOME_INTERNAL_URL`. The security sweep in `security_test.go` covers 401 rejection; the proxy mechanics are covered by `internal/rhizome/client_test.go` using a fake Rhizome server. The combination is sufficient for unit/integration testing.
**Re-enable when:** E2E test suite is set up (both services running). Use `pytest` + `requests` against live Cambium and verify side effects in Postgres directly.

### AI trigger endpoint responses
**What:** `POST /api/v1/triage/run`, `/weather/tasks/draft`, `/incidents/{id}/treatment`, `/projects/{id}/tasks/generate` — verify that an authenticated request with a valid thread_id and a running Rhizome returns an agent response.
**Why deferred:** These route to the LangGraph agent, which requires a live LLM API key and a running Rhizome. Testing the agent response content is outside the scope of Cambium unit tests.
**Re-enable when:** E2E test suite is set up. These are good candidates for smoke tests that run in CI against a staging environment.

### 502 handling under Rhizome failure
**What:** When Rhizome is unreachable, authenticated proxy requests should return 502 with a structured error body, not panic or hang.
**Why deferred:** Covered structurally by `TestRunAgent_RhizomeError` in `client_test.go` (client returns error on non-200). The handler wraps the error into `writeError(w, 502, ...)`. A targeted test would spin up a fake Rhizome that returns 503.
**Re-enable when:** Adding integration tests with a configurable fake Rhizome server.

---

## Search and thread context (#16)

### `rhizomeErrorDetail` fallback path
**What:** `createThread`'s 400 handling extracts Rhizome's `{"detail": "..."}` message via `rhizomeErrorDetail`. The fallback string (`"rhizome rejected the request"`) fires when the body has no `detail` field or fails to decode — that branch is untested.
**Why deferred:** Low risk — Rhizome's FastAPI `HTTPException` always includes `detail`; this only guards against a malformed or empty body.
**Re-enable when:** Touching `rhizomeErrorDetail` again, or doing a hardening pass on the proxy error paths.

### Thread-ID-collision retry with `initial_context`
**What:** `createThread` retries once with a new botanical ID when Rhizome reports `created=false` (rare hash collision). Whether the retry payload correctly resends `initial_context` is untested — only collision-free creation paths are covered.
**Why deferred:** The retry payload is built once and reused (`payload["thread_id"] = threadID`), so the existing structure should carry `initial_context` through correctly by construction. Exercising the actual collision branch requires forcing two identical generated IDs, which isn't easily triggerable without mocking `generateThreadID`.
**Re-enable when:** `generateThreadID` gets a test seam (e.g. injectable RNG), or collisions become observable in practice.

### `proxyData` (literal-path routes) under PATCH
**What:** Routes like `PATCH /api/v1/garden/profile` use the bare `proxyData` helper rather than `proxyDataWithPathParam`. The PATCH-forwarding regression test (`TestDispatchData_PATCHForwardsAsPatchNotPost`) only exercises the path-param variant.
**Why deferred:** Both helpers call the same shared `dispatchData`, so the method-dispatch logic under test is identical; the risk of `proxyData` specifically regressing without `proxyDataWithPathParam` also regressing is low.
**Re-enable when:** `proxyData` and `proxyDataWithPathParam` diverge in implementation (e.g. one gets path-specific logic the other doesn't).

### Transport failure for `/search` and `/threads/{id}/context`
**What:** When Rhizome is unreachable, `GET /api/v1/search`, `POST /api/v1/threads/{id}/context`, and the DELETE variant should return 502, consistent with every other proxy route.
**Why deferred:** Same underlying mechanism as the existing "502 handling under Rhizome failure" deferral above — `dispatchData` surfaces the client error through `writeError(w, 502, ...)` for any route. Not re-verified per-route.
**Re-enable when:** Adding a dedicated "Rhizome down" smoke test that sweeps all proxy routes in one pass, rather than one per route.
