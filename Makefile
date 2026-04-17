# Korva — Go workspace Makefile
# Requires Go 1.22+ and a working `go.work` at the repo root.

.PHONY: all build test lint clean tidy sync vault cli sentinel help

# ─── Defaults ─────────────────────────────────────────────────────────────────

all: sync build test

# ─── Workspace sync ──────────────────────────────────────────────────────────

sync:
	go work sync

# ─── Build ───────────────────────────────────────────────────────────────────

build:
	go build github.com/alcandev/korva/...

vault:
	go build -o bin/korva-vault github.com/alcandev/korva/vault/cmd/korva-vault

cli:
	go build -o bin/korva github.com/alcandev/korva/cli/cmd/korva

sentinel:
	go build -o bin/korva-sentinel github.com/alcandev/korva/sentinel/validator/cmd/korva-sentinel

# ─── Test ────────────────────────────────────────────────────────────────────

test:
	go test github.com/alcandev/korva/... -count=1 -timeout 120s

test-race:
	go test github.com/alcandev/korva/... -race -count=1 -timeout 120s

test-cover:
	go test github.com/alcandev/korva/... -count=1 -coverprofile=coverage.out -timeout 120s
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-vault:
	cd vault && go test ./... -race -count=1 -timeout 60s

test-cli:
	cd cli && go test ./... -race -count=1 -timeout 60s

# ─── Lint ────────────────────────────────────────────────────────────────────

lint:
	golangci-lint run --timeout=5m

lint-fix:
	golangci-lint run --fix --timeout=5m

# ─── Tidy ────────────────────────────────────────────────────────────────────

tidy:
	cd internal && go mod tidy
	cd vault && go mod tidy
	cd cli && go mod tidy
	cd sentinel/validator && go mod tidy
	go work sync

# ─── Beacon (React) ──────────────────────────────────────────────────────────

beacon-dev:
	cd beacon && npm run dev

beacon-build:
	cd beacon && npm ci && npm run build

# ─── Clean ───────────────────────────────────────────────────────────────────

clean:
	rm -rf bin/ coverage.out coverage.html

# ─── Help ────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "Korva — available make targets:"
	@echo ""
	@echo "  make build         Build all Go modules"
	@echo "  make vault         Build korva-vault binary  → bin/korva-vault"
	@echo "  make cli           Build korva CLI binary    → bin/korva"
	@echo "  make sentinel      Build korva-sentinel      → bin/korva-sentinel"
	@echo ""
	@echo "  make test          Run all tests (no race)"
	@echo "  make test-race     Run all tests with race detector"
	@echo "  make test-cover    Run tests + HTML coverage report"
	@echo "  make test-vault    Run vault tests only"
	@echo ""
	@echo "  make lint          Run golangci-lint"
	@echo "  make lint-fix      Run golangci-lint --fix"
	@echo ""
	@echo "  make tidy          Run go mod tidy on all modules + sync"
	@echo "  make sync          Run go work sync"
	@echo ""
	@echo "  make beacon-dev    Start Beacon dev server (port 5173)"
	@echo "  make beacon-build  Build Beacon for production"
	@echo ""
	@echo "  make clean         Remove bin/ and coverage files"
	@echo ""
