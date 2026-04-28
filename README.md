# Korva — The OS for AI-driven Engineering Teams

> Give your AI agents persistent memory, architecture guardrails, knowledge injection, and structured workflows — all in a single local system. Free. Open source. Zero cloud.

[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go 1.26+](https://img.shields.io/badge/go-1.26%2B-blue.svg)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-2024--11--05-cyan.svg)](https://modelcontextprotocol.io)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)](#build-from-source)
[![Version](https://img.shields.io/badge/version-1.0-orange.svg)](CHANGELOG.md)

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
| ⚙️ **Forge** | Structured workflow — 9-phase SDD prevents AI from diving straight into code |
| 🎛️ **Beacon** | Web dashboard — vault explorer, admin panel, Skills Hub |

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

### Windows (PowerShell)

```powershell
irm https://korva.dev/install.ps1 | iex
```

> Installs to `%LOCALAPPDATA%\korva\bin` and updates your user PATH automatically. Open a new terminal after install.

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
vault_save          — save a decision, bug fix, or pattern
vault_context       — load relevant context (local + cloud hybrid)
                        auto_skills injected automatically (Teams+ feature)
vault_search        — full-text search across everything saved
vault_sdd_phase     — track/advance the 9-phase SDD workflow
vault_qa_checklist  — get quality criteria for the current phase
vault_qa_checkpoint — record a QA assessment and unlock phase gates
vault_team_context  — load team skills and private scrolls
vault_bulk_save     — save up to 50 observations in one call
vault_timeline      — show a project's observation history over time
vault_stats         — vault usage statistics
vault_skill_match   — find best skills for a task (Teams+)
vault_code_health   — code quality score A–F (Business+)
vault_pattern_mine  — extract recurring patterns from vault (Business+)
vault_compress      — compress long context to key insights
vault_hint          — get context-aware suggestions
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

> **60+ community skills**: Install curated community skills from [skills.sh](https://skills.sh) to give your AI best practices for every library in your stack — alongside Korva's team scrolls. See [docs/COMMUNITY-SKILLS.md](docs/COMMUNITY-SKILLS.md).

> **Smart Skill Auto-Loader** *(Teams+)*: Tag any skill with `auto_load=true` and Korva automatically injects the best-matching skills into every `vault_context` call — no explicit invocation needed. Your AI arrives to each session pre-loaded with the right conventions for the file it's editing.

### Forge — 9-Phase Spec-Driven Development (SDD)

```
Phase       Gate?  Description
──────────────────────────────────────────────────────
explore     —      Map the problem space. No code.
propose     —      Define approach options + trade-offs
spec        —      Write the formal specification
architect   —      Design system/module structure
apply       ✅ QA  Write the code
verify      ✅ QA  Tests, coverage, Sentinel rules
archive     —      Commit learnings to vault
retrospect  —      Team review and process notes
close       —      Final sign-off

✅ = quality gate required (vault_qa_checkpoint with gate_passed=true)
    before the phase can advance. Prevents shipping untested code.
```

Example — advancing past the quality gate:
```
vault_qa_checklist({ phase: "apply", language: "go" })
// → 12 criteria for Go implementation quality

vault_qa_checkpoint({ project: "payments", phase: "apply",
  status: "pass", score: 87, gate_passed: true,
  findings: [{ rule: "GO-APP-001", status: "pass" }] })
// → "Gate unlocked. Transition apply → verify is now allowed."

vault_sdd_phase({ project: "payments", phase: "verify" })
// → Phase advanced ✓
```

---

## Privacy Model: The 3 Kingdoms

> **Enterprise differentiator.** This is how Korva separates public tooling from private team knowledge — a strict, auditable privacy boundary that keeps your IP in your control.

Korva enforces a **3 Kingdoms privacy model** where each data type lives in exactly one kingdom and **never crosses to another**:

```
┌─────────────────────────────────────────────────────────────────────┐
│  KINGDOM 1 — Public                github.com/alcandev/korva        │
│  MIT license · Open source · Zero knowledge of your team's data    │
│  Core engine · CLI · Vault · Sentinel · 18+ curated Lore scrolls   │
│                                                                     │
│     can reference ↓          never merges ↑                        │
├─────────────────────────────────────────────────────────────────────┤
│  KINGDOM 2 — Private Team      github.com/YOUR-ORG/team-profile     │
│  Team scrolls · Custom Sentinel rules · AI instruction extensions  │
│  Your patterns, your conventions, your IP. Shared within your team.│
│                                                                     │
│     syncs locally ↓          never to cloud ↑                      │
├─────────────────────────────────────────────────────────────────────┤
│  KINGDOM 3 — Your Machine                      ~/.korva/            │
│  vault.db · admin.key (0600) · runtime observations                │
│  Stays here. Forever. Unless you explicitly choose to share it.    │
└─────────────────────────────────────────────────────────────────────┘
```

**The invariant:** Data flows inward only. Nothing from your machine or private repo ever touches the public repo. Your `admin.key` never leaves Kingdom 3. Zero cloud required by default.

### Optional: Share vault across your team

```bash
# Self-host the vault on your own infrastructure
docker run -p 7437:7437 -v ~/.korva:/data ghcr.io/alcandev/korva-vault

# Team members sync (only non-sensitive observations shared)
korva sync --remote https://korva.your-company.internal
```

This is **always opt-in**. Korva never connects to our servers. You control what syncs.

---

## Hybrid Context: Local + Cloud Brain

By default Korva is **100% local** — your vault lives in `~/.korva/vault/observations.db` and never leaves your machine. When Korva Hive (the optional community cloud) is enabled, `vault_context` and `vault_search` query **both sources in parallel** and merge the results:

```
vault_context({ project: "payments" })

→ {
    context:      [...],          // local SQLite — your team's history
    hive_context: [...],          // community brain — patterns from the ecosystem
    hive_status:  "ok",           // "ok" | "unavailable" | "disabled"
    sdd_phase:    "apply"
  }
```

**If Hive is unreachable, the tool succeeds with `hive_status: "unavailable"`.** Local context is always returned regardless — cloud failure never blocks your workflow.

The cloud privacy filter enforces a **default-deny** policy before any data leaves your machine: PII, file paths, secrets, and JWT tokens are stripped before upload. `KORVA_HIVE_DISABLE=1` is a kill-switch that disables the entire outbound pipeline without touching config.

---

## Supported Editors

| Editor | Integration | Status |
|---|---|---|
| VS Code + GitHub Copilot | MCP via `mcp.json` | ✅ Supported |
| Claude Code | MCP via `settings.json` | ✅ Supported |
| Cursor | MCP via `mcp.json` | ✅ Supported |
| Windsurf | MCP via `global_rules.md` | ✅ Supported |
| Gemini CLI | MCP via `GEMINI.md` | ✅ Supported |
| OpenAI Codex | MCP via `.codex-plugin.json` | ✅ Supported |
| OpenCode | MCP via `opencode.json` | ✅ Supported |
| JetBrains (IntelliJ, GoLand...) | MCP via plugin | 🔨 Roadmap |
| Neovim | MCP via plugin | 🔨 Roadmap |

See [`integrations/`](integrations/) for copy-paste configuration snippets for every supported editor.

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
├── internal/        → shared Go packages (db, config, privacy, admin, license, hive)
├── vault/           → Vault server — SQLite FTS5 + 18 MCP tools (stdio) + REST :7437
├── cli/             → korva CLI — init, setup, sync, teams, license, hive
├── sentinel/        → Architecture validator — 10 built-in rules + custom YAML
├── lore/
│   └── curated/     → 13+ knowledge scrolls (NestJS, TypeScript, Docker, CI/CD...)
├── forge/
│   └── hive-mock/   → local Hive server for development (:7438)
├── integrations/    → copy-paste MCP configs for 8 AI editors
└── beacon/          → Web dashboard — React 19 + Vite (vault explorer + admin panel)
```

---

## Build from Source

Requires **Go 1.26+**.

```bash
git clone https://github.com/AlcanDev/korva.git
cd korva

# Build all binaries (vault without embedded Beacon)
make build

# Build vault with Beacon UI embedded (requires Node 18+)
make vault-full

# Add to PATH
export PATH="$PATH:$(pwd)/bin"

# Run all tests
go test github.com/alcandev/korva/...

# Generate shell completions (bash, zsh, fish → completions/)
make completions
```

---

## Documentation

| Document | Description |
|---|---|
| [VISION.md](VISION.md) | Strategic vision — 5-layer architecture, public/private model |
| [ROADMAP.md](ROADMAP.md) | Phase 1→3 roadmap with detailed task breakdown |
| [docs/USAGE.md](docs/USAGE.md) | Detailed usage guide (all commands, all options) |
| [docs/LICENSING.md](docs/LICENSING.md) | Licensing server deployment + license activation guide |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Deploy shared vault server (Railway/Fly.io/VPS/K8s) |
| [docs/COMMUNITY-SKILLS.md](docs/COMMUNITY-SKILLS.md) | Community skills — 60+ curated skills for your stack |
| [lore/SCROLL_TEMPLATE.md](lore/SCROLL_TEMPLATE.md) | How to write a Lore scroll for any stack |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute scrolls, rules, and code |
| [SECURITY.md](SECURITY.md) | Security policy and responsible disclosure |

---

## Open Core Model

**Korva Community** (this repo) is free forever — MIT license, no telemetry, no cloud required.

**Korva for Teams** and **Korva Business** are paid tiers for engineering teams:

| Feature | Community | Teams | Business |
|---------|-----------|-------|---------|
| Vault, Sentinel, Lore, Forge, Beacon | ✅ | ✅ | ✅ |
| 8-editor integration manifests | ✅ | ✅ | ✅ |
| Team Profile via private Git repo | ✅ | ✅ | ✅ |
| Private Scrolls managed from Beacon | ❌ | ✅ | ✅ |
| Skills Hub in Beacon (`vault_skill_match`) | ❌ | ✅ | ✅ |
| Smart Skill Auto-Loader in `vault_context` | ❌ | ✅ | ✅ |
| Audit log (who changed what, when) | ❌ | ✅ | ✅ |
| Custom config overrides per team | ❌ | ✅ | ✅ |
| Code Health grades (`vault_code_health`) | ❌ | ❌ | ✅ |
| Pattern mining (`vault_pattern_mine`) | ❌ | ❌ | ✅ |
| Multi-profile workspaces | ❌ | ❌ | ✅ |
| Private Hive sync (not community) | ❌ | ❌ | ✅ |
| Priority support | ❌ | ✅ | ✅ |

License keys are **offline-first** (RS256 JWS) — the vault verifies the license locally on every start with no network call. A single online heartbeat every 24 h keeps the license current; 7-day grace period if the server is temporarily unreachable.

```bash
# Activate after purchase
korva license activate KORVA-XXXX-XXXX-XXXX-XXXX

# Check license status
korva license status
```

> Teams: $12/user/month · Business: $32/user/month → [korva.dev/pricing](https://korva.dev/pricing)

---

## FAQ

**Is Korva really free? What's the catch?**  
The core product (Vault, Sentinel, Lore, Forge, Beacon) is MIT license — free forever, no telemetry, no SaaS required. Korva for Teams is an optional paid upgrade for teams that want managed private scrolls, skills, and audit logging from the Beacon panel. See the table above.

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

---

*Build with intent. Ship with confidence.*
