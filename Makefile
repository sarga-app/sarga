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

## ──────────────── targets ────────────────

help: ## Show this help
	@awk 'BEGIN { FS = ":.*?## "; printf "\n  Usage: make <target>\n\n  Targets:\n" } \
	      /^[a-zA-Z_-]+:.*?## / { printf "    \033[36m%-14s\033[0m %s\n", $$1, $$2 } \
	      END { print "" }' $(MAKEFILE_LIST)

SCALAR_VERSION := 1.35.5
SCALAR_BUNDLE  := internal/transport/routes/api/docs/scalar-standalone.js

vendor: $(SCALAR_BUNDLE) ## Fetch all vendored third-party assets

$(SCALAR_BUNDLE): ## Download the pinned Scalar API reference bundle
	@echo "fetching scalar@$(SCALAR_VERSION)"
	@curl -sSLf -o $@ "https://cdn.jsdelivr.net/npm/@scalar/api-reference@$(SCALAR_VERSION)/dist/browser/standalone.js"

web-build: ## Build the React frontend into internal/transport/routes/web/dist/
	@cd web && pnpm build

build: $(SCALAR_BUNDLE) web-build ## Build the sarga binary (version injected from git describe)
	@$(GO) build $(GOFLAGS) -o $(BIN) $(PKG)

run: build ## Build and run `sarga serve`
	@./$(BIN) serve

test: ## Run all tests with race detector
	@$(GO) test -race -count=1 ./...

fmt: ## Format code (gofmt + goimports)
	@$(GOIMPORTS) -w -local github.com/sarga-app/sarga .
	@gofmt -w .

fmt-check: ## Fail if formatting is needed
	@diff=$$(gofmt -l . ; $(GOIMPORTS) -l -local github.com/sarga-app/sarga .); \
	if [ -n "$$diff" ]; then \
	  echo "the following files need formatting (run 'make fmt'):"; \
	  echo "$$diff"; \
	  exit 1; \
	fi

vet: ## Run go vet
	@$(GO) vet ./...

lint: ## Run golangci-lint (install via `make tools`)
	@$(GOLANGCI) run ./...

vuln: ## Run govulncheck (install via `make tools`)
	@$(GOVULNCHECK) ./...

sqlc: ## Regenerate sqlc Go code from migrations + queries
	@$(SQLC) generate

sqlc-check: ## Fail if generated sqlc code is out of date
	@$(SQLC) diff

api-lint: ## Lint the OpenAPI spec with Redocly CLI (uses npx)
	@npx -y @redocly/cli@latest lint --config api/redocly.yaml

api-gen: ## Regenerate Go code from api/openapi.yaml (oapi-codegen)
	@$(GO) tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

api-check: ## Fail if api/gen is out of date with api/openapi.yaml
	@$(GO) tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
	@if ! git diff --quiet -- internal/transport/routes/api/gen; then \
	  echo "api gen out of date — run 'make api-gen' and commit the result"; \
	  git --no-pager diff --stat -- internal/transport/routes/api/gen; \
	  exit 1; \
	fi

tidy: ## Tidy go.mod / go.sum
	@$(GO) mod tidy

tidy-check: ## Fail if `go mod tidy` would change anything
	@cp go.mod go.mod.bak; cp go.sum go.sum.bak; \
	$(GO) mod tidy; \
	if ! diff -q go.mod go.mod.bak >/dev/null || ! diff -q go.sum go.sum.bak >/dev/null; then \
	  mv go.mod.bak go.mod; mv go.sum.bak go.sum; \
	  echo "go.mod / go.sum would change — run 'make tidy'"; \
	  exit 1; \
	fi; \
	rm go.mod.bak go.sum.bak

docker: ## Build the Docker image as sarga:dev
	@docker build --build-arg VERSION=$(VERSION) -t sarga:dev .

clean: ## Remove built binary
	@rm -f $(BIN)

tools: ## Install dev tools (golangci-lint, govulncheck, goimports)
	@$(GO) install golang.org/x/tools/cmd/goimports@latest
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "ensure $$($(GO) env GOPATH)/bin is on your PATH"

ci: tidy-check fmt-check vet lint test vuln sqlc-check api-check build ## Run the full CI bundle locally

.PHONY: help build web-build run test fmt fmt-check vet lint vuln tidy tidy-check sqlc sqlc-check api-lint api-gen api-check docker clean tools ci vendor
