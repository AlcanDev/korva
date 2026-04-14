# Korva

> AI ecosystem for enterprise development teams.

Korva gives your AI coding assistant (GitHub Copilot, Claude Code, Cursor) persistent memory, architecture-aware instructions, and a structured workflow — all installed with a single command.

## Components

| Component | Description |
|-----------|-------------|
| **Vault** | Persistent memory server — SQLite + FTS5 + MCP protocol |
| **CLI** (`korva`) | Orchestrator — install, sync, status, doctor |
| **Lore** | Knowledge Scrolls — architecture rules loaded on-demand |
| **Sentinel** | Quality guardian — pre-commit hooks + validation |
| **Forge** | SDD workflow — 5-phase structured development |
| **Beacon** | Web dashboard — explore memory, sessions, scrolls |

## Quick Start

```bash
# Install (macOS)
brew install alcandev/tap/korva

# Install (Windows)
winget install AlcanDev.Korva

# Initialize in your project
korva init

# With a team profile (private config)
korva init --profile git@github.com:your-org/korva-team-profile.git

# Check status
korva status

# Install pre-commit hooks
korva sentinel install
```

## How It Works

```
AI Assistant ──MCP──▶ Vault (memory) ──▶ SQLite FTS5
     │                     │
     │               HTTP :7437
     │                     │
     └── Scrolls ──────────▶ Beacon (dashboard)
         (rules)
```

1. **Vault** runs as an MCP server — the AI can save and search observations across sessions
2. **Scrolls** load architecture rules on-demand based on the files you're editing
3. **Sentinel** validates every commit against your team's architecture rules
4. **Forge** guides the AI through a structured design-before-code workflow

## Team Profiles

Private team configuration (proprietary scrolls, internal rules, vault sync) lives in a separate private repository and never touches the public codebase:

```bash
korva init --profile git@github.com:your-org/korva-profile.git
korva sync --profile   # pull latest team config
```

See [docs/TEAM_PROFILES.md](docs/TEAM_PROFILES.md) for details.

## Repository Structure

```
korva/
├── cli/          # korva CLI — Go + Cobra + Bubbletea
├── vault/        # Vault server — Go + SQLite + MCP
├── sentinel/     # Pre-commit hooks + Go validator
├── lore/         # Curated knowledge Scrolls
├── forge/        # SDD workflow phases
└── beacon/       # Web dashboard — React 19 + Vite
```

## Contributing

Korva is open source under the MIT license. Community Scrolls are especially welcome — see [lore/SCROLL_TEMPLATE.md](lore/SCROLL_TEMPLATE.md) to get started.

---

*Build with intent. Ship with confidence.*
