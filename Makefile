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
PROTOBUF_VERSION      := v1.36.12
CONNECT_VERSION       := v1.21.0
BUF_VERSION           := v1.73.0
BUF                   := $(GO) run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
TOOLS_DIR             := bin/tools
PROTO_DIR             := packages/proto
PROTO_AGAINST_BRANCH  ?= main
PROTO_AGAINST         ?= $(shell git rev-parse --git-common-dir 2>/dev/null)\#branch=$(PROTO_AGAINST_BRANCH),subdir=$(PROTO_DIR)
WEB_DIST              := apps/server/internal/App/WebDist
IMAGE                 ?= dhow:dev
E2E_BASE              ?= debian:12
IMAGE_DOCKERFILE      ?= deploy/docker/Dockerfile
DOCKER_BUILD_FLAGS    ?=
COMPOSE_DIR           := deploy/compose
COMPOSE               := docker compose --project-directory $(COMPOSE_DIR) -f $(COMPOSE_DIR)/compose.yaml

.PHONY: build
build: ## Build bin/dhow-server
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/dhow-server ./apps/server/cmd/dhow-server

.PHONY: build-runner
build-runner: ## Build bin/dhow-runner (Linux only)
	CGO_ENABLED=0 GOOS=linux $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/dhow-runner ./apps/runner/cmd/dhow-runner

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

.PHONY: dist
dist: ## Build reproducible release archives and SHA256SUMS in dist/ (run make web first)
	deploy/release/dist.sh $(VERSION) $(COMMIT)

.PHONY: installer-e2e
installer-e2e: build ## Run the installer end to end in a systemd container (E2E_BASE=debian:12)
	deploy/install/e2e/run.sh $(E2E_BASE) bin/dhow-server

.PHONY: image
image: ## Build the container image (IMAGE=dhow:dev)
	docker buildx build --load $(DOCKER_BUILD_FLAGS) -f $(IMAGE_DOCKERFILE) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t $(IMAGE) .

.PHONY: compose-up
compose-up: ## Start the Compose stack with the local image (run deploy/compose/init.sh first)
	@test -f $(COMPOSE_DIR)/.env || { echo "$(COMPOSE_DIR)/.env is missing: run $(COMPOSE_DIR)/init.sh first" >&2; exit 1; }
	DHOW_IMAGE=$(IMAGE) $(COMPOSE) up --detach --wait

.PHONY: compose-down
compose-down: ## Stop the Compose stack and keep its data (COMPOSE_DOWN_FLAGS=--volumes deletes it)
	DHOW_IMAGE=$(IMAGE) $(COMPOSE) down $(COMPOSE_DOWN_FLAGS)

.PHONY: smoke
smoke: image ## Build the image, start a fresh Compose stack, check it, and remove it
	deploy/smoke.sh $(IMAGE)

.PHONY: proxy-check
proxy-check: image ## Build the image and test the documented nginx, Caddy and Traefik configs in Docker (local only)
	deploy/proxy-check/run.sh $(IMAGE)

.PHONY: tools
tools: ## Build the pinned protoc plugins into bin/tools
	GOBIN=$(abspath $(TOOLS_DIR)) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOBUF_VERSION)
	GOBIN=$(abspath $(TOOLS_DIR)) $(GO) install connectrpc.com/connect/cmd/protoc-gen-connect-go@$(CONNECT_VERSION)

.PHONY: gen
gen: tools ## Regenerate code from the OpenAPI spec (one config per module), SQL queries and the runner protocol
	set -e; for cfg in apps/server/internal/Modules/*/oapi-codegen.yaml; do \
		$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) \
			-config $$cfg packages/openapi/openapi.yaml; \
	done
	cd apps/server && $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate
	$(BUF) generate $(PROTO_DIR) --template $(PROTO_DIR)/buf.gen.yaml

.PHONY: proto-lint
proto-lint: ## Lint the runner protocol and check it for breaking changes against main
	$(BUF) lint $(PROTO_DIR)
	@git rev-parse --verify --quiet refs/heads/$(PROTO_AGAINST_BRANCH) >/dev/null || \
		{ echo "proto-lint: branch $(PROTO_AGAINST_BRANCH) is missing: git fetch origin $(PROTO_AGAINST_BRANCH):$(PROTO_AGAINST_BRANCH)" >&2; exit 1; }
	@if git cat-file -e $(PROTO_AGAINST_BRANCH):$(PROTO_DIR)/buf.yaml 2>/dev/null; then \
		echo "$(BUF) breaking $(PROTO_DIR) --against '$(PROTO_AGAINST)'"; \
		$(BUF) breaking $(PROTO_DIR) --against '$(PROTO_AGAINST)'; \
	else \
		echo "proto-lint: $(PROTO_AGAINST_BRANCH) has no $(PROTO_DIR) yet, so there is nothing to break"; \
	fi

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

.PHONY: check
check: lint lint-runner lint-actions proto-lint test test-runner vuln hooks-test ## Everything CI checks

.PHONY: fmt
fmt: ## Format Go code
	$(GO) run mvdan.cc/gofumpt@v0.11.0 -w .

.PHONY: lint
lint: ## Run golangci-lint
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

.PHONY: lint-runner
lint-runner: ## Run golangci-lint on the runner with the fake driver build tag
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --build-tags dhowtest ./apps/runner/...

.PHONY: lint-actions
lint-actions: ## Lint GitHub Actions workflows
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

.PHONY: hooks-test
hooks-test: ## Test the Claude Code project hooks
	python3 -B -m unittest discover -s .claude/hooks/tests -p "test_*.py"

.PHONY: test
test: ## Run Go tests with the race detector
	$(GO) test -race -count=1 -coverprofile=coverage.out $(PKG)

FUZZTIME     ?= 10s
FUZZ_TARGETS := \
	Core/Auth:FuzzHashSessionToken Core/Auth:FuzzAPITokenHash Core/Auth:FuzzAuthorizationHeader Core/Auth:FuzzTotp \
	Core/Auth:FuzzRecoveryCode Core/Auth:FuzzEmailAndName Core/Auth:FuzzPasswordLength Core/Auth:FuzzWebAuthnResponses \
	Core/I18n:FuzzParseAcceptLanguage Core/Middleware:FuzzClientIP Core/RateLimit:FuzzNetwork \
	Core/Oidc:FuzzEmailVerifiedClaim Core/Oidc:FuzzAcceptedEmail

.PHONY: fuzz
fuzz: ## Fuzz the auth inputs, FUZZTIME per target (default 10s; the M2.5 bar is 60s)
	@found=$$(grep -rhoE '^func Fuzz[A-Za-z0-9]+' apps/server/internal --include='*_test.go' | sed 's/^func //' | sort); \
	listed=$$(printf '%s\n' $(FUZZ_TARGETS) | sed 's/.*://' | sort); \
	test "$$found" = "$$listed" || { echo "fuzz: FUZZ_TARGETS does not match the Fuzz functions in the code" >&2; exit 1; }
	set -e; for t in $(FUZZ_TARGETS); do \
		echo "fuzz $${t#*:} for $(FUZZTIME)"; \
		$(GO) test -run '^$$' -fuzz "^$${t#*:}$$" -fuzztime $(FUZZTIME) ./apps/server/internal/$${t%%:*}; \
	done

.PHONY: timing
timing: ## Run the statistical timing tests against DHOW_TEST_DATABASE_URL (slow, separate CI job)
	$(GO) test -tags timing -count=1 -run '^TestTiming' -timeout 30m -v ./apps/server/internal/App/Timing/...

.PHONY: test-runner
test-runner: ## Run the runner tests with the fake sandbox driver (dhowtest build tag)
	$(GO) test -race -count=1 -tags dhowtest ./apps/runner/...

RUNNER_FUZZ := Masking:FuzzMasker Masking:FuzzMaskerKeepsCleanText Logs:FuzzChunker Logs:FuzzSpillRecords \
	Journal:FuzzJournalOpen Executor:FuzzVerifyGrant Sandbox/Wire:FuzzDecode

.PHONY: fuzz-runner
fuzz-runner: ## Fuzz every runner target for FUZZTIME each (default 10s; the M3.5 bar is 60s)
	set -e; for t in $(RUNNER_FUZZ); do \
		$(GO) test -run '^$$' -fuzz "^$${t##*:}$$" -fuzztime $(FUZZTIME) ./apps/runner/internal/$${t%%:*}; \
	done

.PHONY: vuln
vuln: ## Scan Go dependencies for known vulnerabilities
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) $(PKG)

.PHONY: tidy
tidy: ## Tidy go.mod
	$(GO) mod tidy
