SHELL       := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS   += --warn-undefined-variables --no-print-directory

BIN         := sarga
PKG         := ./cmd/sarga
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOFLAGS     := -trimpath -ldflags '$(LDFLAGS)'

GO          ?= go
GOLANGCI    ?= golangci-lint
GOVULNCHECK ?= govulncheck
GOIMPORTS   ?= goimports
SQLC        ?= sqlc

.DEFAULT_GOAL := help

SCALAR_VERSION := 1.35.5
SCALAR_BUNDLE  := internal/server/docs/scalar-standalone.js

# ── Help ──────────────────────────────────────────────────────────────────────

help: ## Show this help
	@awk ' \
	  BEGIN { FS = ":.*?## "; printf "\n  Usage: make <target>\n" } \
	  /^## ──/ { printf "\n  %s\n", substr($$0, 4) } \
	  /^[a-zA-Z_-]+:.*?## / { printf "    \033[36m%-16s\033[0m %s\n", $$1, $$2 } \
	  END { print "" }' $(MAKEFILE_LIST)

# ── Build ─────────────────────────────────────────────────────────────────────
## ── Build ─────────────────────────────────────────────────────────────────────

vendor: $(SCALAR_BUNDLE) ## Fetch vendored third-party assets (Scalar API docs bundle)

$(SCALAR_BUNDLE):
	@echo "fetching scalar@$(SCALAR_VERSION)"
	@curl -sSLf -o $@ "https://cdn.jsdelivr.net/npm/@scalar/api-reference@$(SCALAR_VERSION)/dist/browser/standalone.js"

web-build: ## Build React frontend → internal/server/frontend/dist/ (embedded by internal/server/frontend/frontend.go)
	@cd web && pnpm install --frozen-lockfile && pnpm build

web-types: ## Regenerate web/src/types/api.d.ts from openapi/openapi.yaml
	@cd web && pnpm run types

build: $(SCALAR_BUNDLE) web-build ## Build the sarga binary (full: vendor + frontend + go)
	@$(GO) build $(GOFLAGS) -o $(BIN) $(PKG)

build-go: $(SCALAR_BUNDLE) ## Build Go binary only — skips frontend rebuild (fast, for dev)
	@$(GO) build $(GOFLAGS) -o $(BIN) $(PKG)

run: build-go ## Build Go binary and run sarga serve (frontend served by pnpm dev)
	@./$(BIN) serve

clean: ## Remove built binary
	@rm -f $(BIN)

# ── Go quality ────────────────────────────────────────────────────────────────
## ── Go quality ────────────────────────────────────────────────────────────────

fmt: ## Format Go files (gofmt + goimports)
	@$(GOIMPORTS) -w -local github.com/sarga-app/sarga .
	@gofmt -w .

fmt-check: ## CI: fail if any Go file needs formatting
	@diff=$$(gofmt -l . ; $(GOIMPORTS) -l -local github.com/sarga-app/sarga .); \
	if [ -n "$$diff" ]; then \
	  echo "the following files need formatting (run 'make fmt'):"; \
	  echo "$$diff"; \
	  exit 1; \
	fi

vet: ## Run go vet ./...
	@mkdir -p internal/server/frontend/dist && touch internal/server/frontend/dist/.gitkeep
	@$(GO) vet ./...

lint: ## Run golangci-lint (~25 linters)
	@$(GOLANGCI) run ./...

tidy: ## Tidy go.mod / go.sum
	@$(GO) mod tidy

tidy-check: ## CI: fail if go.mod / go.sum are not tidy
	@cp go.mod go.mod.bak; cp go.sum go.sum.bak; \
	$(GO) mod tidy; \
	if ! diff -q go.mod go.mod.bak >/dev/null || ! diff -q go.sum go.sum.bak >/dev/null; then \
	  mv go.mod.bak go.mod; mv go.sum.bak go.sum; \
	  echo "go.mod / go.sum would change — run 'make tidy'"; \
	  exit 1; \
	fi; \
	rm go.mod.bak go.sum.bak

vuln: ## Run govulncheck (CVE scan on dependencies)
	@$(GOVULNCHECK) ./...

# ── Testing ───────────────────────────────────────────────────────────────────
## ── Testing ───────────────────────────────────────────────────────────────────

test: ## Run all Go tests with race detector
	@$(GO) test -race -count=1 ./...

# ── Code generation ───────────────────────────────────────────────────────────
## ── Code generation ───────────────────────────────────────────────────────────

api-gen: ## Regenerate openapi/ from openapi/openapi.yaml
	@$(GO) tool oapi-codegen -config openapi/oapi-codegen.yaml openapi/openapi.yaml

api-check: ## CI: fail if openapi is out of date with openapi/openapi.yaml
	@$(GO) tool oapi-codegen -config openapi/oapi-codegen.yaml openapi/openapi.yaml
	@if ! git diff --quiet -- openapi; then \
	  echo "api gen out of date — run 'make api-gen' and commit the result"; \
	  git --no-pager diff --stat -- openapi; \
	  exit 1; \
	fi

api-lint: ## Lint openapi/openapi.yaml with Redocly CLI
	@npx -y @redocly/cli@latest lint --config openapi/redocly.yaml

sqlc: ## Regenerate internal/db/ from SQL queries + migrations
	@$(SQLC) generate

sqlc-check: ## CI: fail if sqlc output is out of date
	@$(SQLC) diff

# ── Docker & CI ───────────────────────────────────────────────────────────────
## ── Docker & CI ───────────────────────────────────────────────────────────────

docker: ## Build Docker image tagged sarga:dev
	@docker build -f deploy/Dockerfile --build-arg VERSION=$(VERSION) -t sarga:dev .

smoke: build-go ## CI: build a binary and verify it boots (`version`, `--help`, `healthcheck`)
	@./$(BIN) version >/dev/null
	@./$(BIN) --help >/dev/null
	@! ./$(BIN) healthcheck --addr 127.0.0.1:1 --timeout 1s >/dev/null 2>&1 || \
	  { echo "healthcheck should fail dialling a closed port"; exit 1; }

ci: tidy-check sqlc-check api-check api-lint fmt-check vet lint test vuln build smoke ## Run the full CI suite locally (mirrors .github/workflows/ci.yml)

# ── Setup ─────────────────────────────────────────────────────────────────────
## ── Setup ─────────────────────────────────────────────────────────────────────

tools: ## Install Go dev tools (golangci-lint, govulncheck, goimports)
	@$(GO) install golang.org/x/tools/cmd/goimports@latest
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "ensure $$($(GO) env GOPATH)/bin is on your PATH"

.PHONY: help vendor web-build web-types build run clean \
        fmt fmt-check vet lint tidy tidy-check vuln \
        test \
        api-gen api-check api-lint sqlc sqlc-check \
        docker ci smoke \
        tools
