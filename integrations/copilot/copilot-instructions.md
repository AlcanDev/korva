# GitHub Copilot — Korva instructions

This repo is wired to Korva. When using Copilot Chat or Copilot CLI:

- Korva exposes its MCP server as `korva-vault`. The MCP tools are available via the standard `@korva-vault` mention in chat or via the CLI.
- Always start a conversation by calling `vault_context` with the active project name, your prompt, and any relevant file paths. The response includes `auto_skills` — team conventions automatically matched to your task. Apply them silently.
- Use `vault_hint` (10x cheaper than `vault_search`) to scan for prior knowledge before proposing architectural changes.
- After meaningful work, call `vault_save` with the appropriate type (decision, pattern, bugfix, learning, refactor). The vault deduplicates similar entries and warns about conflicting decisions automatically.

## Tool reference

| Tool                  | When to use                                                       |
| --------------------- | ----------------------------------------------------------------- |
| `vault_context`       | Always at the start of a task                                     |
| `vault_skill_match`   | When you need to explicitly fetch skills for a non-obvious prompt |
| `vault_hint`          | Cheap discovery — does anything related to X exist?               |
| `vault_search`        | Full-content search when you'll act on the results immediately    |
| `vault_save`          | Capture knowledge after completing work                           |
| `vault_pattern_mine`  | Periodically — surfaces undocumented conventions                  |
| `vault_code_health`   | Project quality dashboards / pre-merge checks                     |
| `vault_compress`      | Long outputs you'll save to memory or scrolls                     |

## Environment

- `KORVA_MCP_PROFILE` (`agent`/`readonly`/`admin`) controls the tool surface.
- `KORVA_OUTPUT_MODE` (`off`/`lite`/`full`/`ultra`) sets default compression for `vault_compress`.
