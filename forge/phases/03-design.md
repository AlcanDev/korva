# Phase 3 — Technical Design

**Goal:** Define the exact structure of the solution before writing code.

## Before designing

Load team architecture patterns from Vault:

```
vault_context   ← run at session start to load all active context
vault_search "adapter pattern"   ← find prior decisions for this area
vault_search "module structure"  ← confirm established conventions
```

Use what you find to align the design with existing patterns. If a prior decision conflicts with the proposed approach, surface it explicitly.

## Output Format (mandatory)

```
## Technical Design: [feature name]

### New files:
- src/modules/payments/domain/commands/get-plans.command.ts
  Layer: Domain | Exports: GetPlansCommand class

### Modified files:
- src/modules/payments/domain/ports/payment.port.ts
  Change: Add getPlans(command: GetPlansCommand): Promise<PaymentPlan[]>

### API contracts (if applicable):
GET /api/v1/payments/plans
  Request: CommonHeadersRequestDTO + GetPlansQueryDTO
  Response: PaymentPlansResponseDTO

### Dependency injection changes:
- No changes to module providers

### Key decisions:
- Using Template Method: StripePaymentAdapterBase + US/EU implementations
- Cache key: `plans:${region}:${customerId}`
```

## Rules

- STOP HERE. Wait for explicit approval from the developer before implementing.
- Respect the team's architecture layers — no reverse dependencies
- Every new file must have a stated layer and responsibility
- If you find a conflict with the approved spec, flag it before proceeding
- If Vault reveals a prior decision that changes the design, update and re-present
