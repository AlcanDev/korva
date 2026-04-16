---
id: nestjs-hexagonal
version: 1.1.0
team: backend
stack: NestJS, TypeScript, Hexagonal Architecture, DDD
---

# Scroll: NestJS Hexagonal Architecture

## Triggers — load when:
- Files: `*.port.ts`, `*.adapter.ts`, `*.adapter.base.ts`, `*.module.ts`, `*.controller.ts`, `*.service.ts`
- Keywords: port, adapter, hexagonal, domain layer, application layer, infrastructure layer, usecase, command
- Tasks: creating a new feature, adding a region adapter, refactoring to hexagonal, wiring a module

## Context
Hexagonal architecture keeps your domain pure and your business rules independent of frameworks, HTTP clients, and databases. Business logic lives in the domain layer, orchestration in the application layer, and I/O (HTTP, DB, cache) in the infrastructure layer. Region-specific behavior (e.g., multi-country or multi-provider deployments) is handled via the Template Method pattern in adapter base classes — never via conditionals in services.

---

## Rules

### 1. Port definition: interface + SCREAMING_SNAKE_CASE token

Every port is an interface plus a constant injection token. Both live in the same file inside `domain/ports/`.

```typescript
// domain/ports/payment.port.ts
export interface PaymentPort {
  getPlans(command: GetPaymentPlansCommand): Promise<PaymentPlan[]>;
  getById(id: PaymentId): Promise<PaymentPlan | null>;
}

export const PAYMENT_PORT = 'PaymentPort';
```

### 2. Service injects the PORT token, never a concrete adapter

```typescript
// application/payment.service.ts
import { Inject, Injectable } from '@nestjs/common';
import { PaymentPort, PAYMENT_PORT } from '../domain/ports/payment.port';
import { GetPaymentPlansCommand } from '../domain/commands/get-payment-plans.command';

@Injectable()
export class PaymentService {
  constructor(
    @Inject(PAYMENT_PORT) private readonly paymentPort: PaymentPort,
  ) {}

  async getPlans(command: GetPaymentPlansCommand): Promise<PaymentPlan[]> {
    return this.paymentPort.getPlans(command);
  }
}
```

### 3. Adapter base: abstract class + Template Method per region

Region-specific logic lives in concrete subclasses. The base class defines the algorithm skeleton.

```typescript
// infrastructure/adapters/stripe-payment.adapter.base.ts
export abstract class StripePaymentAdapterBase implements PaymentPort {
  constructor(protected readonly httpService: HttpService) {}

  async getPlans(command: GetPaymentPlansCommand): Promise<PaymentPlan[]> {
    const url = this.buildPlansUrl(command);
    const headers = this.buildHeaders(command.headers);
    const response = await this.httpService.get<RawPaymentResponse>(url, { headers });
    return this.mapPlans(response.data);
  }

  protected abstract buildPlansUrl(command: GetPaymentPlansCommand): string;
  protected abstract buildHeaders(headers: CommonHeadersRequestDTO): Record<string, string>;
  protected abstract mapPlans(raw: RawPaymentResponse): PaymentPlan[];
}

// infrastructure/adapters/stripe-payment.adapter.us.ts
@Injectable()
export class StripePaymentAdapterUS extends StripePaymentAdapterBase {
  constructor(
    protected readonly httpService: HttpService,
    private readonly configService: ConfigService,
  ) {
    super(httpService);
  }

  protected buildPlansUrl(command: GetPaymentPlansCommand): string {
    const base = this.configService.getOrThrow<string>('STRIPE_BASE_URL');
    return `${base}/us/payments/${command.productId}/plans`;
  }

  protected buildHeaders(headers: CommonHeadersRequestDTO): Record<string, string> {
    return { 'x-region': 'US', 'x-channel': headers.channel };
  }

  protected mapPlans(raw: RawPaymentResponse): PaymentPlan[] {
    return raw.plans.map((p) => ({ id: p.id as PaymentId, name: p.name, price: p.amount }));
  }
}
```

### 4. Controller orchestrates only — zero business logic

```typescript
// infrastructure/controllers/payment.controller.ts
@Controller('payments')
export class PaymentController {
  constructor(private readonly paymentService: PaymentService) {}

  @Get('plans')
  async getPlans(
    @Headers() headers: CommonHeadersRequestDTO,
    @Query() query: GetPaymentPlansQueryDTO,
  ): Promise<PaymentPlansResponseDTO> {
    const command = new GetPaymentPlansCommand(headers, query.productId);
    const plans = await this.paymentService.getPlans(command);
    return PaymentPlansMapper.toResponseDTO(plans);
  }
}
```

### 5. Module wires the region adapter via PORT_TOKEN

```typescript
// payment.module.ts
@Module({
  imports: [HttpModule],
  controllers: [PaymentController],
  providers: [
    PaymentService,
    { provide: PAYMENT_PORT, useClass: StripePaymentAdapterUS },
  ],
})
export class PaymentModule {}
```

### 6. DTO suffix: UPPERCASE

Always `DTO` (not `Dto`). This is a team convention enforced by linting.

```typescript
// CORRECT
export class CommonHeadersRequestDTO { ... }
export class GetPaymentPlansQueryDTO { ... }
export class PaymentPlansResponseDTO { ... }

// WRONG
export class CommonHeadersRequestDto { ... }   // lowercase suffix forbidden
```

### 7. File naming: kebab-case with descriptive suffix

```
stripe-payment.adapter.base.ts
stripe-payment.adapter.us.ts
stripe-payment.adapter.eu.ts
payment.port.ts
payment.service.ts
payment.controller.ts
payment.module.ts
get-payment-plans.command.ts
```

### 8. Layer import constraints

| Layer | Allowed imports | Forbidden imports |
|---|---|---|
| `domain/` | Pure TypeScript | `@nestjs/*`, adapter classes |
| `application/` | domain/, `@nestjs/common` decorators | Adapter classes, HTTP libs |
| `infrastructure/` | application/, domain/, `@nestjs/*` | Cross-app relative imports |

---

## Anti-Patterns

### BAD: Business logic in controller
```typescript
// BAD
@Get('plans')
async getPlans(@Query() query: GetPaymentPlansQueryDTO) {
  if (query.productId.startsWith('PREMIUM')) {
    return this.httpService.get('/premium-plans');  // logic in controller!
  }
  return this.httpService.get('/standard-plans');
}
```

```typescript
// GOOD — controller delegates to service
@Get('plans')
async getPlans(@Headers() headers: CommonHeadersRequestDTO, @Query() query: GetPaymentPlansQueryDTO) {
  const command = new GetPaymentPlansCommand(headers, query.productId);
  return this.paymentService.getPlans(command);
}
```

### BAD: Injecting concrete adapter instead of port
```typescript
// BAD
constructor(private readonly adapter: StripePaymentAdapterUS) {} // tight coupling
```

```typescript
// GOOD
constructor(@Inject(PAYMENT_PORT) private readonly port: PaymentPort) {}
```

### BAD: NestJS import in domain layer
```typescript
// BAD — domain/ports/payment.port.ts
import { Injectable } from '@nestjs/common';  // framework leak into domain

@Injectable()
export interface PaymentPort { ... }
```

```typescript
// GOOD — domain is pure TypeScript
export interface PaymentPort {
  getPlans(command: GetPaymentPlansCommand): Promise<PaymentPlan[]>;
}
```

### BAD: Region conditionals in service or base adapter
```typescript
// BAD
async getPlans(command: GetPaymentPlansCommand) {
  if (command.region === 'US') { return this.getUSPlans(command); }
  if (command.region === 'EU') { return this.getEUPlans(command); }
}
```

```typescript
// GOOD — each region is its own adapter subclass wired at module level
// StripePaymentAdapterUS handles US, StripePaymentAdapterEU handles EU
// The module decides which adapter is injected
```
