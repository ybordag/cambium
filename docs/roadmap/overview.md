# Cambium Roadmap

## Current status

Phases 0–5 are complete, along with the frontend API pass, Group B, and the
follow-on fixes for unified search/thread context, notifications, and static
Verdant serving. The current `main` branch contains the merged API surface.

| Phase | Status | What it delivered |
|---|---|---|
| Phase 0 — Postgres setup | ✓ done | Rhizome migrated to Postgres; `cambium` + `rhizome` schemas created |
| Phase 1 — Go skeleton | ✓ done | `go mod init`, `/health`, pgxpool, `cambium` schema migrations |
| Phase 2 — Auth + key management | ✓ done | Register/login/refresh/logout, JWT middleware, AES-256-GCM provider key storage |
| Phase 3 — Rhizome proxy | ✓ done | Rhizome HTTP client, SSE streaming, provider key injection, partial route wiring |
| Phase 4 — Full API surface | ✓ done | Core API surface wired; AI-trigger handlers; media stubs; comprehensive docs |
| Phase 5 — Thread management | ✓ done | Botanical name generator; `POST/GET/DELETE /api/v1/threads`; full conversation history; normalized `GET/PATCH /api/v1/threads/{id}/session-context` |
| Frontend API pass | ✓ done | Expanded route surface for task CRUD/series/dependencies, garden detail, calendar, shopping, activity stats |
| Group B + account | ✓ done | Quick care recording, incident PATCH/DELETE, manual treatment plans, native profile/password endpoints |
| #16 — Unified search + thread context | ✓ done | `GET /api/v1/search`; thread pinned context; fixed a method-collapsing bug in the data proxy |
| #19 — Notification SSE + sync | ✓ done | `GET /api/v1/notifications/stream`, `GET /api/v1/notifications` |
| #21 — Static frontend serving | ✓ done | Serves built Verdant Pages `dist/`, SPA fallback to `index.html` |

---

## What's next

### Garden spatial layout (#118 rhizome / #10 cambium)

Garden spatial layout model and map endpoints — not yet started.

### Media/image attachments (#117 rhizome / #9 cambium)

Media upload and garden object image attachment endpoints — not yet started.

### Production hardening and security ops

Cambium is ready for local/integrated development, but public exposure should
wait on a focused auth and key-operations hardening pass.

**1. Clock seam and token-expiry tests**

- Add an injectable clock in `internal/auth/jwt.go` for deterministic access
  token expiry tests.
- Use the same time-control pattern where refresh-token expiry is checked.
- Cover expired access tokens, valid boundary cases, and expired refresh tokens.

**2. Atomic refresh-token rotation**

- Replace the current separate `GetRefreshToken` + `RevokeRefreshToken` refresh
  flow with a DB-level consume operation that only succeeds when the token is
  unrevoked and unexpired.
- Add reuse and concurrency tests: the first refresh succeeds, subsequent or
  simultaneous attempts with the same token return `401`.

**3. Provider-key DB security tests**

- Add an integration test that stores a provider key through
  `PUT /api/v1/auth/keys`, verifies the DB value is not plaintext, decrypts the
  DB value, and confirms `GET /api/v1/auth/keys` still returns booleans only.

**4. Provider-key master-key rotation tooling**

- Add a rotation command, likely `cmd/rotate-keys`, that accepts
  `DATABASE_URL`, `CAMBIUM_OLD_ENCRYPTION_KEY`,
  `CAMBIUM_NEW_ENCRYPTION_KEY`, and optional `--dry-run`.
- In one transaction, decrypt each stored provider key with the old key,
  re-encrypt with the new key, and update the row.
- Refactor encryption helpers so tests and the rotation command can use
  explicit keys without mutating process env.
- Cover dry-run, one-row rotation, all-provider rotation, and undecryptable-row
  failure behavior.

**5. Auth rate limiting**

- Add brute-force protection for `POST /auth/register` and `POST /auth/login`.
- Start with an in-process limiter keyed by IP + route, and for login
  optionally IP + normalized email.
- Configure with env vars such as `AUTH_RATE_LIMIT_ENABLED`,
  `AUTH_RATE_LIMIT_WINDOW`, and `AUTH_RATE_LIMIT_MAX`.
- Return structured `429` responses.
- If Cambium runs multiple public replicas, graduate to Redis/Postgres-backed
  limiting or enforce limits at ingress.

**6. Refresh-cookie security policy**

- Add env-driven cookie policy, at minimum `COOKIE_SECURE`, and optionally
  configurable SameSite behavior.
- Keep local defaults developer-friendly; production docs should require secure
  refresh cookies under HTTPS.

Suggested implementation order:

1. Clock seam + token expiry tests.
2. Atomic refresh-token consume + reuse/concurrency tests.
3. Provider-key DB round-trip tests.
4. Provider-key rotation command.
5. Auth rate limiting.
6. Cookie security policy.

See [Production Readiness](../operations/production-readiness.md) and
[Deferred Tests](../DEFERRED_TESTS.md) for the current risk inventory.

### Multi-tenancy in Rhizome — audited 2026-06-20

Full audit of every model in `db/models.py` for missing/inconsistent user scoping is complete (see Rhizome's `CLAUDE.md`, "Known issues" + the two 2026-06-20 audit sections). Findings fixed: unscoped `GardeningProject` lookups, `IncidentReport` missing `user_id`, unscoped `get_activity_for_subject`, and `WeatherSnapshot`/`TriageSnapshot` being single-tenant (now scoped via `garden_profile_id`). `user_id == 1` is still the CLI-mode fallback by design — that's fine, since Cambium always supplies a real `user_id` from the verified JWT in production.

### gRPC migration

Switch Cambium→Rhizome transport to gRPC when Verdant needs token streaming with bidirectional flow (e.g. user sends mid-generation interrupts). SSE handles the current unidirectional streaming well; gRPC becomes worth the overhead only when the protocol changes.

---

## Intelligence track (Rhizome — not Cambium)

The following are Rhizome-side features that Cambium will proxy automatically once they exist:

| Initiative | What it adds to Cambium's API |
|---|---|
| Google Search grounding | No new endpoints — agent uses it internally during planning |
| RAG / knowledge base | No new endpoints — agent uses it internally |
| Unified/full-text search | `GET /api/v1/search` is wired; richer Rhizome full-text ranking can improve results behind the same route |
| iNaturalist pest monitoring | no new Cambium route expected; pest alerts should appear through existing alert/notification routes |
| Visual garden understanding | `POST /api/v1/media` — currently 501, implements when Epic 2 ships |

---

## Deployment

Current target: two DGX Spark nodes (Thor + Loki) via k3s.

```
Thor  — Cambium, Verdant, Rhizome #1
Loki  — Postgres, Rhizome #2
```

See `rhizome/docs/architecture/deployment.md` for the full topology, k3s setup, Helm charts, and Temporal future evolution.
