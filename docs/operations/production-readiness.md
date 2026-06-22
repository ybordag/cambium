# Production Readiness

Cambium is functional for local and integrated development, but public
deployment requires a few explicit security and operations checks. This page is
the checklist before exposing Cambium outside a trusted development network.

---

## Current State

Cambium currently provides:

- JWT registration/login/session/logout.
- Refresh-token rotation with hashed server-side storage.
- AES-256-GCM provider-key encryption at rest.
- Public `/api/v1` proxy surface for Verdant.
- Static Verdant serving from `STATIC_DIR`.
- Chat and notification SSE proxying.
- Cambium-owned schema migrations at startup.

Known production gaps:

- Auth rate limiting is not implemented.
- `CAMBIUM_ENCRYPTION_KEY` rotation tooling is not implemented.
- Media upload routes intentionally return `501 Not Implemented`.
- Some security hardening tests are deferred in
  [Deferred Tests](../DEFERRED_TESTS.md).
- Live notification SSE depends on Rhizome's current delivery model; durable
  recovery should use alerts, interactions, and notification snapshots.

---

## Required Before Public Exposure

### Transport And Hosting

- Serve Cambium only over HTTPS.
- Terminate TLS at the ingress/load balancer or at Cambium's deployment
  boundary.
- Set secure production hostnames in ingress and frontend config.
- Keep Rhizome internal routes private to the cluster/network.
- Do not expose Postgres directly outside the private network.

### Secrets

- Use a high-entropy `JWT_SECRET`; do not reuse local development values.
- Use a unique 32-byte `CAMBIUM_ENCRYPTION_KEY`; do not rotate it casually
  without a re-encryption plan.
- Store secrets in Kubernetes secrets or another deployment secret manager, not
  in git or baked images.
- Ensure logs never include provider keys, refresh tokens, or access tokens.

### Auth Hardening

- Add rate limiting for `POST /auth/register` and `POST /auth/login`.
- Add tests for refresh-token reuse and concurrent refresh behavior.
- Add tests for expired access and refresh tokens once a clock seam exists.
- Decide whether production refresh cookies need the `Secure` attribute set at
  the application layer or enforced by deployment/session policy.
- Review the access-token storage strategy with Verdant before public launch.

### Provider-Key Operations

- Add a master-key rotation script before storing long-lived real user provider
  keys at scale.
- Rotation must decrypt with the old key, re-encrypt with the new key, and swap
  deployment secrets atomically.
- Confirm backups and restore procedures preserve encrypted provider-key rows
  and the corresponding master key lifecycle.

### Database And Migrations

- Confirm the `cambium` schema exists and startup migrations are idempotent in
  the target environment.
- Confirm Cambium never receives permissions to query the `rhizome` schema.
- Back up Postgres before production migrations or key-rotation work.
- Monitor connection pool usage under concurrent Verdant traffic.

### Proxy And Service Health

- Configure health checks against `GET /health`.
- Monitor `502` rates from Rhizome proxy failures.
- Keep `RHIZOME_INTERNAL_URL` private and stable.
- Confirm method-preserving proxy behavior remains covered by tests when adding
  new `PATCH` or `DELETE` routes.
- Confirm streaming routes are not buffered by ingress/proxy configuration.

### Static Frontend Serving

- Build Verdant separately and point `STATIC_DIR` at the immutable build output.
- Confirm `/api/v1/*`, `/auth/*`, `/health`, and `/docs/*` route precedence
  before enabling SPA fallback.
- Decide whether Cambium should serve docs publicly in production or restrict
  `/docs/` to trusted environments.

### Media

- Treat `/api/v1/media` as unavailable until the media/vision implementation
  lands.
- Do not build Verdant flows that depend on media upload without handling `501`.

---

## Operational Smoke Tests

Before promoting a deployment:

```bash
curl https://<host>/health
```

Then verify:

- register or login returns an access token and refresh cookie
- `GET /auth/session` works with the access token
- `PUT /api/v1/auth/keys` stores a provider key and `GET /api/v1/auth/keys`
  returns booleans only
- `POST /api/v1/threads` creates a thread
- `GET /api/v1/tasks/daily` reaches Rhizome through Cambium
- `POST /api/v1/chat/stream` streams SSE through ingress
- `GET /api/v1/notifications` returns a structured snapshot
- `GET /api/v1/notifications/stream` is not buffered
- unauthenticated protected routes return `401`

---

## Deployment References

- [Setup](../getting-started/setup.md) for local env vars and startup.
- [Auth Flow](../architecture/auth-flow.md) for token and provider-key
  behavior.
- [Request Lifecycle](../architecture/request-lifecycle.md) for proxy and SSE
  behavior.
- [Testing Guide](../development/testing.md) for current coverage and deferred
  E2E areas.
- Rhizome's deployment docs for the shared cluster/Postgres topology.
