GO ?= go
SWAG ?= $(HOME)/go/bin/swag
PORT ?= 8080
BINARY ?= cambium
RHIZOME_DIR ?= ../rhizome
RHIZOME_PORT ?= 8001
POSTGRES_CONTAINER ?= rhizome-pg
POSTGRES_PASSWORD ?= dev
POSTGRES_DB ?= postgres
POSTGRES_PORT ?= 5432
PKG ?= ./internal/api
RUN ?=

.PHONY: help
help:
	@printf '%s\n' 'Cambium targets:'
	@printf '%s\n' ''
	@printf '%s\n' 'Setup:'
	@printf '%s\n' '  make setup              Create .env if needed and download Go dependencies'
	@printf '%s\n' '  make env-file           Create .env from .env.example if missing'
	@printf '%s\n' '  make deps               Download Go module dependencies'
	@printf '%s\n' '  make tidy               Tidy go.mod/go.sum'
	@printf '%s\n' '  make install-swag       Install the Swagger generator'
	@printf '%s\n' ''
	@printf '%s\n' 'Postgres:'
	@printf '%s\n' '  make postgres-up        Start or create the local Postgres container'
	@printf '%s\n' '  make postgres-wait      Wait until the local Postgres container is ready'
	@printf '%s\n' '  make postgres-check     Verify the local Postgres container responds'
	@printf '%s\n' '  make postgres-logs      Show local Postgres container logs'
	@printf '%s\n' ''
	@printf '%s\n' 'Dev stack:'
	@printf '%s\n' '  make dev-rhizome        Run Rhizome API via RHIZOME_DIR'
	@printf '%s\n' '  make dev-cambium        Run Cambium with .env and RHIZOME_INTERNAL_URL'
	@printf '%s\n' '  make dev-stack          Run Rhizome and Cambium together'
	@printf '%s\n' '  make dev-stack-db       Start Postgres, wait, then run the backend stack'
	@printf '%s\n' '  make stack-health       Check Rhizome and Cambium health endpoints'
	@printf '%s\n' ''
	@printf '%s\n' 'Run and build:'
	@printf '%s\n' '  make run                Run Cambium with current shell environment'
	@printf '%s\n' '  make run-env            Run Cambium after loading .env'
	@printf '%s\n' '  make build              Build the Cambium binary'
	@printf '%s\n' '  make clean              Remove the Cambium binary'
	@printf '%s\n' '  make health             Check the local health endpoint'
	@printf '%s\n' '  make docs-ui            Open local Swagger UI'
	@printf '%s\n' ''
	@printf '%s\n' 'Tests:'
	@printf '%s\n' '  make test               Run the full Go test suite'
	@printf '%s\n' '  make test-api           Run API tests'
	@printf '%s\n' '  make test-auth          Run auth tests'
	@printf '%s\n' '  make test-rhizome       Run Rhizome client tests'
	@printf '%s\n' '  make test-one RUN=...   Run one test; override PKG=... if needed'
	@printf '%s\n' '  make check              Alias for make test'
	@printf '%s\n' ''
	@printf '%s\n' 'Swagger and Docker:'
	@printf '%s\n' '  make swagger            Regenerate committed Swagger docs'
	@printf '%s\n' '  make swagger-check      Validate Swagger generation into /tmp'
	@printf '%s\n' '  make swagger-ui         Alias for make docs-ui'
	@printf '%s\n' '  make docker-build       Build a local cambium:latest image'

.PHONY: setup
setup: env-file deps

.PHONY: env-file
env-file:
	@test -f .env || cp .env.example .env

.PHONY: deps
deps:
	$(GO) mod download

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: install-swag
install-swag:
	$(GO) install github.com/swaggo/swag/cmd/swag@latest

.PHONY: postgres-up
postgres-up:
	docker start $(POSTGRES_CONTAINER) || docker run --name $(POSTGRES_CONTAINER) -e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) -e POSTGRES_DB=$(POSTGRES_DB) -p $(POSTGRES_PORT):5432 -v rhizome_pgdata:/var/lib/postgresql/data -d postgres:16

.PHONY: postgres-wait
postgres-wait:
	until docker exec $(POSTGRES_CONTAINER) pg_isready -U postgres; do sleep 1; done

.PHONY: postgres-check
postgres-check:
	docker exec $(POSTGRES_CONTAINER) psql -U postgres -c "SELECT version();"

.PHONY: postgres-logs
postgres-logs:
	docker logs $(POSTGRES_CONTAINER)

.PHONY: dev-rhizome
dev-rhizome:
	$(MAKE) -C $(RHIZOME_DIR) api PORT=$(RHIZOME_PORT)

.PHONY: dev-cambium
dev-cambium:
	set -a; . ./.env; set +a; PORT=$(PORT) RHIZOME_INTERNAL_URL=http://localhost:$(RHIZOME_PORT) $(GO) run ./cmd/server/

.PHONY: dev-stack
dev-stack:
	$(MAKE) -j2 dev-rhizome dev-cambium

.PHONY: dev-stack-db
dev-stack-db: postgres-up postgres-wait dev-stack

.PHONY: stack-health
stack-health:
	curl http://localhost:$(RHIZOME_PORT)/health
	curl http://localhost:$(PORT)/health

.PHONY: run
run:
	$(GO) run ./cmd/server/

.PHONY: run-env
run-env:
	set -a; . ./.env; set +a; $(GO) run ./cmd/server/

.PHONY: build
build:
	$(GO) build -o $(BINARY) ./cmd/server/

.PHONY: clean
clean:
	rm -f $(BINARY)

.PHONY: health
health:
	curl http://localhost:$(PORT)/health

.PHONY: docs-ui
docs-ui:
	open http://localhost:$(PORT)/docs/index.html

.PHONY: test
test:
	$(GO) test ./...

.PHONY: test-api
test-api:
	$(GO) test ./internal/api

.PHONY: test-auth
test-auth:
	$(GO) test ./internal/auth

.PHONY: test-rhizome
test-rhizome:
	$(GO) test ./internal/rhizome

.PHONY: test-one
test-one:
	@test -n "$(RUN)" || (printf '%s\n' 'Usage: make test-one RUN=TestName [PKG=./internal/api]' && exit 1)
	$(GO) test $(PKG) -run $(RUN)

.PHONY: check
check: test

.PHONY: swagger
swagger:
	$(SWAG) init -g cmd/server/main.go -o docs

.PHONY: swagger-check
swagger-check:
	rm -rf /tmp/cambium-swagger
	$(SWAG) init -g cmd/server/main.go -o /tmp/cambium-swagger

.PHONY: swagger-ui
swagger-ui: docs-ui

.PHONY: docker-build
docker-build:
	docker build -t cambium:latest .
