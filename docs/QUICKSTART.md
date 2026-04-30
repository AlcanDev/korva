# Korva Quickstart

> Get from zero to your first AI-assisted session in five minutes.

*Last updated: 2026-04-30*

---

## Prerequisites

- macOS, Linux, or Windows
- One of: Claude Code, Cursor, VS Code, GitHub Copilot, OpenCode, Codex, Gemini CLI, or Windsurf

That's it. No cloud account, no signup, no credit card.

---

## Step 1 — Install

Pick whichever fits your platform:

```bash
# macOS / Linux — single curl
curl -fsSL https://korva.dev/install | bash

# Homebrew (macOS / Linux)
brew install alcandev/tap/korva

# Windows PowerShell
iwr -useb https://korva.dev/install.ps1 | iex

# Manual — download the latest release
#   https://github.com/AlcanDev/korva/releases/latest
```

The installer drops three binaries on your PATH:

| Binary | Purpose |
|--------|---------|
| `korva` | The CLI (init, setup, status, sentinel, license, etc.) |
| `korva-vault` | The local memory + MCP server |
| `korva-sentinel` | Pre-commit architecture validator |

Verify:

```bash
korva --version
# korva 1.0.0 (commit abc1234, 2026-04-30)
```

---

## Step 2 — Initialise the Vault

```bash
korva init
```

This:
- Creates `~/.korva/` with the SQLite vault, config, and admin key
- Generates a fresh `install.id` so cross-team sync (Hive) can identify you
- Drops a default `korva.config.json` you can customise later
- Starts the vault server in the background on `localhost:7437`

Run `korva status` any time to see what's running.

---

## Step 3 — Wire your AI assistant

Each editor has a one-shot setup command. Pick yours:

```bash
korva setup claude-code      # Claude Code
korva setup cursor           # Cursor (drops .cursor/ and .cursor-memory/)
korva setup vscode           # VS Code MCP (drops .vscode/mcp.json)
korva setup copilot          # GitHub Copilot (.github/copilot-instructions.md)
korva setup opencode         # OpenCode (opencode.json)
korva setup codex            # OpenAI Codex (drops AGENTS.md)
korva setup gemini           # Gemini CLI
korva setup windsurf         # Windsurf
```

The setup command writes editor-specific config files into your **current directory**. Run it from the root of the project you want to enable Korva on.

You can run multiple setups in the same project — they don't conflict.

---

## Step 4 — Open your editor

Open the project. Korva is alive. Try:

> "What did we decide about authentication last sprint?"

Your assistant queries the vault and answers from real context — not from a generic LLM hallucination.

> "Add a new endpoint following our patterns."

Your assistant pulls in the matching scrolls (e.g. `nestjs-bff`, `error-handling`) and writes code that follows your team's conventions.

---

## Step 5 — Save knowledge as you go

After a meaningful change, tell your assistant:

> "Save this decision to the vault: we're switching auth from session cookies to JWT with refresh rotation."

It calls the `vault_save` MCP tool and your decision is now part of the team's memory.

Or save automatically at every commit by enabling the post-commit hook:

```bash
korva sentinel install --hook post-commit
```

---

## What's next?

| Want to… | Read |
|----------|------|
| Understand the architecture | [ARCHITECTURE.md](ARCHITECTURE.md) |
| See every CLI command | [CLI.md](CLI.md) |
| Author your own scrolls | [SKILLS.md](SKILLS.md) |
| Self-host the vault for a team | [DEPLOYMENT.md](DEPLOYMENT.md) |
| Activate a Teams or Business licence | [LICENSING.md](LICENSING.md) |
| Open the Beacon dashboard | [ADMIN_PANEL.md](ADMIN_PANEL.md) |

---

*If anything in the quickstart didn't work for you, please [open an issue](https://github.com/AlcanDev/korva/issues/new). The first-time experience matters.*
