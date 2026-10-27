# Dhow developer tasks. CI runs the same targets.
GO       ?= go
# Build tools with the toolchain pinned in go.mod so they understand the module's Go version.
export GOTOOLCHAIN := $(shell awk '/^toolchain/ {print $$2}' go.mod)
PKG      := ./...
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null)
LDFLAGS  := -s -w \
	-X github.com/Arab-OpenSource/dhow/internal/buildinfo.version=$(VERSION) \
	-X github.com/Arab-OpenSource/dhow/internal/buildinfo.commit=$(COMMIT)
GOLANGCI_LINT_VERSION := v2.15.0
GOVULNCHECK_VERSION   := v1.8.0
OAPI_CODEGEN_VERSION  := v2.8.1
SQLC_VERSION          := v1.31.1

.PHONY: build
build: ## Build bin/dhow-server
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/dhow-server ./apps/server/cmd/dhow-server

.PHONY: gen
gen: ## Regenerate code from the OpenAPI spec and SQL queries
	cd apps/server && $(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) \
		-config oapi-codegen.yaml ../../packages/openapi/openapi.yaml
	cd apps/server && $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

.PHONY: check
check: lint test vuln ## Everything CI checks for Go

.PHONY: fmt
fmt: ## Format Go code
	$(GO) run mvdan.cc/gofumpt@v0.11.0 -w .

.PHONY: lint
lint: ## Run golangci-lint
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

.PHONY: test
test: ## Run Go tests with the race detector
	$(GO) test -race -count=1 -coverprofile=coverage.out $(PKG)

.PHONY: vuln
vuln: ## Scan Go dependencies for known vulnerabilities
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) $(PKG)

.PHONY: tidy
tidy: ## Tidy go.mod
	$(GO) mod tidy
