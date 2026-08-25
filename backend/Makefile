.DEFAULT_GOAL := help
SHELL := /bin/bash

# The Go proxy default is unreachable from some networks. Override with
# GOPROXY=direct on the command line if the mirror is not wanted.
export GOPROXY ?= https://goproxy.cn,direct

# Migration commands need a DSN, so .env is read to keep it defined in one
# place. It is read but NOT exported wholesale: a bare `export` would inject
# every variable in .env into every target, including `go test`, and tests that
# set their own environment would then be overridden by whatever happens to be
# in a developer's .env. Test results must not depend on that file.
#
# DATABASE_URL is passed explicitly to the targets that need it instead.
ifneq (,$(wildcard .env))
include .env
endif

MIGRATIONS_DIR := migrations

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_:-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# --------------------------------------------------------------------------
# Development
# --------------------------------------------------------------------------

# Targets that start the application need .env in the process environment.
# `set -a` exports for the duration of that one recipe line only, so nothing
# leaks into test targets.
LOAD_ENV := set -a; [ -f .env ] && . ./.env; set +a;

.PHONY: run
run: ## Run the HTTP server
	$(LOAD_ENV) go run ./cmd/server

.PHONY: config-check
config-check: ## Load and validate configuration, then report it
	$(LOAD_ENV) go run ./cmd/cli config:check

.PHONY: user-create
user-create: ## Create an account: make user-create name=owner
	@test -n "$(name)" || { echo "usage: make user-create name=owner"; exit 1; }
	$(LOAD_ENV) go run ./cmd/cli user:create --username "$(name)"

.PHONY: session-prune
session-prune: ## Delete every expired session
	$(LOAD_ENV) go run ./cmd/cli session:prune

.PHONY: build
build: ## Compile both binaries into bin/
	go build -o bin/server ./cmd/server
	go build -o bin/cli ./cmd/cli

.PHONY: tidy
tidy: ## Sync go.mod and go.sum
	go mod tidy

# --------------------------------------------------------------------------
# Quality
# --------------------------------------------------------------------------

.PHONY: fmt
fmt: ## Format all Go source
	gofmt -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: test
test: ## Run tests with the race detector (database tests skip)
	go test -race ./...

# The integration tests TRUNCATE users CASCADE, so they get their own database.
# Pointing them at DATABASE_URL would delete development data, and once entries
# exists the cascade would take written content with it. The tests refuse any
# database whose name does not end in _test, so this is enforced twice.
TEST_DB_NAME := alive_test
TEST_DATABASE_URL := postgres://alive:alive@127.0.0.1:5432/$(TEST_DB_NAME)?sslmode=disable

.PHONY: test-integration
test-integration: ## Run tests including those that need PostgreSQL
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -count=1 -race ./...

.PHONY: test-db-create
test-db-create: ## Create the integration test database and migrate it
	createdb -O alive $(TEST_DB_NAME) || true
	migrate -path $(MIGRATIONS_DIR) -database "$(TEST_DATABASE_URL)" up

.PHONY: test-db-drop
test-db-drop: ## Drop the integration test database
	dropdb --if-exists $(TEST_DB_NAME)

.PHONY: check
check: fmt vet test ## Format, vet and test

# --------------------------------------------------------------------------
# Database
# --------------------------------------------------------------------------
# Requires golang-migrate: brew install golang-migrate

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

.PHONY: migrate-version
migrate-version: ## Show the current schema version
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version

.PHONY: migrate-create
migrate-create: ## Create a migration pair: make migrate-create name=add_entries
	@test -n "$(name)" || { echo "usage: make migrate-create name=add_entries"; exit 1; }
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

.PHONY: sqlc
sqlc: ## Generate Go from SQL (produces nothing until sql/queries has files)
	sqlc generate
