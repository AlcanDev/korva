# Phase 2 — Specification

**Goal:** Define exactly what will be built, in terms the developer can approve.

## Output Format (mandatory)

```
## Spec: [feature name]

**Objective:** What this does in one sentence.

**Inputs:**
- paramName: TypeScript type — description

**Outputs:**
- returnType — description

**Business Rules / Constraints:**
1. Rule one
2. Rule two

**Affects:**
- src/modules/insurances/domain/ports/insurance.port.ts (add method)
- src/modules/insurances/application/services/insurance.service.ts (update)
- src/modules/insurances/infrastructure/adapters/base/ (extend)
```

## Rules

- ⏸️ **STOP HERE.** Wait for explicit ✅ from the developer before proceeding.
- If the developer modifies the spec, update it and wait for ✅ again.
- No code, no design — just the spec.
- If multiple specs are possible, present them as numbered options (max 3).
