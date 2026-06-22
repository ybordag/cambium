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
| Phase 5 — Thread management | ✓ done | Botanical name generator; `POST/GET/DELETE /api/v1/threads`; full conversation history |
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

### Rate limiting

Auth endpoints (`/auth/register`, `/auth/login`) need brute-force protection. Options:
- `golang.org/x/time/rate` (in-process, resets on restart)
- Redis-backed rate limiter (survives restarts, works across multiple Cambium instances)

Decision needed before production deployment. See
[Production Readiness](../operations/production-readiness.md).

### Multi-tenancy in Rhizome — audited 2026-06-20

Full audit of every model in `db/models.py` for missing/inconsistent user scoping is complete (see Rhizome's `CLAUDE.md`, "Known issues" + the two 2026-06-20 audit sections). Findings fixed: unscoped `GardeningProject` lookups, `IncidentReport` missing `user_id`, unscoped `get_activity_for_subject`, and `WeatherSnapshot`/`TriageSnapshot` being single-tenant (now scoped via `garden_profile_id`). `user_id == 1` is still the CLI-mode fallback by design — that's fine, since Cambium always supplies a real `user_id` from the verified JWT in production.

### CAMBIUM_ENCRYPTION_KEY rotation

If the master encryption key ever needs rotating, all stored provider keys must be re-encrypted atomically. A rotation script is needed before storing real user keys in production. Pattern:
1. Decrypt all keys with old key
2. Re-encrypt with new key
3. Swap env var + redeploy atomically

See [Production Readiness](../operations/production-readiness.md) for the
broader security checklist.

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
