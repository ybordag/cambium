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

---

## 5. Verify

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
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
