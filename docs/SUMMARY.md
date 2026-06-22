# Cambium Documentation

## Getting Started
- [Setup](getting-started/setup.md) — prerequisites, local dev modes, Postgres, env vars, running locally, troubleshooting
- [Using the API](getting-started/using-the-api.md) — Swagger UI, auth, chat, CRUD, triggers

## Architecture
- [Overview](architecture/overview.md) — system topology, components, DB ownership, provider key flow
- [API Surface](architecture/api-surface.md) — public route groups, ownership, and runtime path
- [Auth Flow](architecture/auth-flow.md) — JWT lifecycle, token rotation, security properties
- [Request Lifecycle](architecture/request-lifecycle.md) — data requests, SSE streaming, AI triggers, interaction pause/resume

## Roadmap
- [Roadmap Overview](roadmap/overview.md) — phase status, what's next, deployment plan

## Operations
- [Production Readiness](operations/production-readiness.md) — security, deployment, and operational checklist

## Historical Design Reference
- [Full Design Document](design.md) — original design and build-order record; current behavior may be superseded by architecture docs
- [Deferred Tests](DEFERRED_TESTS.md) — consciously skipped test areas with rationale

## Development
- [Code Organization](development/code-organization.md) — directory guide, module responsibilities, change rules
- [Adding Routes](development/adding-routes.md) — choosing native/proxy/agent/SSE patterns safely
- [Testing Guide](development/testing.md) — test layers, route testing rules, fake Rhizome pattern
