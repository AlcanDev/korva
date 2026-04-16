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
TypeScript codebases run with `strict: true` and treat `any` as a build-time error via `@typescript-eslint/no-explicit-any`. Domain errors are modeled as values (Result pattern), never thrown as exceptions — this makes error paths visible to the type system and forces callers to handle them. External API payloads are validated with Zod at the infrastructure boundary before entering the domain, so domain code can trust the types it receives.

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
// domain/value-objects/ids.ts
export type OrderId   = string & { readonly __brand: 'OrderId' };
export type CustomerId = string & { readonly __brand: 'CustomerId' };
export type ProductId  = string & { readonly __brand: 'ProductId' };

export function toOrderId(raw: string): OrderId {
  if (!raw || raw.length === 0) throw new Error('OrderId cannot be empty');
  return raw as OrderId;
}

// Usage — compiler prevents mixing IDs from different domains
function getProduct(id: ProductId): Promise<Product> { ... }
const orderId = toOrderId('ORD-001');
getProduct(orderId);  // Type error: Argument of type 'OrderId' is not assignable to parameter of type 'ProductId'
```

### 3. Discriminated unions for domain states

Model all possible states explicitly. No boolean flags, no nullable fields to represent state.

```typescript
// domain/order.ts
export type Order =
  | { status: 'pending';    id: OrderId; createdAt: Date; items: OrderItem[] }
  | { status: 'processing'; id: OrderId; paymentIntentId: string }
  | { status: 'shipped';    id: OrderId; trackingNumber: string; estimatedDelivery: Date }
  | { status: 'cancelled';  id: OrderId; reason: string; cancelledAt: Date };

function describeOrder(order: Order): string {
  switch (order.status) {
    case 'pending':    return `Order pending — ${order.items.length} items`;
    case 'processing': return `Payment processing — intent ${order.paymentIntentId}`;
    case 'shipped':    return `Shipped — tracking: ${order.trackingNumber}`;
    case 'cancelled':  return `Cancelled: ${order.reason}`;
  }
  // TypeScript ensures exhaustiveness — no default needed
}
```

### 4. satisfies operator for complex config objects

Use `satisfies` to get type-checking without widening the inferred type.

```typescript
// config/payment-providers.config.ts
type Region = 'us' | 'eu' | 'apac';
type ProviderConfig = { baseUrl: string; timeout: number; retries: number };

const PAYMENT_PROVIDERS = {
  us:   { baseUrl: 'https://api.stripe.com', timeout: 5000, retries: 3 },
  eu:   { baseUrl: 'https://api.stripe.com', timeout: 8000, retries: 2 },
  apac: { baseUrl: 'https://api.stripe.com', timeout: 6000, retries: 3 },
} satisfies Record<Region, ProviderConfig>;

// PAYMENT_PROVIDERS.us.baseUrl is inferred as 'https://api.stripe.com', not widened to string
```

### 5. Result<T, E> pattern for domain errors

Never throw exceptions from domain or application layer. Return a typed Result.

```typescript
// shared/result.ts
export type Result<T, E> = { ok: true; value: T } | { ok: false; error: E };

export const ok  = <T>(value: T): Result<T, never>  => ({ ok: true,  value });
export const err = <E>(error: E): Result<never, E>   => ({ ok: false, error });

// domain/errors/order.errors.ts
export type OrderError =
  | { type: 'ORDER_NOT_FOUND';       orderId: OrderId }
  | { type: 'PAYMENT_FAILED';        reason: string }
  | { type: 'INSUFFICIENT_STOCK';    productId: string; requested: number; available: number };

// application/order.service.ts
async placeOrder(command: PlaceOrderCommand): Promise<Result<Order, OrderError>> {
  const stock = await this.inventory.check(command.productId, command.quantity);
  if (!stock.sufficient) {
    return err({ type: 'INSUFFICIENT_STOCK', productId: command.productId, requested: command.quantity, available: stock.count });
  }
  const order = Order.create(command);
  await this.orderRepository.save(order);
  return ok(order);
}
```

### 6. Zod for validating external API responses

Validate at the infrastructure boundary. Never trust external API response shapes — they change without warning.

```typescript
// infrastructure/adapters/schemas/stripe-payment-intent.schema.ts
import { z } from 'zod';

export const StripePaymentIntentSchema = z.object({
  id:            z.string().min(1),
  status:        z.enum(['requires_payment_method', 'requires_confirmation', 'processing', 'succeeded', 'canceled']),
  amount:        z.number().int().positive(),
  currency:      z.string().length(3),
  client_secret: z.string(),
});

export const StripePaymentIntentResponseSchema = z.object({
  data:   z.array(StripePaymentIntentSchema),
  has_more: z.boolean(),
});

export type StripePaymentIntentResponse = z.infer<typeof StripePaymentIntentResponseSchema>;

// In adapter — validate before the data enters the domain
const raw = await this.httpService.get<unknown>(url, { headers });
const parsed = StripePaymentIntentResponseSchema.safeParse(raw.data);

if (!parsed.success) {
  throw new ExternalApiValidationError('Stripe', parsed.error.flatten());
}

return this.mapPaymentIntents(parsed.data);
```

### 7. Type guards over type assertions

Prefer `is` type guards. Use `as` only at trust boundaries where a Zod schema has already validated the data.

```typescript
// GOOD — type guard using a structural check
function isOrder(value: unknown): value is Order {
  return (
    typeof value === 'object' &&
    value !== null &&
    'id' in value &&
    'status' in value
  );
}

// ACCEPTABLE — after Zod validation at the infrastructure boundary
const intent = StripePaymentIntentSchema.parse(raw) as PaymentIntent;

// BAD — blind cast without any validation
const intent = raw as PaymentIntent;  // no validation, runtime error risk
```

### 8. unknown instead of any for unsafe inputs

```typescript
// ❌ BAD — disables type checking downstream
function processApiResponse(data: any) { ... }

// ✅ GOOD — forces explicit validation before use
function processApiResponse(data: unknown): Order[] {
  const parsed = StripePaymentIntentResponseSchema.parse(data);
  return parsed.data.map(mapToOrder);
}
```

---

## Anti-Patterns

### BAD: Optional fields to represent state
```typescript
// ❌ BAD — ambiguous state, requires runtime null checks everywhere
interface Product {
  id: ProductId;
  price?: number;
  unavailableReason?: string;  // nullable flags = implicit states
}
```

```typescript
// ✅ GOOD — explicit discriminated union, compiler enforces handling of each state
type Product =
  | { status: 'available';   id: ProductId; price: number }
  | { status: 'unavailable'; id: ProductId; unavailableReason: string }
```

### BAD: Throwing in domain layer
```typescript
// ❌ BAD — NestJS exception leaking into domain logic
async placeOrder(command: PlaceOrderCommand): Promise<Order[]> {
  const stock = await this.inventory.check(command.productId);
  if (!stock) throw new NotFoundException('Product not found'); // framework leak!
}
```

```typescript
// ✅ GOOD — return Result, let the controller decide the HTTP status
async placeOrder(command: PlaceOrderCommand): Promise<Result<Order, OrderError>> {
  const stock = await this.inventory.check(command.productId);
  if (!stock) return err({ type: 'ORDER_NOT_FOUND', orderId: command.orderId });
  return ok(Order.create(command));
}
```

### BAD: Plain string IDs without branding
```typescript
// ❌ BAD — arguments swapped, no compile error, runtime chaos
function processRefund(orderId: string, customerId: string) { ... }
processRefund(customerId, orderId); // no compiler error — wrong at runtime
```

```typescript
// ✅ GOOD — compiler prevents swapped arguments
function processRefund(orderId: OrderId, customerId: CustomerId) { ... }
processRefund(orderId, customerId); // correct — types enforce order
```

---

## Community Skills

| Skill | Install command |
|---|---|
| [TypeScript Advanced Types](https://skills.sh/wshobson/agents/typescript-advanced-types) | `npx skills add wshobson/agents --skill typescript-advanced-types -a claude-code` |
| [Zod Schema Validation](https://skills.sh/pproenca/dot-skills/zod) | `npx skills add pproenca/dot-skills --skill zod -a claude-code` |
| [React Hook Form](https://skills.sh/pproenca/dot-skills/react-hook-form) | `npx skills add pproenca/dot-skills --skill react-hook-form -a claude-code` |
