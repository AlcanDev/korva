# Phase 5 — Verification

**Goal:** Confirm the implementation matches the spec. No new code — only validation.

## Actions

1. **Spec review:** Go through each item in Phase 2 spec. Mark ✓ or ✗.
2. **Anti-pattern scan:** Check for Sentinel violations (HEX-001 through TEST-001).
3. **Test list:** Enumerate the tests that should be written (do NOT generate them unless asked).
4. **Vault save:** Record what was learned.

## Output Format

```
## Verification: [feature name]

### Spec checklist:
✓ Objective: GetInsuranceOffersCommand implemented
✓ Input types: correct TypeScript types
✓ Output types: InsuranceOffer[] matches spec
✓ Business rule 1: cache key includes country
✗ Business rule 2: rate limiting NOT implemented (not in scope for this PR)

### Anti-pattern check:
✓ No framework imports in domain/
✓ Application layer only depends on InsurancePort interface
✓ No console.log in src/
✓ DTO uses uppercase suffix

### Tests to write:
1. InsuranceService.getOffers: mock InsurancePort, verify cache miss → fetch → cache set
2. LifeInsuranceAdapterCL.getOffers: mock FifHttpService, verify URL and headers
3. InsuranceController.getOffers: verify delegation to service, no logic

### Saved to Vault:
- vault_save type=decision: "Template Method for country adapters with base class"
- vault_save type=pattern: "InsurancePort injection via @Inject(INSURANCE_PORT)"
```

## Rules

- Be honest about what's missing — incomplete implementations should be flagged
- Never mark ✓ for something you're unsure about
- The test list is your recommendation — the developer decides what to implement
