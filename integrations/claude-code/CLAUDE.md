# Korva — Claude Code Integration

This project is wired to Korva via the `korva-vault` MCP server.

## Behavioral guidelines

Korva ships a universal behavioral discipline file at the repo root:
[BEHAVIOR.md](../../BEHAVIOR.md). Read it once per task — its four principles
(think before coding, simplicity first, surgical changes, goal-driven
execution) apply across every IDE Korva supports. They are deliberately short
and concrete; they exist to prevent the typical LLM coding pitfalls
(over-engineering, orthogonal refactors, silent assumption-picking).

## What you get for free

- `vault_context` runs at session start — recent decisions, SDD phase, team skills, and **auto-matched team conventions** are injected automatically.
- `vault_save` is the canonical way to capture a decision or pattern. Semantic dedup catches near-duplicates; decision-conflict detection warns when a new decision contradicts an existing one.
- Auto-loaded team skills appear in the `auto_skills` array of the `vault_context` response. Apply them as guidance without asking the user.

## Workflow

1. **Start a task** — call `vault_context` with `project`, `prompt` (your understanding of the task), and `file_paths` (files you're touching). You receive recent context + matched skills in one round-trip.
2. **Search before proposing** — `vault_hint` is 10x cheaper than `vault_search`. Use it to scan for prior work; only call `vault_get` for entries you actually need full content for.
3. **Save what you learn** — `vault_save` with `type=decision|pattern|bugfix|learning`. Dedup runs automatically; pass `force=true` to override.
4. **End the session** — `vault_session_end` with a summary so the next session has a starting point.

## When to use compression

Long outputs (full README rewrites, multi-file diffs explained inline) cost a lot of tokens. Use `vault_compress` with `mode=full` to produce a 50-65% smaller version for memory/scroll storage. Code blocks pass through untouched.
