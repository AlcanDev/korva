# Korva — Go workspace Makefile
# Requires Go 1.22+ and a working `go.work` at the repo root.

.PHONY: all build test lint clean tidy sync vault vault-full cli sentinel \
        licensing-server licensing-mock hive-mock keygen release-dry \
        release-patch release-minor release-major \
        docker docker-licensing completions help

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

# vault-full: build korva-vault with the Beacon SPA embedded inside the binary.
# Requires Node 18+ and npm. The resulting binary serves the Beacon at http://localhost:7437.
vault-full:
	@echo "Building Beacon..."
	cd beacon && npm ci --silent && npm run build
	@echo "Copying dist into vault/internal/ui/..."
	rm -rf vault/internal/ui/dist
	cp -r beacon/dist vault/internal/ui/dist
	@echo "Building korva-vault with embedded UI..."
	go build -tags embedui -o bin/korva-vault github.com/alcandev/korva/vault/cmd/korva-vault
	@echo "Done: bin/korva-vault ($(du -sh bin/korva-vault | cut -f1))"

cli:
	go build -o bin/korva github.com/alcandev/korva/cli/cmd/korva

sentinel:
	go build -o bin/korva-sentinel github.com/alcandev/korva/sentinel/validator/cmd/korva-sentinel

hive-mock:
	go run github.com/alcandev/korva/forge/hive-mock

licensing-mock:
	go run github.com/alcandev/korva/forge/licensing-mock

# licensing-server: start the production licensing server (requires env vars)
#   KORVA_LICENSING_PRIVATE_KEY_FILE=./priv.pem
#   KORVA_LICENSING_ADMIN_SECRET=change-me
licensing-server:
	go run github.com/alcandev/korva/forge/licensing-server

# keygen: generate a fresh RSA-4096 key pair for the licensing server.
# Output: priv.pem (server only) + pubkey.pem (copy to internal/license/keys/)
keygen:
	go run github.com/alcandev/korva/forge/licensing-server/keygen

# release-dry: validate the goreleaser config + build all platforms locally.
release-dry:
	goreleaser release --snapshot --clean

# ─── Release helpers ─────────────────────────────────────────────────────────
# These targets bump the version, create an annotated tag, and push it.
# The tag push triggers the release.yml workflow which runs GoReleaser.
#
# Usage:
#   make release-patch   → v0.1.0 → v0.1.1  (bug fixes)
#   make release-minor   → v0.1.0 → v0.2.0  (new features, no breaking changes)
#   make release-major   → v0.1.0 → v1.0.0  (breaking changes)
#
# Tip: for automated releases on every PR merge, let release-please handle
#      this — it analyses conventional commits and creates the Release PR.
#      These targets are for when you want to cut a release immediately.

_LATEST_TAG    := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
_VER_PARTS     := $(subst ., ,$(patsubst v%,%,$(_LATEST_TAG)))
_VER_MAJOR     := $(word 1,$(_VER_PARTS))
_VER_MINOR     := $(word 2,$(_VER_PARTS))
_VER_PATCH     := $(word 3,$(_VER_PARTS))

release-patch: test
	$(eval _NEXT := v$(_VER_MAJOR).$(_VER_MINOR).$(shell echo $$(($(_VER_PATCH)+1))))
	@echo "Tagging $(_LATEST_TAG) → $(_NEXT)"
	git tag -a $(_NEXT) -m "Release $(_NEXT)"
	git push origin $(_NEXT)
	@echo "Released $(_NEXT) — GoReleaser running at https://github.com/AlcanDev/korva/actions"

release-minor: test
	$(eval _NEXT := v$(_VER_MAJOR).$(shell echo $$(($(_VER_MINOR)+1))).0)
	@echo "Tagging $(_LATEST_TAG) → $(_NEXT)"
	git tag -a $(_NEXT) -m "Release $(_NEXT)"
	git push origin $(_NEXT)
	@echo "Released $(_NEXT) — GoReleaser running at https://github.com/AlcanDev/korva/actions"

release-major: test
	$(eval _NEXT := v$(shell echo $$(($(_VER_MAJOR)+1))).0.0)
	@echo "Tagging $(_LATEST_TAG) → $(_NEXT)"
	git tag -a $(_NEXT) -m "Release $(_NEXT)"
	git push origin $(_NEXT)
	@echo "Released $(_NEXT) — GoReleaser running at https://github.com/AlcanDev/korva/actions"

# completions: generate shell completion scripts for bash, zsh, and fish.
completions:
	@mkdir -p completions
	go run github.com/alcandev/korva/cli/cmd/korva completion bash  > completions/korva.bash
	go run github.com/alcandev/korva/cli/cmd/korva completion zsh   > completions/korva.zsh
	go run github.com/alcandev/korva/cli/cmd/korva completion fish  > completions/korva.fish
	@echo "Completions written to completions/"

# docker: build the korva-vault Docker image locally.
docker:
	docker build -t ghcr.io/alcandev/korva-vault:dev .

# docker-licensing: build the licensing server Docker image locally.
# Build context must be the repo root (go.work).
docker-licensing:
	docker build \
	  -f forge/licensing-server/Dockerfile \
	  -t ghcr.io/alcandev/korva-licensing:dev \
	  .

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
	@echo "  make vault-full    Build korva-vault with embedded Beacon UI (requires Node)"
	@echo "  make cli           Build korva CLI binary    → bin/korva"
	@echo "  make sentinel      Build korva-sentinel      → bin/korva-sentinel"
	@echo "  make completions   Generate shell completions (bash, zsh, fish) → completions/"
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
	@echo "  make hive-mock         Start Hive mock server (port 7438)"
	@echo "  make licensing-mock    Start licensing mock server (port 7439)"
	@echo "  make licensing-server  Start production licensing server (port 7440)"
	@echo "  make keygen            Generate RSA-4096 key pair for licensing"
	@echo "  make release-patch     Bump patch, tag, push → triggers GoReleaser"
	@echo "  make release-minor     Bump minor, tag, push → triggers GoReleaser"
	@echo "  make release-major     Bump major, tag, push → triggers GoReleaser"
	@echo "  make release-dry       Dry-run goreleaser (no publish)"
	@echo "  make docker            Build korva-vault Docker image locally"
	@echo "  make docker-licensing  Build korva-licensing Docker image locally"
	@echo ""
