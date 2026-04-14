---
id: testing-jest
version: 1.0.0
team: backend
stack: Jest, NestJS, TypeScript, ts-jest
---

# Scroll: Testing Patterns with Jest

## Triggers — load when:
- Files: `*.spec.ts`, `jest.config.js`, `jest.config.ts`, `__fixtures__/**`
- Keywords: jest, describe, it, beforeEach, mock, spy, coverage, fixture, AAA, arrange act assert, port mock
- Tasks: writing a unit test, mocking a port, testing an adapter, configuring coverage thresholds

## Context
All backend services at Falabella Financiero follow a co-located spec convention: the test file lives next to the production file. Tests mock the PORT interface (never a concrete adapter), use fixtures stored in `__fixtures__/` directories, and must meet 90% coverage thresholds. The test database rule is absolute — if a test requires a database, it mocks the port instead.

---

## Rules

### 1. Co-located spec files

Place the `.spec.ts` file directly next to the file under test:

```
src/
  application/
    insurance.service.ts
    insurance.service.spec.ts       <- co-located
  infrastructure/
    adapters/
      life-insurance.adapter.cl.ts
      life-insurance.adapter.cl.spec.ts
      __fixtures__/
        cigo-offers-response.fixture.ts
        cigo-offers-response.empty.fixture.ts
```

### 2. Fixtures in __fixtures__/ directory

Fixtures are typed constant objects — never inline anonymous objects in test files.

```typescript
// infrastructure/adapters/__fixtures__/cigo-offers-response.fixture.ts
import { CigoOffersResponse } from '../schemas/cigo-offers.schema';

export const CIGO_OFFERS_FIXTURE: CigoOffersResponse = {
  offers: [
    { id: 'INS-001', name: 'Seguro de Vida Básico', premium: 15990, available: true },
    { id: 'INS-002', name: 'Seguro de Vida Plus',   premium: 29990, available: true },
  ],
  total: 2,
};

export const CIGO_OFFERS_EMPTY_FIXTURE: CigoOffersResponse = {
  offers: [],
  total: 0,
};
```

### 3. Mock the PORT interface, never a concrete adapter

```typescript
// application/insurance.service.spec.ts
import { Test, TestingModule } from '@nestjs/testing';
import { InsurancePort, INSURANCE_PORT } from '../domain/ports/insurance.port';
import { InsuranceService } from './insurance.service';
import { INSURANCE_OFFERS_FIXTURE } from './__fixtures__/insurance-offers.fixture';

describe('InsuranceService', () => {
  let service: InsuranceService;
  let portMock: jest.Mocked<InsurancePort>;

  beforeEach(async () => {
    portMock = {
      getOffers:  jest.fn(),
      getById:    jest.fn(),
    };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        InsuranceService,
        { provide: INSURANCE_PORT, useValue: portMock },
      ],
    }).compile();

    service = module.get<InsuranceService>(InsuranceService);
  });

  describe('getOffers', () => {
    describe('when the provider returns offers', () => {
      it('should return mapped insurance offers', async () => {
        // Arrange
        portMock.getOffers.mockResolvedValue(INSURANCE_OFFERS_FIXTURE);
        const command = new GetInsuranceOffersCommand(COMMON_HEADERS_FIXTURE, 'PROD-001');

        // Act
        const result = await service.getOffers(command);

        // Assert
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toHaveLength(2);
          expect(portMock.getOffers).toHaveBeenCalledWith(command);
        }
      });
    });

    describe('when the provider returns no offers', () => {
      it('should return an OFFER_NOT_FOUND error', async () => {
        // Arrange
        portMock.getOffers.mockResolvedValue([]);
        const command = new GetInsuranceOffersCommand(COMMON_HEADERS_FIXTURE, 'PROD-999');

        // Act
        const result = await service.getOffers(command);

        // Assert
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error.type).toBe('OFFER_NOT_FOUND');
        }
      });
    });
  });
});
```

### 4. Adapter tests: mock FifHttpService

```typescript
// infrastructure/adapters/life-insurance.adapter.cl.spec.ts
import { FifHttpService } from '@df-libs/rest-client';
import { ConfigService } from '@nestjs/config';
import { LifeInsuranceAdapterCL } from './life-insurance.adapter.cl';
import { CIGO_OFFERS_FIXTURE } from './__fixtures__/cigo-offers-response.fixture';

describe('LifeInsuranceAdapterCL', () => {
  let adapter: LifeInsuranceAdapterCL;
  let httpMock: jest.Mocked<FifHttpService>;
  let configMock: jest.Mocked<ConfigService>;

  beforeEach(() => {
    httpMock = { get: jest.fn(), post: jest.fn() } as unknown as jest.Mocked<FifHttpService>;
    configMock = { get: jest.fn(), getOrThrow: jest.fn() } as unknown as jest.Mocked<ConfigService>;
    configMock.get.mockReturnValue('https://cigo.cl/api');

    adapter = new LifeInsuranceAdapterCL(httpMock, configMock);
  });

  describe('getOffers', () => {
    it('should call CIGO with the correct URL and country header', async () => {
      // Arrange
      httpMock.get.mockResolvedValue({ data: CIGO_OFFERS_FIXTURE });
      const command = new GetInsuranceOffersCommand(COMMON_HEADERS_FIXTURE, 'LIFE-001');

      // Act
      await adapter.getOffers(command);

      // Assert
      expect(httpMock.get).toHaveBeenCalledWith(
        'https://cigo.cl/api/cl/v2/insurances/LIFE-001/offers',
        expect.objectContaining({ headers: expect.objectContaining({ 'x-country': 'CL' }) }),
      );
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

Each `it` block follows Arrange–Act–Assert with a blank line between sections (as shown in the examples above).

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

### 7. Type-safe port mocks with jest.Mocked<T>

Always declare the mock variable with `jest.Mocked<PortInterface>` so TypeScript validates mock setup.

```typescript
// CORRECT
let portMock: jest.Mocked<InsurancePort>;
portMock = { getOffers: jest.fn(), getById: jest.fn() };

// Then assertions are type-safe
expect(portMock.getOffers).toHaveBeenCalledWith(command);
```

---

## Anti-Patterns

### BAD: Testing the concrete adapter in service tests
```typescript
// BAD — tight coupling to infrastructure
const module = await Test.createTestingModule({
  providers: [InsuranceService, LifeInsuranceAdapterCL],  // concrete class
}).compile();
```

```typescript
// GOOD — mock the port
const module = await Test.createTestingModule({
  providers: [
    InsuranceService,
    { provide: INSURANCE_PORT, useValue: portMock },
  ],
}).compile();
```

### BAD: Inline anonymous objects as test data
```typescript
// BAD — anonymous inline fixture
portMock.getOffers.mockResolvedValue([
  { id: 'INS-001', name: 'Seguro', price: 9990, status: 'available' },
]);
```

```typescript
// GOOD — named fixture from __fixtures__/
import { INSURANCE_OFFERS_FIXTURE } from './__fixtures__/insurance-offers.fixture';
portMock.getOffers.mockResolvedValue(INSURANCE_OFFERS_FIXTURE);
```

### BAD: Assertions without AAA structure
```typescript
// BAD — mixed setup and assertions
it('should work', async () => {
  portMock.getOffers.mockResolvedValue(FIXTURE);
  const result = await service.getOffers(command);
  portMock.getOffers.mockResolvedValue([]);  // setup mixed in after act
  expect(result.ok).toBe(true);
});
```

```typescript
// GOOD — clear Arrange / Act / Assert sections
it('should return mapped offers', async () => {
  // Arrange
  portMock.getOffers.mockResolvedValue(INSURANCE_OFFERS_FIXTURE);
  const command = new GetInsuranceOffersCommand(HEADERS_FIXTURE, 'PROD-001');

  // Act
  const result = await service.getOffers(command);

  // Assert
  expect(result.ok).toBe(true);
});
```
