# Korva — IDE Integrations

Korva ships first-class integrations for every major AI coding tool. Each subdirectory is a drop-in plugin manifest that wires the local `korva-vault` MCP server into the corresponding IDE.

## Universal behavioral guidelines

Every integration references [`BEHAVIOR.md`](../BEHAVIOR.md) at repo root. That file defines four universal coding principles (think before coding, simplicity first, surgical changes, goal-driven execution) that apply to AI agents in any IDE — whether or not the vault MCP server is running. Pure markdown, zero runtime dependency. Inspired by Karpathy's observations on LLM coding pitfalls, adapted to Korva's Go monorepo.

| IDE / Tool         | Directory          | Install                                            |
| ------------------ | ------------------ | -------------------------------------------------- |
| Claude Code        | `claude-code/`     | `cp claude-code/* ~/.claude/`                      |
| Cursor             | `cursor/`          | `cp cursor/.cursorrules <project>` + MCP config    |
| Windsurf           | `windsurf/`        | `cp windsurf/* <project>/.windsurf/`               |
| OpenAI Codex       | `codex/`           | `cp codex/.codex-plugin.json ~/.codex/plugins/`    |
| GitHub Copilot CLI | `copilot/`         | `cp copilot/copilot-instructions.md .github/`      |
| OpenCode           | `opencode/`        | `cp opencode/* <project>/.opencode/`               |
| Gemini CLI         | `gemini/`          | `gemini extensions install ./gemini`               |

## What gets wired

All integrations expose the same Korva capabilities:

- **`vault_context`** — auto-loaded at session start; injects recent observations, SDD phase, OpenSpec, team skills, team scrolls, and **auto-matched skills** based on the current project + prompt
- **`vault_save`** — decision/pattern/bugfix capture with **semantic dedup** + **decision-conflict detection**
- **`vault_search`**, **`vault_hint`** — full-text search; hint is ~10x cheaper for discovery
- **`vault_skill_match`** — explicit skill resolver
- **`vault_pattern_mine`** — surface emerging implicit patterns
- **`vault_code_health`** — composite project quality score (0-100, A-F grade)
- **`vault_compress`** — caveman-style output compression for token-heavy responses

## Smart Skill Auto-Loader (every IDE)

When the AI calls `vault_context` (which most IDEs do automatically at session start), Korva:

1. Detects the active project
2. Reads the developer's prompt + open file paths
3. Queries the team DB for skills tagged `auto_load=1`
4. Scores each skill against the context (file patterns 45%, keywords 30%, project 15%, tags 10%)
5. Returns the top matches inline in `auto_skills` with full body content + `reason`

The developer never types a slash command; the right team conventions just appear in context.

## Environment knobs

| Variable                | Effect                                                              |
| ----------------------- | ------------------------------------------------------------------- |
| `KORVA_MCP_PROFILE`     | `agent` (default) / `readonly` / `admin` — controls tool surface    |
| `KORVA_OUTPUT_MODE`     | `off` / `lite` / `full` / `ultra` — default compression for output  |
| `KORVA_SESSION_TOKEN`   | Team session token (auto-loaded from `~/.korva/session.token`)      |
