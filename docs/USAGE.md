# Korva — Complete Usage Guide

This guide covers the full lifecycle: installation, configuration, daily use, team management, and advanced topics. Bookmark it — it is the single source of truth for working with Korva.

---

## Table of Contents

1. [Installation](#1-installation)
2. [First Run — `korva init`](#2-first-run--korva-init)
3. [Connecting Editors to the Vault MCP](#3-connecting-editors-to-the-vault-mcp)
4. [MCP Tools Reference](#4-mcp-tools-reference)
5. [Sentinel: Architecture Guardrails](#5-sentinel-architecture-guardrails)
6. [Team Profiles](#6-team-profiles)
7. [Daily Workflow with AI](#7-daily-workflow-with-ai)
8. [Vault HTTP API](#8-vault-http-api)
9. [Beacon Dashboard](#9-beacon-dashboard)
10. [Maintenance Commands](#10-maintenance-commands)
11. [Advanced Configuration](#11-advanced-configuration)
12. [Troubleshooting](#12-troubleshooting)
13. [Quick Reference Cheatsheet](#13-quick-reference-cheatsheet)

---

## 1. Installation

### macOS — Homebrew (recommended)

```bash
brew tap AlcanDev/tap
brew install korva
```

### macOS / Linux — curl installer

```bash
curl -fsSL https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.sh | sh
```

### Windows — PowerShell (run as Administrator)

```powershell
iwr -useb https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.ps1 | iex
```

> After the Windows installer finishes, **restart your terminal** so the PATH update takes effect.

### Verify the installation

```bash
korva version
# Expected output: 0.1.0 (abc1234) built 2026-04-15

korva-vault --help
korva-sentinel --help
```

All three binaries — `korva`, `korva-vault`, and `korva-sentinel` — should be on your PATH.

---

## 2. First Run — `korva init`

```bash
# Run from any directory — configures the global Korva environment
korva init
```

What this creates:

| Path (macOS/Linux) | Path (Windows) | Purpose |
|--------------------|----------------|---------|
| `~/.korva/config.json` | `%APPDATA%\korva\config.json` | Main configuration file |
| `~/.korva/vault/` | `%APPDATA%\korva\vault\` | Local SQLite memory store |
| `~/.korva/lore/` | `%APPDATA%\korva\lore\` | Knowledge scrolls |
| `~/.korva/profiles/` | `%APPDATA%\korva\profiles\` | Installed team profiles |
| `~/.korva/logs/` | `%APPDATA%\korva\logs\` | Operational logs |

### Initializing with a Team Profile

If your team maintains a private Korva profile repository, pass it at init time:

```bash
korva init --profile https://github.com/YOUR-ORG/korva-team-profile.git
```

### Admin flag

If you are the team administrator (the person who manages Sentinel rules and signs off on profile changes):

```bash
korva init --admin --owner=you@your-org.com
```

This generates `~/.korva/admin.key` with permissions `0600` — only your user can read it. This file **never leaves your machine** and is excluded from all sync operations. Guard it accordingly.

---

## 3. Connecting Editors to the Vault MCP

The Vault MCP server is the backbone of Korva — it gives your AI assistant persistent memory across sessions.

### Starting the server

```bash
# Recommended: run both MCP and HTTP modes together
korva-vault --mode=both --port=7437

# MCP only (for editors that manage their own lifecycle)
korva-vault --mode=mcp

# HTTP only (for the REST API and Beacon dashboard)
korva-vault --mode=http
```

The server listens on:
- `stdin/stdout` — MCP protocol (consumed by your editor's AI extension)
- `http://localhost:7437` — REST API and Beacon dashboard

### VS Code + GitHub Copilot

Create or edit `.vscode/mcp.json` in your project root:

```json
{
  "servers": {
    "korva-vault": {
      "type": "stdio",
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

Restart VS Code. Copilot Chat now has access to all `vault_*` tools.

To confirm it works, open Copilot Chat and type:

```
Use vault_stats to show me vault statistics.
```

### Claude Code

For global configuration, edit `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "korva-vault": {
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

For per-project configuration, create `.claude/settings.json` at the project root:

```json
{
  "mcpServers": {
    "korva-vault": {
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

### Cursor

Edit `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "korva-vault": {
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

Restart Cursor. The `vault_*` tools will appear in the tool list inside Agent mode.

---

## 4. MCP Tools Reference

These are the ten tools the Vault MCP exposes to your AI assistant. All tools are available in any editor that supports MCP.

---

### `vault_save`

Saves a new observation (decision, pattern, bug fix, learning, or general context) to the vault.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project` | string | yes | Project identifier (e.g., `my-project`) |
| `type` | string | yes | One of: `decision`, `pattern`, `bugfix`, `learning`, `context` |
| `title` | string | yes | Short, searchable title |
| `content` | string | yes | Full content of the observation |
| `tags` | string[] | no | Optional labels for filtering |

**Example:**

```
Use vault_save with:
- project: my-project
- type: decision
- title: Adopted hexagonal architecture
- content: We chose ports & adapters to separate domain logic from infrastructure. Domain layer has zero framework imports. Application layer depends only on port interfaces via DI tokens.
- tags: ["architecture", "adr"]
```

---

### `vault_search`

Full-text search across all saved observations.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Search terms |
| `project` | string | no | Scope search to a specific project |
| `type` | string | no | Filter by observation type |
| `limit` | integer | no | Max results to return (default: 10) |

**Example:**

```
Use vault_search with query "repository pattern" and project "my-project"
```

**Tip:** Prefix your query with `scroll:` to load a specific knowledge scroll:

```
Use vault_search with query "scroll:nestjs-hexagonal"
```

---

### `vault_context`

Retrieves the most recent and relevant context for a project — the recommended tool to call at the **start of every session**.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project` | string | yes | Project identifier |
| `limit` | integer | no | Number of recent observations to include (default: 20) |

**Example:**

```
Use vault_context with project "my-project" to restore my previous work context.
```

Returns a structured summary of recent decisions, active patterns, and the last session summary — everything needed to resume work without repeating yourself.

---

### `vault_timeline`

Returns observations ordered chronologically, useful for reviewing what changed over a time window.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project` | string | yes | Project identifier |
| `since` | string | no | ISO 8601 date — return observations after this date |
| `until` | string | no | ISO 8601 date — return observations before this date |
| `limit` | integer | no | Max results (default: 50) |

**Example:**

```
Use vault_timeline with project "my-project" and since "2026-04-01" to see what we decided this week.
```

---

### `vault_get`

Retrieves a single observation by its ULID.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | yes | ULID of the observation |

**Example:**

```
Use vault_get with id "01HZ8XKQNVG9F4Q3PXWSM2T7R"
```

Useful when you have a reference to a specific observation (e.g., from a `vault_search` result) and want to read its full content.

---

### `vault_session_start`

Marks the beginning of a working session. Records a timestamp and optional goal, enabling session-scoped queries later.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project` | string | yes | Project identifier |
| `goal` | string | no | What you intend to accomplish in this session |

**Example:**

```
Use vault_session_start with project "my-project" and goal "Implement the payment domain service and wire up the adapter"
```

---

### `vault_session_end`

Closes the current session and saves a summary of what was accomplished. Pair with `vault_session_start`.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project` | string | yes | Project identifier |
| `summary` | string | yes | What was done, decided, or learned in this session |

**Example:**

```
Use vault_session_end with:
- project: my-project
- summary: Implemented PaymentService with the Stripe adapter. Decided to use idempotency keys on all charge calls. Found and fixed a race condition in the refund flow — see observation 01HZ9...
```

---

### `vault_summary`

Generates a high-level summary of a project's vault — all decisions, key patterns, and recent activity condensed into a readable overview.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project` | string | yes | Project identifier |

**Example:**

```
Use vault_summary for project "my-project" to give me a project overview before the sprint planning.
```

---

### `vault_save_prompt`

Saves a reusable AI prompt (a template you want to reuse across sessions or share with teammates).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Short unique name for the prompt |
| `content` | string | yes | The full prompt text |
| `tags` | string[] | no | Labels for organization |

**Example:**

```
Use vault_save_prompt with:
- name: code-review-checklist
- content: Review the following code for: 1) layer boundary violations 2) missing error handling 3) untested edge cases 4) hardcoded values that should come from config
- tags: ["review", "quality"]
```

---

### `vault_stats`

Returns statistics about the vault: total observations, breakdown by type and project, storage size, and sync status.

**Parameters:** none

**Example:**

```
Use vault_stats to show me an overview of the vault.
```

This is also the fastest way to confirm the MCP connection is working — if it returns any data, the server is reachable.

---

## 5. Sentinel: Architecture Guardrails

Sentinel validates every commit against your team's architecture rules. Violations block the commit by default, keeping the codebase clean without relying on code review to catch structural problems.

### Installing hooks in a project

```bash
cd ~/repos/my-project
korva sentinel install
```

This installs `.git/hooks/pre-commit`, which runs `korva-sentinel --staged` automatically on every `git commit`.

### Running Sentinel manually

```bash
# Analyze only staged files (same as the hook)
korva-sentinel --staged

# Analyze an entire directory
korva-sentinel ./src/

# List all active rules
korva-sentinel --list-rules
```

### Built-in rules

| ID | Severity | Description |
|----|----------|-------------|
| `HEX-001` | Error | Infrastructure must not be imported from domain layer |
| `HEX-002` | Error | Application layer must not be imported from domain layer |
| `HEX-003` | Error | Infrastructure must not be imported from application layer |
| `HEX-004` | Error | Circular imports detected between layers |
| `HEX-005` | Error | External dependencies (npm/pip/etc.) used directly in domain |
| `NAM-001` | Warning | Application services must end in `UseCase` or `Service` |
| `NAM-002` | Warning | Repository implementations must end in `Repository` |
| `NAM-003` | Warning | HTTP controllers must end in `Controller` |
| `SEC-001` | Error | Hardcoded secrets detected (API keys, tokens, passwords) |
| `TEST-001` | Warning | `console.log` calls found in production source files |

### Adding team-specific rules

Team-specific rules live in your [Team Profile](#6-team-profiles) under `sentinel/`. Create a Markdown file following this structure:

```markdown
## TEAM-001: No direct database access outside repositories

**Severity:** Error
**Pattern:** `(knex|prisma|typeorm)\.(query|raw)\(`
**Files:** `src/application/**/*.ts`, `src/domain/**/*.ts`

Database access belongs exclusively in the infrastructure layer behind a repository interface.

// ❌ Wrong
const result = await this.knex('users').where({ id });

// ✅ Correct
const user = await this.userRepository.findById(id);
```

Then reference it in `team-profile.json`:

```json
"sentinel": {
  "rules_path": "sentinel/my-rules.md"
}
```

---

## 6. Team Profiles

A Team Profile is a private Git repository that extends Korva with your organization's specific knowledge, rules, and AI instructions — without modifying the public Korva codebase.

### What a profile contains

```
korva-team-profile/
├── team-profile.json              — Profile manifest and configuration overrides
├── scrolls/                       — Private knowledge bases for the AI
│   ├── architecture/              — ADRs, bounded contexts, design decisions
│   ├── dev-workflow/              — Branching strategy, review process, CI conventions
│   └── ...
├── instructions/
│   ├── copilot-extensions.md      — Injected into .github/copilot-instructions.md
│   └── claude-extensions.md      — Injected into CLAUDE.md
└── sentinel/
    └── team-rules.md              — Custom Sentinel rules for your stack
```

### Installing the team profile

```bash
korva init --profile https://github.com/YOUR-ORG/korva-team-profile.git
```

This will:
1. Clone the profile to `~/.korva/profiles/`
2. Validate `team-profile.json` against the Korva schema
3. Apply configuration overrides (vault sync URL, Sentinel rules path, active scrolls)
4. Copy private scrolls to `~/.korva/lore/private/`
5. Inject the instruction extensions into `.github/copilot-instructions.md` and `CLAUDE.md`

### Syncing updates

When the team profile is updated (new scrolls, updated rules, changed instructions):

```bash
korva sync --profile
```

This runs `git pull` on the installed profile and re-applies all configurations.

### Creating your own team profile

1. Fork or use [korva-team-profile](https://github.com/AlcanDev/korva-team-profile) as a starting point
2. Make the repository **private** in your organization
3. Customize `team-profile.json`, scrolls, and instructions for your stack
4. Share the repository URL with your team
5. Each developer runs `korva init --profile <your-url>`

### Role-based access

| Role | Repo access | Can install | Can edit scrolls |
|------|------------|-------------|-----------------|
| Tech Lead / Architect | Write | Yes | Yes |
| Developer | Read | Yes | No |
| QA / Designer | Read | Yes | No |

---

## 7. Daily Workflow with AI

Here is a realistic daily sequence showing how Korva integrates into your work.

### Morning: start a session

```
Use vault_context with project "my-project" to restore my previous context.
```

The AI will load recent decisions, active patterns, and the previous session summary. You are immediately in context — no re-explaining the architecture.

```
Use vault_session_start with project "my-project" and goal "Refactor the notification service to use the outbox pattern"
```

### During work: search before proposing

Before the AI proposes an implementation, prompt it to check existing knowledge:

```
Use vault_search with query "notification" and project "my-project" before proposing an approach.
```

This prevents the AI from suggesting something that conflicts with a prior decision.

### Saving important decisions

When something significant is decided:

```
Use vault_save with:
- project: my-project
- type: decision
- title: Outbox pattern for notification delivery
- content: We adopted the transactional outbox pattern for reliable notification delivery. Events are written atomically with the business transaction, then a background worker polls and dispatches them. This eliminates dual-write issues with the message broker.
- tags: ["notifications", "reliability", "pattern"]
```

### Saving a pattern that worked

```
Use vault_save with:
- project: my-project
- type: pattern
- title: Repository interface naming convention
- content: All repository interfaces are named IEntityRepository (e.g., IUserRepository) and live in the domain layer. Implementations in infrastructure are named EntityRepository (no prefix).
```

### Saving a bug fix

```
Use vault_save with:
- project: my-project
- type: bugfix
- title: Race condition in concurrent order creation
- content: Two concurrent requests could create duplicate orders because the uniqueness check and insert were not atomic. Fixed by using INSERT ... ON CONFLICT DO NOTHING with a unique constraint on (user_id, idempotency_key).
```

### End of day: close the session

```
Use vault_session_end with:
- project: my-project
- summary: Refactored NotificationService to outbox pattern. Created OutboxEntry domain entity and IOutboxRepository port. Implemented OutboxRepository in infrastructure using Prisma. Background worker polls every 5s. All tests passing. Tomorrow: wire up the dead-letter queue for failed dispatches.
```

Tomorrow, `vault_context` will pick up exactly where this summary left off.

### Observation types reference

| Type | When to use |
|------|-------------|
| `decision` | Architecture or technology decisions with lasting impact |
| `pattern` | Code patterns and conventions that worked well |
| `bugfix` | Bugs resolved, including root cause and fix approach |
| `learning` | Team learnings, gotchas, non-obvious behaviors |
| `context` | General project context (goals, constraints, stakeholders) |

---

## 8. Vault HTTP API

The REST API is available at `http://localhost:7437` while `korva-vault` is running. It is the same data store the MCP uses — you can script against it, integrate with CI pipelines, or build custom tooling.

### Health check

```bash
curl http://localhost:7437/healthz
# {"status":"ok","version":"0.1.0"}
```

### Save an observation

```bash
curl -X POST http://localhost:7437/api/v1/observations \
  -H "Content-Type: application/json" \
  -d '{
    "project": "my-project",
    "type": "decision",
    "title": "REST API versioning via URL path",
    "content": "All endpoints are versioned with /api/v1/ prefix. Breaking changes require a new version prefix, not changes to existing endpoints.",
    "tags": ["api", "versioning"]
  }'
```

### Search observations

```bash
curl "http://localhost:7437/api/v1/search?q=hexagonal&project=my-project"
curl "http://localhost:7437/api/v1/search?q=authentication&type=decision&limit=5"
```

### Get project context

```bash
curl http://localhost:7437/api/v1/context/my-project
```

### Get timeline

```bash
# All observations for a project, newest first
curl "http://localhost:7437/api/v1/timeline/my-project"

# Filtered by date range
curl "http://localhost:7437/api/v1/timeline/my-project?since=2026-04-01&until=2026-04-15"
```

### Get a single observation by ID

```bash
curl http://localhost:7437/api/v1/observations/01HZ8XKQNVG9F4Q3PXWSM2T7R
```

### Statistics

```bash
curl http://localhost:7437/api/v1/stats
```

### Admin endpoints (require `admin.key`)

```bash
# Read the admin key
KEY=$(cat ~/.korva/admin.key | jq -r .key)

# Full admin statistics
curl -H "X-Admin-Key: $KEY" http://localhost:7437/admin/stats

# List all projects
curl -H "X-Admin-Key: $KEY" http://localhost:7437/admin/projects

# Force a vault sync
curl -X POST -H "X-Admin-Key: $KEY" http://localhost:7437/admin/sync
```

---

## 9. Beacon Dashboard

Beacon is the web dashboard that visualizes your vault. It provides a searchable, browsable interface for all observations, timelines, and team statistics.

### Starting Beacon (development mode)

```bash
cd beacon
npm install
npm run dev
```

Open [http://localhost:5173](http://localhost:5173) in your browser.

> Beacon requires `korva-vault --mode=http` (or `--mode=both`) to be running on port `7437`.

### What Beacon shows

- **Overview** — total observations, breakdown by type and project
- **Timeline** — chronological view of all activity, filterable by project and date
- **Search** — full-text search with type and project filters
- **Sessions** — history of work sessions with their summaries
- **Scrolls** — browse installed knowledge scrolls

---

## 10. Maintenance Commands

### System health

```bash
# Quick status overview — vault running, profile installed, hooks active
korva status

# Full diagnostic — checks all components and reports issues
korva doctor
```

### Knowledge scrolls (Lore)

```bash
# List all available scrolls (public + private from team profile)
korva lore list

# Install a curated public scroll
korva lore add nestjs-hexagonal
korva lore add typescript
korva lore add docker-compose

# Remove a scroll
korva lore remove nestjs-hexagonal
```

### Sync operations

```bash
# Sync everything (vault + team profile)
korva sync

# Sync only the team profile (re-apply rules and scrolls)
korva sync --profile

# Sync only the vault with the remote server
korva sync --vault
```

### Vault server management

Use `korva vault` to manage the vault server process without opening a terminal.

```bash
# Start the vault server in the background (auto-detaches, writes ~/.korva/vault/vault.pid)
korva vault start

# Check if the vault is running and responsive
korva vault status

# Stop the vault server gracefully
korva vault stop

# Show the path to the vault log file (for tail -f, etc.)
korva vault logs
```

The `start` command:
1. Locates the `korva-vault` binary on your PATH
2. Opens `~/.korva/logs/vault.log` for stdout/stderr
3. Starts the process detached from the terminal (survives terminal close)
4. Polls `/healthz` up to 5 s to confirm the server is up
5. Writes the PID to `~/.korva/vault/vault.pid`

### License management (Korva for Teams)

```bash
# Activate a license key (requires internet — contacts licensing.korva.dev once)
korva license activate KORVA-XXXX-XXXX-XXXX-XXXX

# Show current license status, tier, features, and expiry
korva license status

# Deactivate this install and free the seat
korva license deactivate
```

After activation, the JWS is stored at `~/.korva/license.key`. Validation is fully offline — no network call needed on every vault start. A heartbeat is sent every 24 h; if the server is unreachable for more than 7 days, the install degrades to Community tier with a banner in Beacon.

### Admin operations

```bash
# Rotate the admin key (reads current key from stdin, never as an argument)
korva admin rotate-key

# Export vault data to a JSON file
korva admin export --output=vault-backup.json

# Show which team profile is installed
korva admin profile info
```

---

## 11. Advanced Configuration

### Main configuration file

`~/.korva/config.json` (macOS/Linux) or `%APPDATA%\korva\config.json` (Windows):

```json
{
  "vault": {
    "port": 7437,
    "sync_repo": "",
    "sync_branch": "main",
    "auto_sync": false,
    "sync_interval_minutes": 60,
    "private_patterns": [
      "password",
      "secret",
      "token",
      "Bearer ",
      "api_key",
      "apiKey"
    ]
  },
  "sentinel": {
    "rules_path": "",
    "block_on_violation": true,
    "ignored_paths": ["node_modules", "dist", ".next", "build"]
  },
  "lore": {
    "scroll_priority": "private_first",
    "active_scrolls": ["nestjs-hexagonal", "typescript"]
  }
}
```

**Key settings:**

- `vault.sync_repo` — URL of the remote server for team vault sync; leave empty for local-only mode
- `vault.auto_sync` — when `true`, syncs after every `vault_save` call
- `vault.private_patterns` — strings that trigger privacy filtering before saving; content matching these patterns is redacted
- `sentinel.block_on_violation` — set to `false` to make Sentinel warn-only (not recommended for teams)
- `lore.scroll_priority` — `private_first` loads team profile scrolls before public ones; use `public_first` to reverse

### Environment variables

Environment variables override the config file, which is useful in CI or container environments:

```bash
KORVA_HOME=/custom/path        # Override ~/.korva/ (useful for shared environments)
KORVA_VAULT_PORT=8080          # Override vault port (default: 7437)
KORVA_LOG_LEVEL=debug          # Log verbosity: debug | info | warn | error
KORVA_SYNC_REPO=https://...    # Override vault.sync_repo from config
KORVA_AUTO_SYNC=true           # Override vault.auto_sync from config
```

### Rotating the admin key

```bash
korva admin rotate-key
# Prompts for the current key via stdin
# Generates a new key and writes it to ~/.korva/admin.key
# The old key is immediately invalidated
```

The admin key is always read from `~/.korva/admin.key` at runtime — it is never passed as a CLI argument and never logged. Back it up securely before rotating.

### Team vault server

To share the vault across a team in real time, deploy `korva-vault` on a server (Railway, Fly.io, a VM, etc.) and point everyone's config at it:

```json
{
  "vault": {
    "sync_repo": "https://your-vault-server.example.com",
    "auto_sync": true,
    "sync_interval_minutes": 30
  }
}
```

After the `post-commit` hook is installed via `korva sentinel install`, the vault syncs silently after every commit. If the server is unreachable, the sync queues and retries on the next commit — it never blocks your workflow.

---

## 12. Troubleshooting

### `korva init` cannot clone the team profile

- Confirm you have access: check the repository settings in GitHub
- For HTTPS with a token: `korva init --profile https://YOUR_TOKEN@github.com/YOUR-ORG/korva-team-profile.git`
- For SSH: `korva init --profile git@github.com:YOUR-ORG/korva-team-profile.git`
- If behind a corporate proxy, set `HTTPS_PROXY` and `NO_PROXY` before running

### `korva-vault` is not found after installation

```bash
# macOS/Linux — check which shell is active
echo $SHELL
# For zsh:
echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.zshrc && source ~/.zshrc
# For bash:
echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.bashrc && source ~/.bashrc

# Windows — verify PATH in System Properties → Environment Variables
where korva-vault
```

### The Vault MCP does not appear in VS Code / Copilot

1. Restart VS Code after editing `.vscode/mcp.json` — the server list is not hot-reloaded
2. Confirm `korva-vault` is on PATH: `which korva-vault` (macOS/Linux) or `where korva-vault` (Windows)
3. Check the Output panel (View → Output → GitHub Copilot) for MCP connection errors
4. Try running `korva-vault --mode=mcp` manually in a terminal to see if it starts without errors

### The Vault MCP does not appear in Claude Code

```bash
# Verify the settings file is valid JSON
cat ~/.claude/settings.json | jq .

# Check that the MCP server is listed
cat ~/.claude/settings.json | jq '.mcpServers'

# Restart Claude Code after any change to settings.json
```

### `korva sentinel install` fails on Windows

- Run PowerShell **as Administrator** for the first install
- If Git is installed via Git for Windows, ensure `git` is on the system PATH, not just the user PATH
- Alternatively, use WSL2 and run the install from there

### Observations are not saved (privacy filter is too aggressive)

The privacy filter may be redacting content that contains words from `private_patterns`. Check your config:

```bash
cat ~/.korva/config.json | jq '.vault.private_patterns'
```

Remove any patterns that are too broad. Patterns are matched as plain substrings (case-sensitive).

### `korva sync --profile` does not pick up new scrolls

```bash
# Force a full re-application of the profile
korva doctor          # Check for errors first
korva sync --profile  # Re-pull and re-apply
korva lore list       # Verify scrolls are now listed
```

If scrolls still do not appear, check `~/.korva/logs/` for errors from the last sync operation.

### Vault data seems stale when using a team server

- Check `korva status` — it shows the last successful sync time
- Verify the sync URL is reachable: `curl https://your-vault-server.example.com/healthz`
- If `auto_sync` is `false`, run `korva sync --vault` manually

---

## 13. Quick Reference Cheatsheet

```
SETUP
─────────────────────────────────────────────────────
korva init                           Initialize Korva (local)
korva init --profile <url>           Initialize with a team profile
korva init --admin --owner=<email>   Initialize as admin (generates admin.key)

VAULT SERVER MANAGEMENT
─────────────────────────────────────────────────────
korva vault start                    Start vault server (background, auto-detach)
korva vault stop                     Stop vault server gracefully
korva vault status                   Check if vault is running + responsive
korva vault logs                     Print path to vault.log

korva-vault --mode=both              Start MCP + HTTP server (recommended)
korva-vault --mode=mcp               MCP only (for editor integration)
korva-vault --mode=http              HTTP only (for REST API / Beacon)
korva-vault --port=7437              Set HTTP port (default: 7437)

SENTINEL
─────────────────────────────────────────────────────
korva sentinel install               Install pre-commit hook in current project
korva-sentinel --staged              Analyze staged files (same as hook)
korva-sentinel ./src/                Analyze a directory
korva-sentinel --list-rules          Show all active rules

LORE (KNOWLEDGE SCROLLS)
─────────────────────────────────────────────────────
korva lore list                      List available scrolls
korva lore add <scroll-id>           Install a public scroll
korva lore remove <scroll-id>        Remove a scroll

MAINTENANCE
─────────────────────────────────────────────────────
korva status                         Quick system status
korva doctor                         Full diagnostic
korva sync                           Sync vault + profile
korva sync --profile                 Sync team profile only
korva sync --vault                   Sync vault with remote server

LICENSE (Korva for Teams)
─────────────────────────────────────────────────────
korva license activate <key>         Activate a Teams license key
korva license status                 Show tier, features, expiry, grace period
korva license deactivate             Free this install's seat

ADMIN
─────────────────────────────────────────────────────
korva admin rotate-key               Rotate the admin key (reads from stdin)
korva admin export --output=<file>   Export vault to JSON
korva admin profile info             Show installed profile details

MCP TOOLS (use in your AI assistant)
─────────────────────────────────────────────────────
vault_context    <project>                  Load session context (use at start)
vault_save       <project> <type> <title> <content>  Save an observation
vault_search     <query> [project] [type]   Search observations
vault_timeline   <project> [since] [until]  Chronological view
vault_get        <id>                        Get one observation by ULID
vault_session_start  <project> [goal]        Start a session
vault_session_end    <project> <summary>     End a session
vault_summary    <project>                  Project overview
vault_save_prompt <name> <content>          Save a reusable prompt
vault_stats                                 Vault statistics

VAULT HTTP API (http://localhost:7437)
─────────────────────────────────────────────────────
GET  /healthz                               Health check
POST /api/v1/observations                   Save observation
GET  /api/v1/search?q=<query>              Search
GET  /api/v1/context/<project>             Project context
GET  /api/v1/timeline/<project>            Timeline
GET  /api/v1/observations/<id>             Get by ID
GET  /api/v1/stats                         Statistics
GET  /admin/stats          (admin key)     Admin statistics
```

*Last updated: 2026-04-30*
