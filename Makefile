# Vard — run `make` for the list of targets.

SHELL         := bash
.SHELLFLAGS   := -eu -o pipefail -c
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
MAKEFLAGS     += --no-print-directory

APPS    := console id
# Not ./...: that walks into web/*/node_modules.
PKGS    := . ./cmd/... ./internal/... ./i18n/... ./tests/...
GO_DIRS := cmd internal i18n layers_test.go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# The end-to-end tests borrow the console's Prettier and TypeScript, with
# settings of their own.
COMPOSE := docker compose -f compose.yaml --env-file compose.env

# $(call each-app,command): run the command in every app, stopping at the
# first that fails.
each-app = for app in $(APPS); do echo "==> web/$$app"; (cd web/$$app && $(1)) || exit 1; done

.PHONY: help setup deps tidy dev start run build build-api build-web generate \
	fmt lint lint-go lint-web test test-go test-web test-integration bench vuln audit check ci \
	migrate-up migrate-down migrate-status migrate-new db-create db-reset db-psql \
	docker-up docker-local docker-env db-backup docker-down docker-reset docker-logs docker-build docker-ps docker-config \
	clean rename

help: ## Show this help
	@awk -F ':.*## ' '/^## / {printf "\n\033[1m%s\033[0m\n", substr($$0, 4)} /^[a-z -]+:.*## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Setup

setup: ## Prepare a fresh checkout: .env, the database, dependencies
	@scripts/setup.sh

deps: ## Install the Go modules, both apps' packages and the API description's linter
	go mod download
	cd web && bun install --frozen-lockfile
	cd api/openapi && bun install --frozen-lockfile

tidy: ## Tidy go.mod and go.sum
	go mod tidy

## Run

dev: ## Start the API and both apps with hot reload — http://localhost:5173, :5174
	@scripts/start.sh dev

start: ## Start the API and both apps from production builds — `make start env=dev` for hot reload
	@scripts/start.sh $(or $(env),prod)

run: ## Start only the API — :8080 public, :8081 admin
	go run ./cmd/vard

## Build

build: build-api build-web ## Build the API and both apps

build-api: ## Build the API into bin/vard
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/vard ./cmd/vard

build-web: ## Build both apps into web/*/build (run each with `node build`)
	cd web && bun install --frozen-lockfile
	@$(call each-app,bun run build)

generate: ## Rewrite the generated files: the shared styles' tokens, themes and fonts, both apps' API types
	cd web/console && bun run theme
	go run ./api/openapi/bundle bin/openapi
	cd web/console && ../node_modules/.bin/openapi-typescript ../../bin/openapi/admin.json -o src/lib/api/schema.d.ts && ../node_modules/.bin/prettier --write src/lib/api/schema.d.ts
	cd web/id && ../node_modules/.bin/openapi-typescript ../../bin/openapi/account.json -o src/lib/api/schema.d.ts && ../node_modules/.bin/prettier --write src/lib/api/schema.d.ts

clean: ## Remove build output and logs
	rm -rf bin .logs $(foreach app,$(APPS),web/$(app)/build web/$(app)/.svelte-kit)

rename: ## Rename the project — make rename name="Acme ID" slug=acme (each brand: internal/brand, web/*/src/lib/brand.ts)
	@[[ -n "$(name)" && -n "$(slug)" ]] || { echo 'usage: make rename name="Acme ID" slug=acme'; exit 1; }
	@scripts/rename.sh "$(name)" "$(slug)"

## Quality

fmt: ## Format every Go and app file in place
	gofmt -w $(GO_DIRS)
	cd web/packages/styles && bun run format
	@$(call each-app,bun run format)

lint: lint-go lint-web lint-api ## Check formatting, vet, staticcheck, lint and type-check, changing nothing

lint-go: ## Lint the Go code only
	@unformatted="$$(gofmt -l $(GO_DIRS))"; [[ -z $$unformatted ]] || { echo "not formatted (make fmt):"; echo "$$unformatted"; exit 1; }
	go vet $(PKGS)
	go tool staticcheck $(PKGS)

lint-web: ## Lint and type-check the apps and the shared styles: Prettier, ESLint, Stylelint, svelte-check
	cd web/packages/styles && bun run lint
	@$(call each-app,bun run lint && bun run check)

lint-api: ## Lint the API description with Spectral, bundled as it is served (into bin/openapi)
	go run ./api/openapi/bundle bin/openapi
	cd api/openapi && ./node_modules/.bin/spectral lint ../../bin/openapi/account.json ../../bin/openapi/admin.json --ruleset .spectral.yaml --fail-severity warn
	@cd web/console && ../node_modules/.bin/openapi-typescript ../../bin/openapi/admin.json 2>/dev/null | ../node_modules/.bin/prettier --stdin-filepath src/lib/api/schema.d.ts | diff -q - src/lib/api/schema.d.ts >/dev/null || { echo "web/console/src/lib/api/schema.d.ts is not what the description says: make generate"; exit 1; }
	@cd web/id && ../node_modules/.bin/openapi-typescript ../../bin/openapi/account.json 2>/dev/null | ../node_modules/.bin/prettier --stdin-filepath src/lib/api/schema.d.ts | diff -q - src/lib/api/schema.d.ts >/dev/null || { echo "web/id/src/lib/api/schema.d.ts is not what the description says: make generate"; exit 1; }

test: test-go test-web ## Run the Go and app unit tests

test-go: ## Run the Go unit tests only
	go test $(PKGS)

test-web: ## Run the apps' unit tests only
	@$(call each-app,bun run test)

test-integration: ## Run the TestLive… tests against Postgres and Redis (throwaway databases)
	@. scripts/lib.sh && load_env && \
		dsn="$${VARD_TEST_DB_DSN:-$${DB_URL:-$$VARD_DB_DSN}}" && \
		{ [[ -n "$$dsn" ]] || { echo "test-integration: no database to test against: set VARD_DB_DSN in .env, or VARD_TEST_DB_DSN" >&2; exit 1; }; } && \
		VARD_TEST_DB_DSN="$$dsn" \
		VARD_TEST_REDIS="$${VARD_TEST_REDIS:-$${VARD_REDIS_HOST:+$$VARD_REDIS_HOST:$${VARD_REDIS_PORT:-6379}}}" \
		go test -count=1 -parallel 8 -run Live ./internal/... ./tests/...

bench: ## Run the Go benchmarks
	go test -run '^$$' -bench . -benchmem $(PKGS)

# GO-2026-6909: crewjam/saml mishandles an assertion without its Issuer and
# a NameID outside SAML's namespace, and no release fixes it. Both need the
# provider's signature, and checkSAMLAssertions (internal/federation/sso_saml.go)
# refuses them before the library parses. Drop it when a release does.
VULN_IGNORE := GO-2026-6909

vuln: ## Report known vulnerabilities the Go code calls; fails on any not in VULN_IGNORE
	@VULN_IGNORE="$(VULN_IGNORE)" scripts/vuln.sh $(PKGS)

# Each advisory ignored is in build and lint tooling only, never in a built
# app (neither app's build output holds it), and matches only globs we write:
# - GHSA-g3hv-75wg-m249, minimatch: its fix (11.0.1) is outside the ^10 that
#   eslint and typescript-eslint ask for. Drop it when they move to 11.
# - GHSA-fc5q-39xc-vjpf, picomatch (vite, vitest, svelte-check, eslint): no
#   release fixes it (4.0.7 is the last). Drop it when one does.
AUDIT_IGNORE := --ignore=GHSA-g3hv-75wg-m249 --ignore=GHSA-fc5q-39xc-vjpf

audit: ## Report known vulnerabilities in the web workspace's packages: both apps and the shared styles (bun audit)
	cd web && bun audit $(AUDIT_IGNORE)

check: lint test ## Lint and unit-test everything: the check before a change is done

ci: check test-integration docker-config vuln audit ## Everything a CI run would do

## Database

migrate-up: ## Apply pending migrations
	go run ./cmd/vard migrate up

migrate-down: ## Roll back the newest migration
	go run ./cmd/vard migrate down

migrate-status: ## Show which migrations have run
	go run ./cmd/vard migrate status

migrate-new: ## Create a migration — make migrate-new name=add_x
	@[[ -n "$(name)" ]] || { echo "usage: make migrate-new name=add_x"; exit 1; }
	go run ./cmd/vard migrate new $(name)

db-create: ## Create the database in .env if it is missing
	@scripts/db.sh create

db-reset: ## Take every migration down and up again: a new installation (asks first)
	@scripts/db.sh reset

db-psql: ## Open psql on the database in .env
	@scripts/db.sh psql

## Docker (compose.yaml, with compose.env)

docker-up: ## Build and start the stack in the background, as compose.env says
	VERSION=$(VERSION) $(COMPOSE) build --pull
	VERSION=$(VERSION) $(COMPOSE) up -d --wait

docker-local: ## Start the stack on this machine — http://localhost:8080, :8081 (writes compose.env)
	@VERSION=$(VERSION) scripts/docker-local.sh

docker-env: ## Make compose.env for a server — make docker-env public=https://id.example.com admin=https://admin.example.com
	@[[ -n "$(public)" && -n "$(admin)" ]] || { echo 'usage: make docker-env public=https://id.example.com admin=https://admin.example.com'; exit 1; }
	@scripts/docker-env.sh "$(public)" "$(admin)"

db-backup: ## Dump the stack's database into backups/, keeping the newest 14 (KEEP=n)
	@scripts/db-backup.sh

docker-down: ## Stop the stack, keeping its data
	$(COMPOSE) down

docker-reset: ## Stop the stack and delete its data
	$(COMPOSE) down -v

docker-logs: ## Follow the logs — every service, or one: make docker-logs service=api
	$(COMPOSE) logs -f $(service)

docker-build: ## Build the images
	VERSION=$(VERSION) $(COMPOSE) build

docker-ps: ## Show the stack's containers and their health
	$(COMPOSE) ps

docker-config: ## Validate compose.yaml, with placeholders for what compose.env would say
	@PUBLIC_URL=http://localhost:8080 ADMIN_URL=http://localhost:8081 VARD_SECRET_KEY=x \
		POSTGRES_PASSWORD=x POSTGRES_APP_PASSWORD=x REDIS_PASSWORD=x docker compose -f compose.yaml --env-file /dev/null config -q && echo "compose.yaml is valid"
