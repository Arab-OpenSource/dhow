# Dhow developer tasks. CI runs the same targets.
GO       ?= go
# Build tools with the toolchain pinned in go.mod so they understand the module's Go version.
export GOTOOLCHAIN := $(shell awk '/^toolchain/ {print $$2}' go.mod)
PKG      := ./...
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/Arab-OpenSource/dhow/internal/buildinfo.version=$(VERSION)
GOLANGCI_LINT_VERSION := v2.15.0
GOVULNCHECK_VERSION   := v1.8.0
ACTIONLINT_VERSION    := v1.7.12

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

.PHONY: check
check: lint lint-actions test vuln ## Everything CI checks

.PHONY: fmt
fmt: ## Format Go code
	$(GO) run mvdan.cc/gofumpt@v0.11.0 -w .

.PHONY: lint
lint: ## Run golangci-lint
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

.PHONY: lint-actions
lint-actions: ## Lint GitHub Actions workflows
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

.PHONY: test
test: ## Run Go tests with the race detector
	$(GO) test -race -count=1 -coverprofile=coverage.out $(PKG)

.PHONY: vuln
vuln: ## Scan Go dependencies for known vulnerabilities
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) $(PKG)

.PHONY: tidy
tidy: ## Tidy go.mod
	$(GO) mod tidy
