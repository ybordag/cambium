# Setup

How to run Cambium locally alongside Rhizome.

---

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.25+ | Build and run Cambium |
| Docker | any | Run Postgres |
| Python 3.12 + conda | — | Run Rhizome |

No virtual environment is needed for Go. Dependencies are resolved from `go.mod` automatically.

---

## Local Development Modes

| Mode | Run Cambium | Run Rhizome | Run Verdant build | Use when |
|---|---|---|---|---|
| Auth/API gateway only | yes | no | optional | Working on auth, provider keys, static serving, route registration, or docs |
| Cambium + Rhizome | yes | yes | optional | Working on proxied API routes, chat, triggers, notifications, or end-to-end backend behavior |
| Full app | yes | yes | yes | Testing Verdant against the real public API and SPA fallback |

Cambium can start without Rhizome, but proxied `/api/v1/...` requests will
return `502 Bad Gateway` until `RHIZOME_INTERNAL_URL` points at a running
Rhizome server.

---

## 1. Start Postgres

If you haven't already:

```bash
docker run \
  --name rhizome-pg \
  -e POSTGRES_PASSWORD=dev \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 \
  -v rhizome_pgdata:/var/lib/postgresql/data \
  -d postgres:16
```

Verify it's running:

```bash
docker exec rhizome-pg psql -U postgres -c "SELECT version();"
```

The `cambium` schema is created automatically when Cambium starts for the first time.

---

## 2. Set up environment variables

Copy the example file:

```bash
cp .env.example .env
```

Edit `.env` with real values:

```bash
DATABASE_URL=postgresql://postgres:dev@localhost:5432/postgres
JWT_SECRET=<generate with: openssl rand -hex 32>
CAMBIUM_ENCRYPTION_KEY=<generate with: openssl rand -hex 16>
RHIZOME_INTERNAL_URL=http://localhost:8001
PORT=8080
STATIC_DIR=./dist
```

**Important notes:**
- `STATIC_DIR` points at the built Verdant Pages `dist/` folder — Cambium serves it for any path not claimed by `/api/v1/*`, `/auth/*`, `/health`, or `/docs/*`, with SPA fallback to `index.html`. Defaults to `./dist` if unset.
- `JWT_SECRET` must be at least 32 bytes
- `CAMBIUM_ENCRYPTION_KEY` must be exactly 32 bytes — this encrypts user provider keys at rest
- `DATABASE_URL` uses plain `postgresql://` (pgx driver), not `postgresql+psycopg2://` which Rhizome uses
- Never commit `.env` — it is gitignored

---

## 3. Start Rhizome

Cambium calls Rhizome over HTTP — Rhizome must be running before Cambium can proxy any AI or data requests.

```bash
cd ../rhizome
conda activate RHIZOME_ENV
python server.py          # FastAPI on port 8001
```

Rhizome reads its own `.env` (which has `DATABASE_URL`, `GOOGLE_API_KEY`, etc.).

For shared Postgres local development, Rhizome's `DATABASE_URL` uses the
SQLAlchemy driver form, for example:

```bash
DATABASE_URL=postgresql+psycopg2://postgres:dev@localhost:5432/postgres
```

Cambium's `DATABASE_URL` intentionally uses plain `postgresql://` because it
connects through Go's `pgx` driver.

---

## 4. Start Cambium

```bash
# Source env vars from .env (bash)
export $(grep -v '^#' .env | xargs)

go run ./cmd/server/
# → cambium listening on :8080
```

Or pass env vars inline:

```bash
DATABASE_URL="postgresql://postgres:dev@localhost:5432/postgres" \
JWT_SECRET="your-secret" \
CAMBIUM_ENCRYPTION_KEY="your-32-byte-key" \
go run ./cmd/server/
```

On first run, Cambium creates the `cambium.users` and `cambium.refresh_tokens` tables automatically.

If `STATIC_DIR` points to a built Verdant app, Cambium also serves that app at
`http://localhost:8080/`. API, auth, health, and docs routes continue to take
precedence over the SPA fallback.

---

## 5. Verify

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

For a proxied route, verify Rhizome is reachable through Cambium:

```bash
curl "http://localhost:8080/api/v1/tasks/daily" \
  -H "Authorization: Bearer <access_token>"
```

---

## Build for production

```bash
go build -o cambium ./cmd/server/
./cambium               # reads env vars from environment
```

The binary is statically linked — copy it to any Linux server (same architecture) and run it directly. No Go installation needed on the target machine.

---

## Running tests

```bash
go test ./...
```

API integration tests require a running Postgres instance (uses `DATABASE_URL`). Auth and crypto unit tests run without any external dependencies.

For route or handler changes, also regenerate and commit Swagger output:

```bash
~/go/bin/swag init -g cmd/server/main.go -o docs
```

---

## Makefile shortcuts

Cambium includes a `Makefile` for common local workflows. Run:

```bash
make help
```

Useful examples:

```bash
make setup
make postgres-up
make dev-stack
make dev-stack-db
make run-env
make test
make test-security
make test-proxy
make test-one RUN=TestDispatchData_PATCHForwardsAsPatchNotPost
make swagger
make swagger-check
make clean-swagger-check
```

The Makefile wraps the documented Go, Postgres, Swagger, and local health-check
commands. `make run` uses the current shell environment; `make run-env` sources
`.env` first. `make swagger` updates the committed Swagger artifacts under
`docs/`; `make swagger-check` generates into `/tmp/cambium-swagger` without
changing tracked files, and `make clean-swagger-check` removes that temporary
output.

Use `make dev-stack` to run Rhizome and Cambium together in the foreground with
interleaved logs; `Ctrl-C` stops both processes. Use `make dev-stack-db` to
start/wait for the shared local Postgres container first, then start both
services. By default, Cambium expects Rhizome at `../rhizome` and runs Rhizome
on port `8001`; override with `RHIZOME_DIR=...`, `RHIZOME_PORT=...`, or
`PORT=...` when needed. `make stack-health` checks both health endpoints.

The Makefile intentionally does not include Kubernetes apply/deploy or
destructive Postgres reset targets. Those operations depend on the current
cluster, registry, and database state.

Note that Cambium's `make swagger` regenerates committed Swagger files from Go
handler annotations. Rhizome's `make swagger` exports a FastAPI OpenAPI JSON
snapshot instead, so the same target name has repo-specific behavior.

---

## Troubleshooting

### `docker: Conflict. The container name "rhizome-pg" is already in use`

Start the existing container:

```bash
docker start rhizome-pg
```

Or inspect it before removing anything:

```bash
docker ps -a --filter name=rhizome-pg
```

### `connect: connection refused` or `database: ...`

Postgres is not reachable from Cambium. Check:

- `docker ps` shows `rhizome-pg` running.
- `DATABASE_URL` is set in the shell where `go run ./cmd/server/` starts.
- Cambium uses `postgresql://...`, not `postgresql+psycopg2://...`.
- Port `5432` is not already bound by a different local Postgres.

### `CAMBIUM_ENCRYPTION_KEY must be exactly 32 bytes`

The value is used directly as the AES-256-GCM key. Generate a 32-character
ASCII value:

```bash
openssl rand -hex 16
```

Paste the resulting 32 hex characters as `CAMBIUM_ENCRYPTION_KEY`.

### `JWT_SECRET must be at least 32 bytes`

Generate a longer signing secret:

```bash
openssl rand -hex 32
```

### Proxied API requests return `502 Bad Gateway`

Cambium is running, but Rhizome is unavailable or returning an error. Check:

- Rhizome is running with `python server.py`.
- `RHIZOME_INTERNAL_URL` matches Rhizome's port, usually
  `http://localhost:8001`.
- Rhizome has its own `.env` and database dependencies configured.
- Rhizome Swagger is reachable at `http://localhost:8001/docs`.

### `http://localhost:8080/` returns a file error or blank page

`STATIC_DIR` probably does not point at a built Verdant `dist/` directory.
Either build Verdant and point `STATIC_DIR` at its output, or use Cambium only
through `/api/v1/*`, `/auth/*`, `/health`, and `/docs/*` while doing backend
work.

### `/docs/index.html` is stale after route changes

Regenerate Swagger and restart Cambium:

```bash
~/go/bin/swag init -g cmd/server/main.go -o docs
go run ./cmd/server/
```

### `swag: command not found`

Install the Swagger generator:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

Then rerun:

```bash
~/go/bin/swag init -g cmd/server/main.go -o docs
```
