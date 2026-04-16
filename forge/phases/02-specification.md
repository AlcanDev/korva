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
- src/modules/payments/domain/ports/payment.port.ts (add method)
- src/modules/payments/application/services/payment.service.ts (update)
- src/modules/payments/infrastructure/adapters/base/ (extend)
```

## Rules

- STOP HERE. Wait for explicit approval from the developer before proceeding.
- If the developer modifies the spec, update it and wait for approval again.
- No code, no design — just the spec.
- If multiple specs are possible, present them as numbered options (max 3).
