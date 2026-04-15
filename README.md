# Korva

> AI ecosystem for enterprise development teams.

Korva gives your AI coding assistant (GitHub Copilot, Claude Code, Cursor) **persistent memory**, **architecture-aware instructions**, and a **structured workflow** — all installed with a single command.

---

## ⚡ Install in 1 command

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.sh | sh
```

Or via Homebrew:

```bash
brew tap AlcanDev/tap && brew install korva
```

### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.ps1 | iex
```

> **PATH note for Windows**: The installer adds the binaries to `%LOCALAPPDATA%\korva\bin` and updates your user PATH. **Restart your terminal** (PowerShell / CMD) after installing.

### Verify installation

```bash
korva version       # should print version, commit, date
korva-vault --help
korva-sentinel --help
```

---

## 🚀 Quick Start (5 minutes)

### Step 1 — Initialize Korva

```bash
# In your project root:
korva init
```

This creates `~/.korva/config.json` and starts the Vault server configuration.

### Step 1b — Auto-configure your editors (NEW)

```bash
# Detects VS Code, Cursor, and Claude Code — configures all of them at once
korva setup
```

This automatically writes the MCP server configuration into every editor it finds. **No manual editing required.**

### Step 2 — Start the Vault MCP server

```bash
korva-vault --mode=both   # Starts MCP (stdio) + HTTP REST on :7437
```

### Step 3 — Connect your AI assistant

**VS Code + GitHub Copilot** — add to `.vscode/mcp.json` (or User settings):
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

**Claude Code** — add to `~/.claude/settings.json`:
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

**Cursor** — add to `~/.cursor/mcp.json`:
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

### Step 4 — Install pre-commit hooks

```bash
cd ~/repos/my-project
korva sentinel install
```

### Step 5 — Check everything is working

```bash
korva doctor
```

---

## 📦 Components

| Component | Binary | Description |
|-----------|--------|-------------|
| **Vault** | `korva-vault` | Persistent memory — SQLite + FTS5 + MCP + HTTP REST :7437 |
| **CLI** | `korva` | Orchestrator — init, sync, status, doctor, sentinel |
| **Sentinel** | `korva-sentinel` | Static analysis — 10 architecture rules (HEX, NAM, SEC, TEST) |
| **Lore** | — | Knowledge Scrolls loaded on-demand by your AI assistant |
| **Forge** | — | SDD workflow — 5-phase structured development |
| **Beacon** | — | Web dashboard (React 19 + Vite) — explore memory and sessions |

---

## 👥 Team Profiles (private configuration)

Private configuration (proprietary scrolls, internal rules, vault sync) lives in a **separate private repository** and never touches this public codebase. This is the **3 Kingdoms privacy model**:

- 🌍 **Kingdom 1** — this public repo (open source)
- 🔒 **Kingdom 2** — your private team profile repo
- 💻 **Kingdom 3** — your local machine (`~/.korva/`)

```bash
# Install for your team (use your team's private profile URL)
korva init --profile git@github.com:your-org/korva-team-profile.git

# Sync latest team config (run when team updates the profile)
korva sync --profile
```

To create your own team profile repo, see **[korva-team-profile](https://github.com/AlcanDev/korva-team-profile)** as a reference template (rename and make private for your team).

---

## 🏗️ Architecture

```
AI Assistant ──MCP (stdio)──▶ korva-vault ──▶ SQLite FTS5 (~/.korva/vault/)
     │                              │
     │                        HTTP :7437
     │                              │
     │                         Beacon UI
     │
     └── Lore Scrolls ──▶ .github/copilot-instructions.md
                          CLAUDE.md
                          .cursorrules
```

### MCP Tools available to your AI

| Tool | What it does |
|------|-------------|
| `vault_save` | Save a decision, pattern, or bug fix |
| `vault_search` | Full-text search across all observations |
| `vault_context` | Get recent context for the current project |
| `vault_timeline` | Observations by date range |
| `vault_session_start` / `vault_session_end` | Track work sessions |
| `vault_summary` | Project summary with key decisions |
| `vault_save_prompt` | Save reusable prompts |
| `vault_stats` | Global vault statistics |

---

## 🗂️ Repository Structure

```
korva/
├── cli/               # korva CLI — Go + Cobra
├── vault/             # Vault server — SQLite + MCP + REST
├── sentinel/          # Pre-commit hooks + Go validator
├── lore/
│   └── curated/       # 13 knowledge Scrolls (hexagonal, NestJS, CI/CD...)
├── forge/             # SDD workflow phases (5 .md files)
├── scripts/
│   ├── install.sh     # macOS/Linux one-line installer
│   └── install.ps1    # Windows PowerShell installer
└── beacon/            # Web dashboard — React 19 + Vite 6
```

---

## 🛠️ Build from Source

Requires **Go 1.22+**.

```bash
# Clone
git clone https://github.com/AlcanDev/korva.git
cd korva

# Sync workspace
go work sync

# Build all binaries
go build -o bin/korva        ./cli/cmd/korva/
go build -o bin/korva-vault  ./vault/cmd/korva-vault/
go build -o bin/korva-sentinel ./sentinel/validator/cmd/korva-sentinel/

# Add bin/ to your PATH (macOS/Linux)
export PATH="$PATH:$(pwd)/bin"

# Run all tests
go test github.com/alcandev/korva/...
```

---

## ✅ Tests

```bash
# Full test suite
go test github.com/alcandev/korva/...

# Tests for a specific module
cd vault && go test ./...
cd internal && go test ./...

# With coverage report
go test ./internal/... -cover
go test ./vault/... -cover
go test ./sentinel/validator/... -cover
```

Coverage targets: **>80%** on all testable packages.

---

## 🔐 Security

- `admin.key` is generated locally, stored at `~/.korva/admin.key` with permissions `0600` (read-only by owner). It is **never** committed to git or synced anywhere.
- Admin endpoints (`POST /admin/purge`, etc.) require the `X-Admin-Key` header with a valid key.
- The Privacy Filter (`internal/privacy`) redacts passwords, tokens, secrets, and `<private>` tagged content before saving to SQLite.
- Report security issues via GitHub Security Advisories — see [SECURITY.md](SECURITY.md).

---

## 📖 Documentation

| Document | Contents |
|----------|----------|
| [docs/USAGE.md](docs/USAGE.md) | Step-by-step usage guide |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Deploy shared vault server (Railway / Fly.io / VPS) |
| [docs/ADMIN_PANEL.md](docs/ADMIN_PANEL.md) | Admin panel — monitor team intelligence, manage scrolls |
| [docs/TEAM_PROFILES.md](docs/TEAM_PROFILES.md) | Team profile setup and management |
| [lore/SCROLL_TEMPLATE.md](lore/SCROLL_TEMPLATE.md) | How to write a knowledge Scroll |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | Security policy |
| [CLAUDE.md](CLAUDE.md) | Instructions for Claude Code |

---

## 📄 License

MIT — see [LICENSE](LICENSE).

---

*Build with intent. Ship with confidence.*
