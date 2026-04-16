# Korva — The OS for AI-driven Engineering Teams

> Give your AI agents persistent memory, architecture guardrails, knowledge injection, and structured workflows — all in a single local system. Free. Open source. Zero cloud.

[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go 1.22+](https://img.shields.io/badge/go-1.22%2B-blue.svg)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-compatible-cyan.svg)](https://modelcontextprotocol.io)

---

## The Problem

Every AI session starts from zero. Your AI doesn't know the race condition you fixed last October. It doesn't know you chose event sourcing in March. It doesn't know the team rule "never access the database from a controller." 

Every developer explains context for 15 minutes before every session. Every. Single. Day.

**Korva is the infrastructure layer that fixes this.**

---

## What Korva Does

```
Without Korva                         With Korva
─────────────────────────────         ──────────────────────────────────────
Session #47:                          vault_context() → 89 team memories
You: "Remember: Clean Architecture,    ✓ Architecture: CQRS + Events (Mar 2024)
     Repository pattern, CQRS..."      ✓ Rule: Repository interface only
AI:  [violates everything again]       ✓ Incident: Direct DB = prod outage

// Same mistake, 47 sessions.         AI generates perfect code.
// New dev tomorrow? Same conv.       First try. No explanation needed.
```

Korva is built on **four integrated components**:

| Component | What it does |
|---|---|
| 🧠 **Vault** | Persistent AI memory — decisions, incidents, patterns saved forever |
| 🛡️ **Sentinel** | Architecture guardrails — catches violations before they reach your codebase |
| 📜 **Lore** | Knowledge injection — opens `payments.ts`, AI already knows PCI + Stripe rules |
| ⚙️ **Forge** | Structured workflow — 5-phase SDD prevents AI from diving straight into code |

---

## Install in 30 seconds

### macOS / Linux (Homebrew)

```bash
brew install alcandev/tap/korva
```

### macOS / Linux (shell script)

```bash
curl -fsSL https://korva.dev/install.sh | bash
```

### Windows (PowerShell, run as Administrator)

```powershell
irm https://korva.dev/install.ps1 | iex
```

> **Windows note**: Installs to `%LOCALAPPDATA%\korva\bin`. Restart your terminal after install.

### Verify installation

```bash
korva version
```

---

## Quick Start (5 minutes)

### 1. Initialize

```bash
korva init
```

Creates `~/.korva/` with config, generates `admin.key` (0600 permissions), starts the vault server on `:7437`.

### 2. Connect your AI editors

```bash
korva setup --all
```

Auto-configures **VS Code + Copilot**, **Claude Code**, and **Cursor** to use Korva as an MCP server. No JSON editing required. Idempotent — safe to run multiple times.

### 3. (Optional) Add your team profile

```bash
korva init --profile git@github.com:YOUR_ORG/korva-team-profile.git
```

Clones your team's private scrolls, Sentinel rules, and AI instructions into your local workspace.

### 4. Start using it

In any AI session, your agent now has access to:

```
vault_save   — save a decision, bug fix, or pattern
vault_context — load relevant context for the current project
vault_search  — full-text search across everything saved
vault_why    — explain why a past decision was made (coming v0.2)
```

---

## Real Examples

### Vault — Memory that compounds over time

```javascript
// Friday 11pm: critical production incident
vault_save({
  type: "incident",
  title: "Race condition in payment processor",
  content: "Two concurrent requests can double-charge.
            Fix: Redis distributed lock on payment_id.
            LOCK:payment:{id} with 30s TTL — always."
})

// 9 months later, new developer opens payments.ts:
vault_context("payments")
// → AI: "A past incident shows race conditions here.
//        Use distributed locking on payment_id
//        or you risk double-charging customers."
// Saved: 3-day debugging session, ~$12k incident cost
```

### Sentinel — Guardrails on every commit

```bash
$ git commit -m "feat: user authentication endpoint"

Running Korva Sentinel...
  ✓ NAM-001  Naming conventions
  ✓ TEST-001 No debug logs in production
  ✗ SEC-001  Hardcoded secret detected
  ✗ SEC-003  Timing attack vulnerability
  ✗ ARC-002  HTTP handler in domain layer

  src/auth/AuthService.ts:14
  const secret = "sk_live_4xK9mP..."
                  ^^^ Use process.env.JWT_SECRET

3 critical issues. Commit blocked.
```

### Lore — Knowledge injected automatically

```
// You open: src/payments/checkout.ts
// Korva detects: payments + stripe context

📜 stripe-webhooks  Idempotency keys required
📜 pci-dss          Never log card numbers or CVV
📜 decimal-math     Use Decimal.js — never floats
📜 retry-patterns   Exponential backoff on 429s

// AI already knows all of this. No explanation needed.
```

### Forge — Structured AI development

```markdown
# Building: Real-time notifications system

◆ Phase 1: Exploration (no code yet)
  "Map all mutation paths before designing conflict resolution"

◆ Phase 2: Specification  
  "WebSocket vs SSE? Define the contract before implementation."

◆ Phase 3: Architecture (vault-aware)
  "Team uses Redis pub/sub (23 vault observations). 
   Design topology respecting that constraint."

◆ Phase 4: Implementation — step by step, guided

◆ Phase 5: Sentinel validates
  ✓ 10/10 architecture rules pass
```

---

## The Public / Private Model

Korva is built on the **3 Kingdoms privacy model**:

```
Kingdom 1 — Public (this repo, MIT)
github.com/AlcanDev/korva
Core engine · CLI · Vault · Sentinel · 13+ Lore scrolls
Zero knowledge of your team's data.

     ↓ can reference, never merges ↑

Kingdom 2 — Your private team repo (your GitHub)
github.com/YOUR_ORG/korva-team-profile
Team scrolls · Custom rules · AI instructions
Your patterns, your conventions, your IP.

     ↓ syncs locally, never to cloud ↑

Kingdom 3 — Your machine (~/.korva/)
vault.db · admin.key (0600) · runtime state
Stays here. Forever. Unless you choose otherwise.
```

### Optional: Share vault across your team

```bash
# Self-host the vault on your own infrastructure
docker run -p 7437:7437 -v ~/.korva:/data ghcr.io/alcandev/korva-vault

# Team members sync (only non-sensitive observations shared)
korva sync --remote https://korva.your-company.internal
```

This is **always opt-in**. Korva never connects to our servers. You control what syncs.

---

## Supported Editors

| Editor | Integration | Status |
|---|---|---|
| VS Code + GitHub Copilot | MCP via `mcp.json` | ✅ Supported |
| Claude Code | MCP via `settings.json` | ✅ Supported |
| Cursor | MCP via `mcp.json` | ✅ Supported |
| JetBrains (IntelliJ, GoLand...) | MCP via plugin | 🔨 Roadmap |
| Neovim | MCP via plugin | 🔨 Roadmap |

Any editor that supports the [Model Context Protocol](https://modelcontextprotocol.io) works with Korva.

---

## Manual Editor Configuration

If `korva setup --all` doesn't cover your editor, configure manually:

### VS Code (`~/.vscode/settings.json` or project `.vscode/mcp.json`)

```json
{
  "mcp": {
    "servers": {
      "korva-vault": {
        "type": "stdio",
        "command": "korva-vault",
        "args": ["--mode=mcp"]
      }
    }
  }
}
```

### Claude Code (`~/.claude/settings.json`)

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

### Cursor (`~/.cursor/mcp.json`)

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

---

## Components

```
korva/
├── vault/       → Vault server — SQLite FTS5 + MCP (stdio) + REST :7437
├── cli/         → korva CLI — init, setup, sync, sentinel, forge
├── sentinel/    → Architecture validator — 10 built-in rules + custom YAML
├── lore/
│   └── curated/ → 13 knowledge scrolls (NestJS, TypeScript, Docker, CI/CD...)
├── forge/       → SDD workflow — 5-phase structured development
└── beacon/      → Web dashboard — React 19 + Vite (explore vault history)
```

---

## Build from Source

Requires **Go 1.22+**.

```bash
git clone https://github.com/AlcanDev/korva.git
cd korva

# Build all binaries
go build -o bin/korva          ./cli/cmd/korva/
go build -o bin/korva-vault    ./vault/cmd/korva-vault/
go build -o bin/korva-sentinel ./sentinel/validator/cmd/korva-sentinel/

# Add to PATH
export PATH="$PATH:$(pwd)/bin"

# Run all tests
go test github.com/alcandev/korva/...
```

---

## Documentation

| Document | Description |
|---|---|
| [VISION.md](VISION.md) | Strategic vision — 5-layer architecture, public/private model |
| [ROADMAP.md](ROADMAP.md) | Phase 1→3 roadmap with detailed task breakdown |
| [docs/USAGE.md](docs/USAGE.md) | Detailed usage guide (all commands, all options) |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Deploy shared vault server (Railway/Fly.io/VPS/K8s) |
| [lore/SCROLL_TEMPLATE.md](lore/SCROLL_TEMPLATE.md) | How to write a Lore scroll for any stack |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute scrolls, rules, and code |
| [SECURITY.md](SECURITY.md) | Security policy and responsible disclosure |

---

## FAQ

**Is Korva really free? What's the catch?**  
No catch. MIT license. No paid tier, no telemetry, no SaaS. It runs entirely on your machine. The source is here — verify it yourself.

**Does my code leave my machine?**  
No. The vault runs on `localhost:7437`. MCP communicates via stdin/stdout — no network requests. The privacy filter auto-redacts passwords, tokens, and Bearer keys before saving to SQLite.

**How is this different from .cursorrules or CLAUDE.md?**  
Static files don't accumulate knowledge, can't enforce rules at commit time, and don't inject context automatically. Korva is a cognitive system, not a config file.

**Can I use it with my stack (not NestJS/hexagonal)?**  
Yes. Vault and Lore are stack-agnostic. Sentinel rules are configurable. The community is adding scrolls for Next.js, Laravel, Rust, Go, Python, and more.

---

## Contributing

The highest-impact contributions right now:

1. **Write a Lore scroll** for your stack — see [SCROLL_TEMPLATE.md](lore/SCROLL_TEMPLATE.md)
2. **Add a Sentinel rule** for patterns your team enforces
3. **Report bugs** with clear reproduction steps in [GitHub Issues](https://github.com/AlcanDev/korva/issues)
4. **Star the repo** — helps developers discover Korva

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide.

---

## License

[MIT](LICENSE) — © 2025 AlcanDev

*Build with intent. Ship with confidence.*
