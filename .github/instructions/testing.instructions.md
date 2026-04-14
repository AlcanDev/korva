---
applyTo: "**/*.spec.ts,**/*.test.ts,**/*.e2e.ts,e2e/**,test/**,cypress/**,playwright/**"
---

# Testing — Unit, Integration, E2E, Contract

## Test pyramid (mandatory coverage targets)

| Layer | Tool | Coverage target | What to test |
|-------|------|----------------|-------------|
| Unit | Jest | 80%+ branches | Domain logic, pure functions, value objects |
| Integration | Jest + Testing Module | 70%+ | Service + port interactions |
| E2E | Playwright / Supertest | Critical paths | Full HTTP flows, auth |
| Contract | Pact | All provider/consumer pairs | API contracts between BFFs |

## Unit tests — domain layer (zero framework setup)

```typescript
// insurance.command.spec.ts
describe('GetInsuranceOffersCommand', () => {
  describe('fromRequest()', () => {
    it('builds command from valid headers', () => {
      const cmd = GetInsuranceOffersCommand.fromRequest({
        country: 'CL', commerce: 'BANCO', channel: 'Web',
      });
      expect(cmd.country).toBe('CL');
    });

    it('throws InvalidCountryError for unknown country', () => {
      expect(() => GetInsuranceOffersCommand.fromRequest({ country: 'XX' }))
        .toThrow(InvalidCountryError);
    });

    it('throws for all missing required fields', () => {
      expect(() => GetInsuranceOffersCommand.fromRequest({}))
        .toThrow();
    });
  });
});
```

**Unit test rules:**
- No `describe.skip` or `it.skip` without a Jira ticket comment
- Arrange → Act → Assert: one assert per concept (multiple `expect()` per test is OK if they form one concept)
- Test edge cases, not just happy path (null, empty, boundary values)
- No test should depend on another test's state

## Integration tests — service with mocked port

```typescript
describe('InsuranceService', () => {
  let service: InsuranceService;
  let portMock: jest.Mocked<InsurancePort>;
  let loggerMock: jest.Mocked<Logger>;

  beforeEach(async () => {
    portMock = { getOffers: jest.fn(), getById: jest.fn() };
    loggerMock = { error: jest.fn(), log: jest.fn(), warn: jest.fn() } as any;

    const module = await Test.createTestingModule({
      providers: [
        InsuranceService,
        { provide: INSURANCE_PORT, useValue: portMock },
        { provide: Logger, useValue: loggerMock },
      ],
    }).compile();

    service = module.get(InsuranceService);
  });

  it('returns transformed offers from port', async () => {
    portMock.getOffers.mockResolvedValue(InsuranceOfferFactory.buildList(3));
    const result = await service.getOffers(validCommand);
    expect(result).toHaveLength(3);
    expect(portMock.getOffers).toHaveBeenCalledWith(validCommand);
  });

  it('re-throws domain errors unchanged', async () => {
    portMock.getOffers.mockRejectedValue(new InsuranceNotFoundError(new InsuranceId('123')));
    await expect(service.getOffers(validCommand)).rejects.toThrow(InsuranceNotFoundError);
  });

  it('wraps infrastructure errors in domain error', async () => {
    portMock.getOffers.mockRejectedValue(new Error('Network timeout'));
    await expect(service.getOffers(validCommand)).rejects.toThrow(InsuranceUnavailableError);
    expect(loggerMock.error).toHaveBeenCalled();
  });
});
```

## Fixtures — centralized test data factory

```typescript
// __fixtures__/insurance-offer.factory.ts
import { faker } from '@faker-js/faker';
import { InsuranceOffer } from '../../src/domain/entities/insurance-offer.entity';

export class InsuranceOfferFactory {
  static build(overrides: Partial<InsuranceOffer> = {}): InsuranceOffer {
    return InsuranceOffer.create({
      id: faker.string.uuid(),
      name: faker.commerce.productName(),
      price: faker.number.float({ min: 10, max: 500, fractionDigits: 2 }),
      currency: 'CLP',
      country: 'CL',
      ...overrides,
    });
  }

  static buildList(count: number, overrides: Partial<InsuranceOffer> = []) {
    return Array.from({ length: count }, (_, i) =>
      this.build(Array.isArray(overrides) ? overrides[i] ?? {} : overrides),
    );
  }
}
```

## E2E tests — Supertest for HTTP endpoints

```typescript
describe('GET /insurance/v1/offers (e2e)', () => {
  let app: INestApplication;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      imports: [AppModule],
    })
      .overrideProvider(INSURANCE_PORT)
      .useValue({ getOffers: jest.fn().mockResolvedValue(InsuranceOfferFactory.buildList(3)) })
      .compile();

    app = moduleRef.createNestApplication();
    app.useGlobalPipes(new ValidationPipe({ transform: true }));
    await app.init();
  });

  afterAll(() => app.close());

  it('200 with valid headers', () => {
    return request(app.getHttpServer())
      .get('/insurance/v1/offers')
      .set('X-Country', 'CL')
      .set('X-Commerce', 'BANCO')
      .set('Authorization', 'Bearer mock-token')
      .expect(200)
      .expect(res => {
        expect(res.body.offers).toHaveLength(3);
        expect(res.body.offers[0]).toHaveProperty('id');
      });
  });

  it('400 when X-Country missing', () => {
    return request(app.getHttpServer())
      .get('/insurance/v1/offers')
      .set('Authorization', 'Bearer mock-token')
      .expect(400);
  });
});
```

## Playwright — E2E browser tests

```typescript
// e2e/insurance-flow.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Insurance offers flow', () => {
  test.beforeEach(async ({ page }) => {
    // Mock API responses — never depend on real Apigee in E2E
    await page.route('**/insurance/v1/offers', route =>
      route.fulfill({ json: { offers: InsuranceOfferFactory.buildList(3) } })
    );
    await page.goto('/insurance');
  });

  test('displays offer cards', async ({ page }) => {
    await expect(page.getByTestId('offer-card')).toHaveCount(3);
  });

  test('selects offer and proceeds', async ({ page }) => {
    await page.getByTestId('offer-card').first().click();
    await expect(page.getByTestId('continue-btn')).toBeEnabled();
  });

  test('shows error state on API failure', async ({ page }) => {
    await page.route('**/insurance/v1/offers', route => route.fulfill({ status: 500 }));
    await page.reload();
    await expect(page.getByRole('alert')).toBeVisible();
  });
});
```

## Coverage configuration (jest.config.ts)

```typescript
export default {
  collectCoverageFrom: [
    'src/**/*.ts',
    '!src/**/*.spec.ts',
    '!src/**/*.dto.ts',     // DTOs are validated by class-validator
    '!src/main.ts',
    '!src/**/index.ts',
  ],
  coverageThresholds: {
    global: {
      branches: 80,
      functions: 80,
      lines: 85,
      statements: 85,
    },
  },
};
```

## Forbidden patterns

```typescript
// ❌ Testing implementation details
expect(service['_privateMethod']).toHaveBeenCalled();

// ❌ Relying on test execution order
// ❌ Real HTTP calls in unit/integration tests — mock always
// ❌ Real Apigee tokens in E2E tests
// ❌ Skipping tests without Jira ticket — describe.skip('TODO')
// ❌ Empty test bodies that always pass
it('works', () => {});
// ❌ Snapshots for business logic — use explicit assertions
```
