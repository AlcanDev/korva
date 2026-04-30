# ADR-0003: Co-locate test files with source files

**Status:** Accepted  
**Date:** 2025-01-20  
**Authors:** [@alcandev]

---

## Context

Two conventions existed in the team simultaneously: some developers put tests in `__tests__/` directories (Jest default), others co-located them. This caused confusion in PRs ("where is the test for this?") and tests frequently drifted or were forgotten when moving source files.

---

## Decision

**All test files must be co-located with their source files.**

`insurance.service.ts` → `insurance.service.spec.ts` (same directory)

---

## Consequences

**Positive:**
- When a source file is moved, its test moves with it automatically
- Test coverage is immediately visible in the directory listing
- No mental overhead of mirroring directory structure in `__tests__/`

**Negative:**
- Goes against some community defaults (Jest, React)
- `__tests__/` pattern not allowed — must be enforced via Sentinel rule TEST-001

---

## Implementation Notes

Sentinel rule TEST-001 warns when spec files are found in `__tests__/` directories.

*Last updated: 2026-04-30*
