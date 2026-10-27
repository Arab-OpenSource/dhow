GO       ?= go
export GOTOOLCHAIN := $(shell awk '/^toolchain/ {print $$2}' go.mod)
PKG      := ./...
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null)
LDFLAGS  := -s -w \
	-X github.com/Arab-OpenSource/dhow/internal/buildinfo.version=$(VERSION) \
	-X github.com/Arab-OpenSource/dhow/internal/buildinfo.commit=$(COMMIT)
GOLANGCI_LINT_VERSION := v2.15.0
GOVULNCHECK_VERSION   := v1.8.0
ACTIONLINT_VERSION    := v1.7.12
OAPI_CODEGEN_VERSION  := v2.8.1
SQLC_VERSION          := v1.31.1
WEB_DIST              := apps/server/internal/App/WebDist

.PHONY: build
build: ## Build bin/dhow-server
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/dhow-server ./apps/server/cmd/dhow-server

.PHONY: web
web: ## Build the web app and copy it into the server embed directory
	pnpm -C apps/web install --frozen-lockfile
	pnpm -C apps/web build
	find $(WEB_DIST) -mindepth 1 ! -name .keep -delete
	cp -R apps/web/dist/. $(WEB_DIST)/

.PHONY: release
release: ## Build bin/dhow-server with the web app embedded
	$(MAKE) web
	$(MAKE) build

.PHONY: gen
gen: ## Regenerate code from the OpenAPI spec (one config per module) and SQL queries
	set -e; for cfg in apps/server/internal/Modules/*/oapi-codegen.yaml; do \
		$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) \
			-config $$cfg packages/openapi/openapi.yaml; \
	done
	cd apps/server && $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

.PHONY: check
check: lint lint-actions test vuln hooks-test ## Everything CI checks

.PHONY: fmt
fmt: ## Format Go code
	$(GO) run mvdan.cc/gofumpt@v0.11.0 -w .

.PHONY: lint
lint: ## Run golangci-lint
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

.PHONY: lint-actions
lint-actions: ## Lint GitHub Actions workflows
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

.PHONY: hooks-test
hooks-test: ## Test the Claude Code project hooks
	python3 -B .claude/hooks/tests/test_hooks.py

.PHONY: test
test: ## Run Go tests with the race detector
	$(GO) test -race -count=1 -coverprofile=coverage.out $(PKG)

.PHONY: vuln
vuln: ## Scan Go dependencies for known vulnerabilities
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) $(PKG)

.PHONY: tidy
tidy: ## Tidy go.mod
	$(GO) mod tidy
