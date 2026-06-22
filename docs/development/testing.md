# Testing Guide

Cambium's tests are Go unit and handler tests focused on the public API
gateway: auth, provider-key encryption, route protection, Rhizome proxying,
thread creation, static serving, and the small Rhizome client.

Most tests do not require a live Rhizome process. Proxy behavior is usually
tested with `httptest.Server` standing in for Rhizome, which keeps the suite
fast and deterministic.

---

## Running Tests

Run the full suite:

```bash
make test
```

Run one package:

```bash
make test-api
make test-auth
make test-rhizome
```

Run one test:

```bash
make test-one RUN=TestDispatchData_PATCHForwardsAsPatchNotPost
```

Focused route-change checks:

```bash
make test-security
make test-proxy
```

Some auth and DB-backed handler tests need `DATABASE_URL` pointing at a running
Postgres instance. Pure crypto/JWT/client tests do not need external services.

---

## Test Layers

### `internal/auth`

Pure unit tests for cryptographic helpers:

- JWT issue/verify behavior.
- bcrypt password hashing and checking.
- AES-256-GCM provider-key encryption/decryption.

These tests should not import HTTP handlers or DB packages.

### `internal/rhizome`

Client tests use a fake Rhizome `httptest.Server`. They verify:

- agent requests hit the correct internal paths
- non-200 Rhizome responses become client errors
- data requests include `user_id`
- `DataRequest` preserves HTTP methods such as `PATCH` and `DELETE`
- `StreamData` handles long-lived SSE GET routes

Use this layer when changing `internal/rhizome/client.go`.

### `internal/api`

Handler and router tests cover Cambium's public boundary:

- auth/register/login/session/profile/password behavior
- provider-key set/list/delete behavior
- route security sweeps for protected endpoints
- proxy dispatch into fake Rhizome data routes
- chat/trigger proxy mechanics
- unified search and thread-context forwarding
- botanical thread-name generation
- static Verdant serving and route precedence

Use this layer when changing `routes.go`, handlers, middleware, proxy logic, or
static serving.

---

## Route Testing Rules

When adding or changing a route:

- Add it to `TestAllProtectedRoutesReject401` if it should require auth.
- Add a positive handler or proxy test when Cambium owns behavior directly.
- For Rhizome data proxy routes, verify method, path, query params, `user_id`,
  and body forwarding with a fake Rhizome server when the route has unusual
  behavior.
- For AI-trigger routes, verify Cambium builds the intended agent request and
  forwards provider context.
- For SSE routes, assert `Content-Type: text/event-stream` and body forwarding.
- Regenerate Swagger if handler annotations or request/response types changed.

The method-preserving proxy path is important. `PATCH`, `DELETE`, and non-agent
`POST` routes must reach Rhizome with the same HTTP method Verdant sent.

For the full route-change workflow, see [Adding Routes](adding-routes.md).

---

## Fake Rhizome Pattern

Most proxy tests should use a fake Rhizome server rather than a live Rhizome
process:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPatch {
        t.Fatalf("expected PATCH, got %s", r.Method)
    }
    if r.URL.Query().Get("user_id") != "user-1" {
        t.Fatalf("user_id not forwarded")
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"ok":true}`))
}))
t.Cleanup(srv.Close)
t.Setenv("RHIZOME_INTERNAL_URL", srv.URL)
```

This gives Cambium confidence that it sends the right request. Rhizome owns
testing of domain behavior behind the internal route.

---

## What Belongs In E2E

Cambium intentionally defers some checks until both Cambium and Rhizome are
running together:

- live Rhizome route response validation
- LLM-backed AI trigger behavior
- provider-key use against a real model provider
- full chat/interaction resume flows
- multi-service notification behavior

Track consciously deferred areas in [Deferred Tests](../DEFERRED_TESTS.md) with
a reason and a clear re-enable condition.

---

## Swagger Checks

Swagger docs are generated from handler annotations. After changing public
routes, request bodies, response bodies, or auth requirements, run:

```bash
make swagger
```

Commit `docs/swagger.json` and `docs/swagger.yaml` alongside the code change.

---

## Practical Coverage Checklist

For a route or auth change, confirm:

- unauthenticated protected requests return `401`
- malformed request bodies return `400`
- Cambium-owned errors use structured JSON via `writeError`
- Rhizome transport failures return `502`
- provider keys are never returned to clients or logged
- tests do not require live external services unless explicitly marked as E2E
