# Architecture

> A tour of how Korva fits together — every component, every interface, every data flow.

*Last updated: 2026-04-30*

---

## High-level

Korva is a **monorepo of Go modules** linked by a `go.work` file plus a React SPA (Beacon) that ships embedded in the vault binary.

```
korva/
├── internal/        Shared Go packages — db, config, privacy, license, identity, profile
├── vault/           Local memory + MCP server (korva-vault binary)
├── cli/             User-facing CLI (korva binary)
├── sentinel/        Pre-commit architecture validator (korva-sentinel binary)
├── beacon/          React 19 + Vite 6 dashboard, embedded into korva-vault via go:embed
├── lore/            Curated knowledge scrolls (Markdown with YAML frontmatter)
├── forge/           SDD workflow templates + licensing/hive servers (private)
└── integrations/    Editor manifests (Claude Code, Cursor, VS Code, etc.)
```

Each Go module has its own `go.mod`. `go.work` ties them together so a single `go build ./...` from the root builds everything.

---

## Components in detail

### Vault (`vault/`)

The memory + knowledge runtime. One binary (`korva-vault`) that runs in three modes:

| Mode | Purpose |
|------|---------|
| `mcp` | JSON-RPC over stdio — what your AI assistant talks to |
| `http` | REST API + Beacon SPA on `:7437` — what your browser/CLI/Hive worker talks to |
| `both` | Both at once (default for desktop installs) |
| `tui` | Terminal UI (interactive vault browser) |

Inside the binary:

```
vault/
├── cmd/korva-vault/        Entry point — boots store, hive, retention, license, mcp/http
├── internal/store/         SQLite store with split read/write pools and a write queue
├── internal/api/           HTTP REST router (admin, license, hive, lore, project)
├── internal/mcp/           MCP JSON-RPC dispatch + 16+ tool handlers
├── internal/email/         Email delivery (Resend) for license events
├── internal/tui/           Terminal UI (Bubbletea)
└── internal/ui/            Beacon SPA embedding (//go:embed all:dist)
```

**Key design choices:**
- **Pure-Go SQLite** (`modernc.org/sqlite`) — no CGo, single static binary, trivial cross-compile
- **WAL mode** + dual connection pools — many parallel readers, one serialised writer
- **Application-level write queue** — concurrent agents calling `vault_save` don't trip over `SQLITE_BUSY`
- **MCP and HTTP share the same store** — one source of truth

→ See [`sqlite-concurrency`](../lore/curated/sqlite-concurrency/SCROLL.md) for the data-layer rationale.

### CLI (`cli/`)

The user's entry point. Cobra-based with these top-level commands:

| Command | What it does |
|---------|--------------|
| `init` | Bootstrap `~/.korva/` and start the vault |
| `setup <editor>` | Wire one editor for the current project |
| `status` | Show running services, license, last sync |
| `doctor` | Health-check + diagnose common problems |
| `sync` | Force a Hive sync now |
| `lore` | Manage scrolls (list, add, info) |
| `sentinel` | Run/install the architecture validator |
| `admin` | Admin commands (server-side only) |
| `hive` | Hive sync admin (status, dry-run, allow-list) |
| `license` | Activate / deactivate / status |
| `teams` | Teams admin (members, RBAC) |
| `auth` | Authentication helpers |
| `vault` | Direct vault CRUD (save, search, context) |
| `update` | Self-update binary |
| `obs` | Observability / logs / metrics |
| `skills` | Smart Skill Loader (Teams+) |

Most commands are thin wrappers over the vault HTTP API. `korva` is to `korva-vault` what `kubectl` is to a Kubernetes API server.

### Sentinel (`sentinel/`)

A pre-commit architecture validator. Reads files from stdin (the standard pre-commit hook protocol), parses them, and validates against rules in `sentinel/rules/*.yml`.

Rules look like:

```yaml
- id: no-direct-axios
  match:
    files: ["**/*.ts"]
    contains: ["from 'axios'"]
  message: "Use the team's typed HTTP client, not axios directly."
  severity: error
```

Output is JSON for tooling, plain text for humans. CI gates and IDE plugins can both consume it.

### Beacon (`beacon/`)

A React 19 + Vite 6 SPA. Two ways to run:

1. **Development**: `make beacon-dev` runs Vite on `:5173` with HMR; the vault on `:7437` redirects unknown paths there.
2. **Production**: `make vault-full` builds the SPA into `vault/internal/ui/dist/`, then compiles `korva-vault` with `-tags embedui` so the binary serves the dashboard from `//go:embed`.

The SPA never talks to the cloud. Every API call goes to `localhost:7437/vault-api/*`.

### Lore (`lore/`)

Two trees:

```
lore/
├── curated/        Public scrolls — shipped in the binary, MIT licensed
└── private/        Per-team scrolls — gitignored, lives only on the user's machine
```

Each scroll is a Markdown file with YAML frontmatter:

```yaml
---
id: my-skill
version: 1.0.0
team: backend
stack: NestJS, Postgres
last_updated: 2026-04-30
---

# Scroll: ...

## Triggers — load when:
- Files: ...
- Keywords: ...
- Tasks: ...

## Context, Rules, Anti-Patterns
...
```

The vault loads scrolls into the AI session when **any trigger** matches the current task.

→ See [`skill-authoring`](../lore/curated/skill-authoring/SCROLL.md) for the full authoring guide.

### Forge (`forge/`)

Templates for the 5-phase Spec-Driven Development workflow:

1. **Discovery** — what's the actual problem?
2. **Specification** — what shape does the solution take?
3. **Plan** — break into reviewable units
4. **Implementation** — code under the plan
5. **Review** — check against the spec

Phase state is persisted in the vault, so a partial workflow survives context resets.

`forge/` also contains private servers (licensing, hive-mock) that are gitignored — they live in a separate private repo and ship as separate Docker images.

### Hive — optional cross-team sync

When enabled, an outbox-pattern worker picks up local observations, runs them through the privacy filter, and pushes content-addressed chunks to a Hive server. Other teammates pull the same chunks back.

→ See [`cloud-sync`](../lore/curated/cloud-sync/SCROLL.md) for the protocol details.

---

## Data flow — saving an observation

```
┌────────────────┐   MCP / vault_save    ┌─────────────┐   tx     ┌──────────┐
│ AI assistant   │──────────────────────►│ vault MCP   │─────────►│ SQLite   │
└────────────────┘                       │ dispatch    │          │ (local)  │
                                         └──────┬──────┘          └──────────┘
                                                │
                                                ├──► privacy filter (redact)
                                                │
                                                ├──► outbox (if Hive enabled)
                                                │       │
                                                │       ▼
                                                │   ┌─────────────┐
                                                │   │ Hive worker │
                                                │   │ (background)│
                                                │   └──────┬──────┘
                                                │          │ HTTPS + canonical hash
                                                │          ▼
                                                │   ┌─────────────┐
                                                │   │ Hive server │
                                                │   └─────────────┘
                                                │
                                                └──► event to /v1/events SSE (Beacon UI live update)
```

Every step is auditable. The privacy filter runs **before** anything reaches storage or the network.

---

## Data flow — loading a session

```
┌────────────────┐  MCP / vault_context  ┌─────────────┐
│ AI assistant   │──────────────────────►│ vault MCP   │
└────────────────┘                       │ dispatch    │
                                         └──────┬──────┘
                                                │
                                                ├──► load recent observations (project-scoped)
                                                │
                                                ├──► run smart-skill matcher (Teams+)
                                                │       └──► rank scrolls by trigger match
                                                │
                                                ├──► inject auto_skills array into context
                                                │
                                                └──► return structured context to assistant
```

The assistant receives:
- A summary of recent decisions / patterns / bugs
- A list of scrolls auto-loaded for this task (with full content)
- The current SDD phase and project conventions

It now has the same situational awareness as a teammate who's been on the project for months.

---

## Process boundaries

| Process | Owner | Lifetime |
|---------|-------|----------|
| `korva-vault --mode both` | the user | long-running, started by `korva init` |
| `korva` (CLI invocations) | the user | short-lived, one command per call |
| AI assistant MCP child | the editor | lives as long as the editor's chat session |
| `korva-sentinel` | git hook | runs once per commit |

The vault is the only long-running process. Everything else is short-lived and stateless.

---

## File / directory layout (`~/.korva/`)

```
~/.korva/
├── config.json              Platform config (Hive endpoint, retention, etc.)
├── admin.key                Admin secret — read-only, mode 0600
├── install.id               Stable per-machine identifier
├── hive.key                 Optional: Hive client key
├── license.jws              Activated license (Teams / Business)
├── license.state            Last heartbeat timestamp
├── version.check            Last update-check cache (24h TTL)
└── vault/
    └── observations.db      SQLite database (the actual memory)
```

Everything is local. Nothing leaves the machine without an explicit Hive sync configuration.

---

## Build & release

| Step | Tool |
|------|------|
| Conventional Commits → version bump + changelog | `release-please` |
| Build artifacts (5 platforms × 3 binaries) | `goreleaser` |
| Embedded Beacon SPA | Node 22 build → `//go:embed all:dist` + `-tags embedui` |
| Homebrew formula update | `goreleaser` → `homebrew-tap` repo |
| Container image | multi-stage Dockerfile (Node → Go → Alpine) |
| Self-update | `korva update` — SHA256-verified, atomic binary swap |

Two GitHub Actions workflows:

| Workflow | Trigger | Job |
|----------|---------|-----|
| `ci.yml` | push / PR to main | build + test + lint on Linux/macOS/Windows + Beacon + gitleaks + sentinel |
| `release.yml` | `v*` tag | wait-for-CI → goreleaser → publish |
| `release-please.yml` | push to main | accumulate commits → open Release PR |

→ See [`release-engineering`](../lore/curated/release-engineering/SCROLL.md) for the rationale.

---

## Where to read code first

| Curiosity | Start here |
|-----------|-----------|
| How does the vault talk to my editor? | [`vault/internal/mcp/server.go`](../vault/internal/mcp/server.go) |
| How does the vault store data? | [`vault/internal/store/store.go`](../vault/internal/store/store.go) |
| How is the privacy filter implemented? | [`internal/privacy/`](../internal/privacy/) |
| How does the CLI launch the vault? | [`cli/internal/cmd/init.go`](../cli/internal/cmd/) |
| How are scrolls loaded? | [`vault/internal/store/skill_matcher.go`](../vault/internal/store/) |
| How is the binary built for release? | [`.goreleaser.yaml`](../.goreleaser.yaml) |
