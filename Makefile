# E-cercise Backend — developer Makefile
# Run `make` or `make help` to list targets.

# ---- config (override on the CLI, e.g. `make db-seed DB_NAME=other`) --------
BINARY       ?= bin/e-cercise
PORT         ?= 8888
DB_CONTAINER ?= e-cercise-database
DB_USER      ?= pg
DB_NAME      ?= crud
DB_SEED      ?= data/db.sql
COMPOSE      ?= docker compose

GO           ?= go
GOFLAGS      ?=

.DEFAULT_GOAL := help
SHELL := /bin/bash

# ---- help -------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@echo "E-cercise Backend — make targets:"
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ---- setup / deps -----------------------------------------------------------
.PHONY: setup
setup: .env tidy ## First-time setup: create .env and download deps

.env: ## Create .env from .env.example (if missing)
	@test -f .env || (cp .env.example .env && echo "created .env from .env.example — edit your secrets")

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: deps
deps: ## Download Go module dependencies
	$(GO) mod download

# ---- build / run ------------------------------------------------------------
.PHONY: run
run: .env ## Run the server (go run, reads .env) on :$(PORT)
	$(GO) run $(GOFLAGS) main.go

.PHONY: build
build: ## Compile the server binary to $(BINARY)
	$(GO) build $(GOFLAGS) -o $(BINARY) .

.PHONY: start
start: build .env ## Build then run the compiled binary
	./$(BINARY)

# ---- quality ----------------------------------------------------------------
.PHONY: fmt
fmt: ## Format all Go code (go fmt)
	$(GO) fmt ./...

.PHONY: vet
vet: ## Static analysis (go vet)
	$(GO) vet ./...

.PHONY: lint
lint: ## Lint with golangci-lint (no-op with a note if not installed)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed — skipping (install: https://golangci-lint.run)"; \
	fi

.PHONY: test
test: ## Run the test suite
	$(GO) test $(GOFLAGS) ./...

.PHONY: check
check: fmt vet test ## fmt + vet + test

# ---- database (docker compose: postgres only) -------------------------------
.PHONY: db-up
db-up: ## Start the Postgres container and wait until healthy
	$(COMPOSE) up -d --wait database

.PHONY: db-down
db-down: ## Stop the Postgres container (keeps the volume/data)
	$(COMPOSE) down

.PHONY: db-nuke
db-nuke: ## Stop the Postgres container AND delete its data volume
	$(COMPOSE) down -v

.PHONY: db-wait
db-wait: ## Block until Postgres accepts connections
	@until docker exec $(DB_CONTAINER) pg_isready -U $(DB_USER) -d $(DB_NAME) >/dev/null 2>&1; do \
		echo "waiting for $(DB_NAME)..."; sleep 1; done; \
	echo "$(DB_NAME) ready"

.PHONY: db-seed
db-seed: db-wait ## Import the data/db.sql dump into the running database
	docker exec -i $(DB_CONTAINER) psql -q -U $(DB_USER) -d $(DB_NAME) < $(DB_SEED)
	@echo "seeded $(DB_NAME) from $(DB_SEED)"

.PHONY: db-reset
db-reset: db-up ## Drop, recreate, and re-seed the database from scratch
	docker exec $(DB_CONTAINER) psql -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_NAME) WITH (FORCE);"
	docker exec $(DB_CONTAINER) psql -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_NAME);"
	@$(MAKE) db-seed

.PHONY: psql
psql: ## Open a psql shell in the database container
	docker exec -it $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME)

.PHONY: db-logs
db-logs: ## Tail the Postgres container logs
	$(COMPOSE) logs -f database

# ---- one-shot local dev -----------------------------------------------------
.PHONY: dev
dev: setup db-up db-seed ## Full local bring-up: setup + db + seed, then run the server
	@$(MAKE) run

# ---- docker image (the deployable backend) ----------------------------------
.PHONY: docker-build
docker-build: ## Build the backend Docker image (tag: e-cercise-backend)
	docker build -t e-cercise-backend .

.PHONY: docker-run
docker-run: ## Run the backend image (needs a reachable DB + .env)
	docker run --rm --env-file .env -p $(PORT):8888 e-cercise-backend

# ---- housekeeping -----------------------------------------------------------
.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin
	$(GO) clean
