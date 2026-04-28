# Korva Windsurf Rules

## Behavioral guidelines

Repo-root `BEHAVIOR.md` defines four universal coding principles that apply
to every task: think before coding, simplicity first, surgical changes,
goal-driven execution. Read it once per session and treat it as
authoritative across all tools.

## Vault MCP

Korva is wired in as `korva-vault` (MCP). Use it like this:

1. Open of any conversation → call `vault_context` with `project`, `prompt`, `file_paths`. Apply returned `auto_skills` silently.
2. Before proposing architecture → call `vault_hint` to scan prior knowledge cheaply.
3. After completing a unit of work → call `vault_save` with `type=decision|pattern|bugfix|learning`.

The vault enforces semantic dedup and decision-conflict detection automatically. Users never need to ask "have we decided this before" — Korva will tell you in the response.

## Compression

For long-form responses where the user requested a summary or report, apply `vault_compress` with `mode=full` before storing in memory or scrolls. Code blocks pass through unaltered.
