# Forge — SDD Workflow

Forge implements a structured development workflow for AI-assisted coding.
It ensures the AI designs before coding and verifies against requirements before closing.

## The 5 Phases

| Phase | Name | Human approval required? |
|-------|------|--------------------------|
| 1 | Exploration | No |
| 2 | Specification | **YES — wait for ✅** |
| 3 | Technical Design | **YES — wait for ✅** |
| 4 | Implementation | No (follows approved design) |
| 5 | Verification | No |

## Phase Details

See `forge/phases/` for the detailed instructions for each phase.

## When to Use Forge

Use the full 5-phase flow for:
- New features or modules
- Changes to public APIs or interfaces
- Refactoring that affects multiple layers
- Any change touching the domain layer

For small, isolated changes (e.g., fixing a typo, updating a config value):
- Phases 1 and 4 only — no spec or design needed

## Integration with Vault

- Phase 1: `vault_search` before proposing anything
- Phase 2: `vault_search "similar patterns"` to inform the spec
- Phase 5: `vault_save` with type=decision or type=pattern after completion

## Abort Condition

If at any point the developer provides new information that invalidates Phase 2 or Phase 3,
**restart from the affected phase** rather than continuing with outdated assumptions.
