# Phase 3 — Technical Design

**Goal:** Define the exact structure of the solution before writing code.

## Output Format (mandatory)

```
## Technical Design: [feature name]

### New files:
- src/modules/insurances/domain/commands/get-quotes.command.ts
  Layer: Domain | Exports: GetQuotesCommand class

### Modified files:
- src/modules/insurances/domain/ports/insurance.port.ts
  Change: Add getQuotes(command: GetQuotesCommand): Promise<InsuranceQuote[]>

### API contracts (if applicable):
GET /api/v1/insurances/quotes
  Request: CommonHeadersRequestDTO + GetQuotesQueryDTO
  Response: InsuranceQuotesResponseDTO

### Dependency injection changes:
- No changes to module providers

### Key decisions:
- Using Template Method: LifeInsuranceAdapterBase + CL/PE implementations
- Cache key: `quotes:${country}:${customerId}`
```

## Rules

- ⏸️ **STOP HERE.** Wait for explicit ✅ from the developer before implementing.
- Respect hexagonal layers: Domain → Application → Infrastructure (no reverse deps)
- Every new file must have a stated layer and responsibility
- If you find a conflict with the approved spec, flag it before proceeding
