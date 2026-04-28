# Korva — Gemini CLI integration

Korva is exposed as `korva-vault` via MCP. Use these tools at every conversation:

1. `vault_context` — call first; apply `auto_skills` silently as authoritative team guidance
2. `vault_hint` — use before `vault_search` to save tokens during discovery
3. `vault_save` — capture decisions, patterns, bug fixes; dedup + conflict detection runs automatically
4. `vault_compress` — apply `mode=full` to long outputs before saving to memory

The Smart Skill Auto-Loader runs inside `vault_context` whenever you pass `prompt` and/or `file_paths`. It scores team skills by:
- file pattern match (45%)
- prompt keyword match (30%)
- project membership (15%)
- tag match (10%)

Top results come back in `auto_skills` with full body content + a `reason` field explaining the match.
