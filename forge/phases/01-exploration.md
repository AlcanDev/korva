# Phase 1 — Exploration

**Goal:** Understand the codebase and gather context before proposing anything.

## Actions (performed by the AI, no approval needed)

1. Read the relevant source files
2. Search Vault: `vault_search "module or concept"`
3. Identify: existing patterns, dependencies, technical debt in this area
4. Understand the team's constraints (architecture layers, adapters, shared libs, etc.)

## Output

A brief analysis in this format:

```
Exploration complete:
- Found: [what already exists]
- Impact: [what this change will affect]
- Debt: [any existing issues to be aware of]
- Vault context: [relevant prior decisions]
```

## Rules

- Do NOT propose solutions yet
- Do NOT write code yet
- If something is unclear, ask ONE focused question
- Proceed to Phase 2 when you have enough context
