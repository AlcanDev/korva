# Phase 4 — Implementation: Observatory

> Log of what landed during the implementation phase. Pairs with
> [03-design.md](03-design.md) — every item below traces to the design's
> §8 implementation order.

## Backend (Go)

### Migrations

`internal/db/migrations.go` appended with:

- `interactions` table + 5 indexes + FTS5 virtual table + 3 sync triggers.
- `config_snapshots` table + 2 indexes.

Migrations are append-only and idempotent — existing installs upgrade
in place on next vault start.

### Store layer

| File | Surface |
|------|---------|
| `vault/internal/store/interactions.go` | `Interaction`, `InteractionFilters`, `TokenStats`, `TokenStatsBucket`, `DailyTokenCount` types · `SaveInteraction`, `GetInteraction`, `ListInteractions`, `CountInteractions`, `GetTokenStats`, `PurgeInteractionsOlderThan` |
| `vault/internal/store/config_snapshots.go` | `ConfigSnapshot` type · `SaveConfigSnapshot`, `ListConfigSnapshots`, `GetConfigSnapshot`, `LatestConfigSnapshot` |

Every prompt goes through `privacy.Filter` before the INSERT, with prompt and
response excerpts truncated to 8 KiB. When `usage` is missing, tokens default
to `len(prompt+response)/4` and the row is tagged `estimated=1`.

### IDE detector

`internal/detect/ide.go` — package-level `IDEs()` with 60s in-process cache.
Probes 7 candidates (Claude Code, Cursor, VS Code, JetBrains, Zed, Neovim,
Vim) by filesystem (OS-conventional config dirs) then PATH fallback. Detects
Korva-MCP wiring by parsing the IDE's settings/MCP JSON.

### Config writer

`internal/config/writer.go` — `Validate(cfg)`, `WriteAtomic(path, cfg, opts)`,
and `HashFile(path)`. Atomic write: validate → marshal → temp file with
unique ULID suffix → fsync → rename. Returns `ConflictError` when the on-disk
hash does not match `opts.ExpectedHash`.

### HTTP API

| Method | Path | File |
|--------|------|------|
| POST | `/api/v1/interactions` | `vault/internal/api/interactions_ingest.go` |
| GET | `/admin/activity` | `vault/internal/api/activity.go` |
| GET | `/admin/activity/{id}` | `vault/internal/api/activity.go` |
| GET | `/admin/tokens/stats` | `vault/internal/api/tokens.go` |
| GET | `/admin/system-status` | `vault/internal/api/system_status.go` |
| GET | `/admin/config` | `vault/internal/api/config.go` |
| PUT | `/admin/config` | `vault/internal/api/config.go` |
| GET | `/admin/config/snapshots` | `vault/internal/api/config.go` |
| POST | `/admin/vault/restart` | `vault/internal/api/restart.go` |
| GET | `/admin/sentinel/rules` | `vault/internal/api/sentinel_rules.go` |
| PUT | `/admin/sentinel/rules` | `vault/internal/api/sentinel_rules.go` |
| POST | `/admin/sentinel/test` | `vault/internal/api/sentinel_rules.go` |

`RouterConfig` in `vault/internal/api/router.go` extended with
`VaultStartedAt`, `VaultVersion`, `VaultPort`, `ConfigPathLocal`. Wired in
`vault/cmd/korva-vault/main.go`. The token stats baseline scans the dir
pointed to by `KORVA_BASELINE_DIR` (defaults to CWD), skipping `.git`,
`node_modules`, `dist`, `build`, `vendor`, etc.

### Sentinel YAML rules

`sentinel/validator/internal/rules/`:

- `custom.go` — `CustomRule` struct that implements the `Rule` interface, with
  doublestar-style glob matching (`**/`, `**`, `*`).
- `loader.go` — `LoadCustomRulesFile`, `LoadRulesFromYAML`, `SaveCustomRulesFile`.
- `cmd/korva-sentinel/main.go` — accepts `--rules` flag and respects
  `KORVA_SENTINEL_RULES` env var; merges YAML rules with the chosen profile.

The `SentinelConfig.RulesPath` field declared in the schema since v1 is now
honored end-to-end.

### Tests (Go)

| Package | Test files | Cases |
|---------|-----------|-------|
| `vault/internal/store` | `interactions_test.go`, `config_snapshots_test.go` | 19 |
| `vault/internal/api` | `interactions_ingest_test.go`, `system_status_test.go`, `config_test.go`, `sentinel_rules_test.go`, `test_helpers_test.go` | 26 |
| `internal/detect` | `ide_test.go` | 8 |
| `internal/config` | `writer_test.go` | 12 |
| `sentinel/validator/internal/rules` | `loader_test.go` | 12 |

77 new test cases. All pass under `go test github.com/alcandev/korva/...`.

## Frontend (Beacon)

### API client

`beacon/src/api/observatory.ts` — typed TanStack Query hooks for every endpoint:

- `useSystemStatus` (15s polling)
- `useConfig`, `useUpdateConfig`
- `useTokenStats`
- `useActivity`, `useInteraction`
- `useSentinelRules`, `useUpdateSentinelRules`, `useTestSentinelRule`
- `useRestartVault`

### Pages

`beacon/src/pages/observatory/`:

- `Observatory.tsx` — sub-router with 5 tabs.
- `SystemHealth.tsx` — 9 status cards + IDE list with Korva-MCP indicator.
- `TokenAnalytics.tsx` — 4 KPI cards, daily trend bar chart, by-model and
  by-project tables.
- `ActivityTimeline.tsx` — virtualized table with FTS search and detail drawer.
- `ConfigEditor.tsx` — 6 tabs (general, vault, lore, sentinel, hive, license)
  covering full CLI parity.
- `SentinelRulesEditor.tsx` — built-in rules list, custom rule editor cards,
  and a live test playground.
- `components/StatusCard.tsx` — shared status-card primitive.

Sidebar in `beacon/src/pages/admin/Admin.tsx` adds "Observatory" as the new
top nav entry, default route `/admin/observatory/health`.

## Smoke artifact

`scripts/smoke-observatory.sh` exercises every endpoint via curl + `jq`,
exits non-zero on any assertion failure. Reads `~/.korva/admin.key` by
default; respects `KORVA_ADMIN_KEY`, `HOST`, and `PORT` env vars.

## Build status

- `go build github.com/alcandev/korva/...` — clean.
- `go test github.com/alcandev/korva/...` — all packages pass.
- `cd beacon && npm run build` — TypeScript clean, Vite production bundle
  ~553 KB (gzipped 148 KB).
- `scripts/smoke-observatory.sh` — 14/14 assertions pass against a live vault.

## Outstanding

- Frontend Vitest coverage for the 5 new pages (planned in spec AC13). The
  Observatory pages are tightly coupled to TanStack Query; recommended next
  step is a small `__tests__/` folder per page using `@testing-library/react`
  and a mocked fetch adapter.
- Hot-reload of `korva.config.json` via `fsnotify` + SSE — explicitly
  out-of-scope for the MVP; users refresh manually.
- Cost USD per model — needs a maintained price table; deferred.
