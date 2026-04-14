---
id: typescript
version: 1.0.0
team: backend
stack: TypeScript 5, NestJS, Zod, strict mode
---

# Scroll: TypeScript Strict Patterns

## Triggers — load when:
- Files: `*.ts`, `tsconfig.json`, `*.port.ts`, `*.domain.ts`
- Keywords: branded type, discriminated union, Result, satisfies, type guard, unknown, any, Zod, type assertion
- Tasks: modeling a domain entity, validating an external API response, handling errors in domain layer, typing a config object

## Context
Acme Financiero TypeScript codebases run with `strict: true` and treat `any` as a build-time error via `@typescript-eslint/no-explicit-any`. Domain errors are modeled as values (Result pattern), never thrown as exceptions. External API payloads (from EXTERNAL_API, TRO, Apigee) are validated with Zod at the infrastructure boundary before entering the domain.

---

## Rules

### 1. strict: true is mandatory

`tsconfig.json` must always have:

```json
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "noUncheckedIndexedAccess": true
  }
}
```

### 2. Branded types for domain IDs

Prevent accidental mixing of string IDs across domain entities.

```typescript
// domain/value-objects/insurance-id.ts
export type InsuranceId = string & { readonly __brand: 'InsuranceId' };
export type CustomerId = string & { readonly __brand: 'CustomerId' };
export type PolicyId   = string & { readonly __brand: 'PolicyId' };

export function toInsuranceId(raw: string): InsuranceId {
  if (!raw || raw.length === 0) throw new Error('InsuranceId cannot be empty');
  return raw as InsuranceId;
}

// Usage — compiler prevents mixing
function getPolicy(id: PolicyId): Promise<Policy> { ... }
const insId = toInsuranceId('INS-001');
getPolicy(insId);  // Type error: Argument of type 'InsuranceId' is not assignable to parameter of type 'PolicyId'
```

### 3. Discriminated unions for domain states

Model all possible states explicitly. No boolean flags, no nullable fields to represent state.

```typescript
// domain/insurance-offer.ts
export type InsuranceOffer =
  | { status: 'available';    id: InsuranceId; name: string; monthlyPrice: number }
  | { status: 'unavailable';  id: InsuranceId; reason: string }
  | { status: 'pending';      id: InsuranceId; estimatedDate: Date };

function renderOffer(offer: InsuranceOffer): string {
  switch (offer.status) {
    case 'available':    return `${offer.name} — $${offer.monthlyPrice}/month`;
    case 'unavailable':  return `Not available: ${offer.reason}`;
    case 'pending':      return `Available from ${offer.estimatedDate.toISOString()}`;
  }
  // TypeScript ensures exhaustiveness — no default needed
}
```

### 4. satisfies operator for complex config objects

Use `satisfies` to get type-checking without widening the inferred type.

```typescript
// config/insurance-providers.config.ts
type CountryCode = 'CL' | 'PE' | 'CO';
type ProviderConfig = { baseUrl: string; timeout: number; retries: number };

const INSURANCE_PROVIDERS = {
  CL: { baseUrl: 'https://external-api.cl/api', timeout: 5000, retries: 3 },
  PE: { baseUrl: 'https://external-api.pe/api', timeout: 8000, retries: 2 },
  CO: { baseUrl: 'https://external-api.co/api', timeout: 6000, retries: 3 },
} satisfies Record<CountryCode, ProviderConfig>;

// INSURANCE_PROVIDERS.CL.baseUrl is inferred as string literal, not widened
```

### 5. Result<T, E> pattern for domain errors

Never throw exceptions from domain or application layer. Return a typed Result.

```typescript
// shared/result.ts
export type Result<T, E> = { ok: true; value: T } | { ok: false; error: E };

export const ok  = <T>(value: T): Result<T, never>  => ({ ok: true,  value });
export const err = <E>(error: E): Result<never, E>   => ({ ok: false, error });

// domain/errors/insurance.errors.ts
export type InsuranceError =
  | { type: 'OFFER_NOT_FOUND';      insuranceId: InsuranceId }
  | { type: 'PROVIDER_UNAVAILABLE'; country: string }
  | { type: 'INVALID_PRODUCT';      productId: string };

// application/insurance.service.ts
async getOffers(command: GetInsuranceOffersCommand): Promise<Result<InsuranceOffer[], InsuranceError>> {
  const offers = await this.insurancePort.getOffers(command);
  if (offers.length === 0) {
    return err({ type: 'OFFER_NOT_FOUND', insuranceId: command.insuranceId });
  }
  return ok(offers);
}
```

### 6. Zod for validating external API responses

Validate at the infrastructure boundary. Do not trust EXTERNAL_API/TRO/Apigee response shapes.

```typescript
// infrastructure/adapters/schemas/external-api-offers.schema.ts
import { z } from 'zod';

export const ExternalOfferSchema = z.object({
  id:        z.string().min(1),
  name:      z.string(),
  premium:   z.number().positive(),
  available: z.boolean(),
});

export const ExternalOffersResponseSchema = z.object({
  offers: z.array(ExternalOfferSchema),
  total:  z.number().int().nonnegative(),
});

export type ExternalOffersResponse = z.infer<typeof ExternalOffersResponseSchema>;

// In adapter
const raw = await this.httpService.get<unknown>(url, { headers });
const parsed = ExternalOffersResponseSchema.safeParse(raw.data);

if (!parsed.success) {
  throw new ExternalApiValidationError('EXTERNAL_API', parsed.error.flatten());
}

return this.mapOffers(parsed.data);
```

### 7. Type guards over type assertions

Prefer `is` type guards. Use `as` only at trust boundaries where a Zod schema has already validated the data.

```typescript
// GOOD — type guard
function isInsuranceOffer(value: unknown): value is InsuranceOffer {
  return (
    typeof value === 'object' &&
    value !== null &&
    'id' in value &&
    'status' in value
  );
}

// ACCEPTABLE — after Zod validation
const offer = ExternalOfferSchema.parse(raw) as InsuranceOffer;

// BAD — blind cast
const offer = raw as InsuranceOffer;  // no validation, runtime error risk
```

### 8. unknown instead of any for unsafe inputs

```typescript
// BAD
function processApiResponse(data: any) { ... }

// GOOD
function processApiResponse(data: unknown): InsuranceOffer[] {
  const parsed = ExternalOffersResponseSchema.parse(data);
  return parsed.offers.map(mapToInsuranceOffer);
}
```

---

## Anti-Patterns

### BAD: Optional fields to represent state
```typescript
// BAD — ambiguous state
interface InsuranceOffer {
  id: InsuranceId;
  price?: number;
  unavailableReason?: string;  // nullable flags = implicit states
}
```

```typescript
// GOOD — explicit discriminated union
type InsuranceOffer =
  | { status: 'available';   id: InsuranceId; price: number }
  | { status: 'unavailable'; id: InsuranceId; unavailableReason: string }
```

### BAD: Throwing in domain layer
```typescript
// BAD
async getOffers(command: GetInsuranceOffersCommand): Promise<InsuranceOffer[]> {
  const offers = await this.port.getOffers(command);
  if (!offers.length) throw new NotFoundException('No offers'); // NestJS exception in domain!
}
```

```typescript
// GOOD — return Result, let the controller decide the HTTP status
async getOffers(command: GetInsuranceOffersCommand): Promise<Result<InsuranceOffer[], InsuranceError>> {
  const offers = await this.port.getOffers(command);
  if (!offers.length) return err({ type: 'OFFER_NOT_FOUND', insuranceId: command.insuranceId });
  return ok(offers);
}
```

### BAD: Plain string IDs without branding
```typescript
// BAD
function cancelPolicy(policyId: string, insuranceId: string) { ... }
cancelPolicy(insuranceId, policyId); // arguments swapped — no compile error
```

```typescript
// GOOD
function cancelPolicy(policyId: PolicyId, insuranceId: InsuranceId) { ... }
cancelPolicy(policyId, insuranceId); // correct — compiler enforces order
```
