# Phase 5 — Verification

**Goal:** Confirm the implementation matches the spec. No new code — only validation.

## Actions

1. **Spec review:** Go through each item in Phase 2 spec. Mark pass or fail.
2. **Anti-pattern scan:** Check for Sentinel violations against the active rule set.
3. **Test list:** Enumerate the tests that should be written (do NOT generate them unless asked).
4. **Vault save:** Record what was learned.

## Sentinel validation

If Sentinel is configured for this project, the output format will look like:

```
sentinel: scanning 4 files...

  PASS  src/modules/payments/domain/ports/payment.port.ts
  PASS  src/modules/payments/application/services/payment.service.ts
  WARN  src/modules/payments/infrastructure/adapters/stripe-payment.adapter.us.ts
        HEX-002: process.env accessed directly — use ConfigService
  PASS  src/modules/payments/payment.module.ts

summary: 3 passed, 1 warning, 0 errors
```

Incorporate Sentinel output into the verification checklist. Warnings must be acknowledged; errors must be resolved before closing.

## Output Format

```
## Verification: [feature name]

### Spec checklist:
[PASS] Objective: GetPaymentPlansCommand implemented
[PASS] Input types: correct TypeScript types
[PASS] Output types: PaymentPlan[] matches spec
[PASS] Business rule 1: cache key includes region
[FAIL] Business rule 2: rate limiting NOT implemented (not in scope for this PR)

### Anti-pattern check:
[PASS] No framework imports in domain/
[PASS] Application layer only depends on PaymentPort interface
[PASS] No console.log in src/
[PASS] DTO uses uppercase suffix
[WARN] HEX-002: direct process.env in stripe-payment.adapter.us.ts — flagged for follow-up

### Tests to write:
1. PaymentService.getPlans: mock PaymentPort, verify cache miss → fetch → cache set
2. StripePaymentAdapterUS.getPlans: mock HttpService, verify URL and headers
3. PaymentController.getPlans: verify delegation to service, no logic in controller

### Saved to Vault:
- vault_save type=decision: "Template Method for region adapters with abstract base class"
- vault_save type=pattern: "PaymentPort injection via @Inject(PAYMENT_PORT)"
```

## Rules

- Be honest about what is missing — incomplete implementations should be flagged
- Never mark PASS for something you are unsure about
- The test list is your recommendation — the developer decides what to implement
- Always run vault_save before closing, even for small changes
