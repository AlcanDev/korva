---
id: testing-jest
version: 2.0.0
team: backend
stack: Jest, NestJS, TypeScript, ts-jest
last_updated: 2026-04-30
---

# Scroll: Testing Patterns with Jest

## Triggers — load when:
- Files: `*.spec.ts`, `jest.config.js`, `jest.config.ts`, `__fixtures__/**`
- Keywords: jest, describe, it, beforeEach, mock, spy, coverage, fixture, AAA, arrange act assert, port mock
- Tasks: writing a unit test, mocking a port, testing an adapter, configuring coverage thresholds

## Context
Tests mock the PORT interface — never a concrete adapter. The test file lives co-located with the file it tests (`payment.service.spec.ts` next to `payment.service.ts`). External data (API responses, DB fixtures) lives in typed `__fixtures__/` constants — never as anonymous inline objects. The test database rule is absolute: if a test requires database access, it is mocking the wrong thing — mock the port interface instead.

---

## Rules

### 1. Co-located spec files

Place the `.spec.ts` file directly next to the file under test:

```
src/
  application/
    payment.service.ts
    payment.service.spec.ts          ← co-located
  infrastructure/
    adapters/
      stripe-payment.adapter.ts
      stripe-payment.adapter.spec.ts
      __fixtures__/
        stripe-payment-intent.fixture.ts
        stripe-payment-intent.empty.fixture.ts
```

### 2. Fixtures in `__fixtures__/` directory

Fixtures are typed constant objects — never inline anonymous objects in test files. Fixtures are reused across multiple tests: change the fixture once, all tests using it update automatically.

```typescript
// infrastructure/adapters/__fixtures__/stripe-payment-intent.fixture.ts
import { StripePaymentIntentResponse } from '../schemas/stripe-payment-intent.schema';

export const STRIPE_PAYMENT_INTENT_FIXTURE: StripePaymentIntentResponse = {
  id: 'pi_3NyKxLBC1234567890',
  status: 'requires_payment_method',
  amount: 2999,
  currency: 'usd',
  client_secret: 'pi_3NyKxLBC_secret_abc123',
};

export const STRIPE_PAYMENT_INTENT_SUCCEEDED_FIXTURE: StripePaymentIntentResponse = {
  ...STRIPE_PAYMENT_INTENT_FIXTURE,
  status: 'succeeded',
};
```

### 3. Mock the PORT interface, never a concrete adapter

```typescript
// application/payment.service.spec.ts
import { Test, TestingModule } from '@nestjs/testing';
import { PaymentPort, PAYMENT_PORT } from '../domain/ports/payment.port';
import { PaymentService } from './payment.service';
import { PAYMENT_PLANS_FIXTURE } from './__fixtures__/payment-plans.fixture';

describe('PaymentService', () => {
  let service: PaymentService;
  let portMock: jest.Mocked<PaymentPort>;

  beforeEach(async () => {
    portMock = {
      getPlans:      jest.fn(),
      createIntent:  jest.fn(),
      confirmIntent: jest.fn(),
      refund:        jest.fn(),
    };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        PaymentService,
        { provide: PAYMENT_PORT, useValue: portMock },
      ],
    }).compile();

    service = module.get<PaymentService>(PaymentService);
  });

  describe('getPlans', () => {
    describe('when the provider returns plans', () => {
      it('should return mapped payment plans', async () => {
        // Arrange
        portMock.getPlans.mockResolvedValue(PAYMENT_PLANS_FIXTURE);
        const command = new GetPaymentPlansCommand({ productId: 'PROD-001' });

        // Act
        const result = await service.getPlans(command);

        // Assert
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toHaveLength(2);
          expect(portMock.getPlans).toHaveBeenCalledWith(command);
        }
      });
    });

    describe('when the provider returns no plans', () => {
      it('should return a PLAN_NOT_FOUND error', async () => {
        // Arrange
        portMock.getPlans.mockResolvedValue([]);
        const command = new GetPaymentPlansCommand({ productId: 'PROD-UNKNOWN' });

        // Act
        const result = await service.getPlans(command);

        // Assert
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error.type).toBe('PLAN_NOT_FOUND');
        }
      });
    });
  });
});
```

### 4. Adapter tests: mock the HTTP client, not the port

```typescript
// infrastructure/adapters/stripe-payment.adapter.spec.ts
import { HttpService } from '@nestjs/axios';
import { ConfigService } from '@nestjs/config';
import { StripePaymentAdapter } from './stripe-payment.adapter';
import { STRIPE_PAYMENT_INTENT_FIXTURE } from './__fixtures__/stripe-payment-intent.fixture';

describe('StripePaymentAdapter', () => {
  let adapter: StripePaymentAdapter;
  let httpMock: jest.Mocked<HttpService>;
  let configMock: jest.Mocked<ConfigService>;

  beforeEach(() => {
    httpMock = {
      get:  jest.fn(),
      post: jest.fn(),
    } as unknown as jest.Mocked<HttpService>;

    configMock = {
      get:         jest.fn(),
      getOrThrow:  jest.fn(),
    } as unknown as jest.Mocked<ConfigService>;

    configMock.getOrThrow.mockReturnValue('https://api.stripe.com');

    adapter = new StripePaymentAdapter(httpMock, configMock);
  });

  describe('createIntent', () => {
    it('should call Stripe with the correct endpoint and idempotency key', async () => {
      // Arrange
      httpMock.post.mockResolvedValue({ data: STRIPE_PAYMENT_INTENT_FIXTURE });
      const command = new CreatePaymentCommand({
        amount: Money.of(2999, 'USD'),
        orderId: 'ord_123',
        idempotencyKey: 'idem-key-abc',
      });

      // Act
      const result = await adapter.createIntent(command);

      // Assert
      expect(httpMock.post).toHaveBeenCalledWith(
        'https://api.stripe.com/v1/payment_intents',
        expect.objectContaining({ amount: 2999, currency: 'usd' }),
        expect.objectContaining({ headers: expect.objectContaining({ 'Idempotency-Key': 'idem-key-abc' }) }),
      );
      expect(result.id).toBe('pi_3NyKxLBC1234567890');
    });
  });
});
```

### 5. Test file structure: describe > describe > it (AAA)

```
describe('<ClassName>')
  describe('<methodName>')
    describe('when <context/condition>')
      it('should <expected behavior>')
```

Each `it` block follows Arrange–Act–Assert with a blank line between sections (as shown in the examples above). Never mix setup and assertions in the same block.

### 6. Coverage thresholds in jest.config.ts

```typescript
// jest.config.ts
export default {
  moduleFileExtensions: ['js', 'json', 'ts'],
  rootDir: 'src',
  testRegex: '.*\\.spec\\.ts$',
  transform: { '^.+\\.(t|j)s$': 'ts-jest' },
  collectCoverageFrom: ['**/*.(t|j)s', '!**/*.module.ts', '!**/main.ts'],
  coverageDirectory: '../coverage',
  testEnvironment: 'node',
  coverageThresholds: {
    global: {
      functions:  90,
      lines:      90,
      statements: 90,
      branches:   80,
    },
  },
};
```

### 7. Type-safe port mocks with `jest.Mocked<T>`

Always declare the mock variable with the port interface type so TypeScript validates mock setup and catches typos in method names.

```typescript
// ✅ CORRECT — TypeScript validates mock method names at compile time
let portMock: jest.Mocked<PaymentPort>;
portMock = {
  getPlans:      jest.fn(),
  createIntent:  jest.fn(),
  confirmIntent: jest.fn(),
  refund:        jest.fn(),
};

// Assertions are type-safe — typo in method name is a compile error
expect(portMock.getPlans).toHaveBeenCalledWith(command);
```

---

## Anti-Patterns

### BAD: Injecting the concrete adapter in service tests

```typescript
// ❌ BAD — test is coupled to Stripe; swap providers → rewrite test
const module = await Test.createTestingModule({
  providers: [PaymentService, StripePaymentAdapter], // tight coupling
}).compile();
```

```typescript
// ✅ GOOD — mock the port interface; provider swap doesn't touch this test
const module = await Test.createTestingModule({
  providers: [
    PaymentService,
    { provide: PAYMENT_PORT, useValue: portMock },
  ],
}).compile();
```

### BAD: Inline anonymous objects as test data

```typescript
// ❌ BAD — fragile, not reusable, not typed
portMock.getPlans.mockResolvedValue([
  { id: 'plan-001', name: 'Basic', price: 999, available: true },
]);
```

```typescript
// ✅ GOOD — named, typed, reusable across tests
import { PAYMENT_PLANS_FIXTURE } from './__fixtures__/payment-plans.fixture';
portMock.getPlans.mockResolvedValue(PAYMENT_PLANS_FIXTURE);
```

### BAD: Mixed Arrange / Act / Assert

```typescript
// ❌ BAD — setup is tangled with assertions
it('should work', async () => {
  portMock.getPlans.mockResolvedValue(FIXTURE);
  const result = await service.getPlans(command);
  portMock.getPlans.mockResolvedValue([]); // setup mixed in after act
  expect(result.ok).toBe(true);
});
```

```typescript
// ✅ GOOD — clear Arrange / Act / Assert sections
it('should return mapped plans', async () => {
  // Arrange
  portMock.getPlans.mockResolvedValue(PAYMENT_PLANS_FIXTURE);
  const command = new GetPaymentPlansCommand({ productId: 'PROD-001' });

  // Act
  const result = await service.getPlans(command);

  // Assert
  expect(result.ok).toBe(true);
  if (result.ok) expect(result.value).toHaveLength(2);
});
```

### BAD: Testing with a live database

```typescript
// ❌ BAD — slow, non-deterministic, environment-dependent
const module = await Test.createTestingModule({
  imports: [TypeOrmModule.forRoot({ ...productionConfig })],
  providers: [PaymentService],
}).compile();
```

```typescript
// ✅ GOOD — if you need a DB, use an in-memory SQLite or mock the repository port
// Option 1: mock the repository interface (preferred for unit tests)
const repoMock: jest.Mocked<PaymentRepository> = {
  findById: jest.fn(),
  save: jest.fn(),
};
// Option 2: TypeORM in-memory DB (for integration tests only)
TypeOrmModule.forRoot({ type: 'sqlite', database: ':memory:', synchronize: true })

---

## Community Skills

| Skill | Install command |
|---|---|
| [Playwright Best Practices](https://skills.sh/currents-dev/playwright-best-practices-skill/playwright-best-practices) | `npx skills add currents-dev/playwright-best-practices-skill --skill playwright-best-practices -a claude-code` |
| [Vitest](https://skills.sh/antfu/skills/vitest) | `npx skills add antfu/skills --skill vitest -a claude-code` |
| [Python Testing Patterns](https://skills.sh/wshobson/agents/python-testing-patterns) | `npx skills add wshobson/agents --skill python-testing-patterns -a claude-code` |
```
