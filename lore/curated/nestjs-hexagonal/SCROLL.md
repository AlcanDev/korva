---
id: nestjs-hexagonal
version: 1.0.0
team: backend
stack: NestJS, TypeScript, Hexagonal Architecture, DDD
---

# Scroll: NestJS Hexagonal Architecture

## Triggers — load when:
- Files: `*.port.ts`, `*.adapter.ts`, `*.adapter.base.ts`, `*.module.ts`, `*.controller.ts`, `*.service.ts`
- Keywords: port, adapter, hexagonal, domain layer, application layer, infrastructure layer, usecase, command
- Tasks: creating a new feature, adding a country adapter, refactoring to hexagonal, wiring a module

## Context
All BFF APIs at Acme Financiero follow strict hexagonal architecture. Business logic lives in the domain layer, orchestration in the application layer, and I/O (HTTP, DB, cache) in the infrastructure layer. Country-specific behavior (CL, PE, CO) is handled via the Template Method pattern in adapter base classes — never via conditionals in services.

---

## Rules

### 1. Port definition: interface + SCREAMING_SNAKE_CASE token

Every port is an interface plus a constant injection token. Both live in the same file inside `domain/ports/`.

```typescript
// domain/ports/insurance.port.ts
export interface InsurancePort {
  getOffers(command: GetInsuranceOffersCommand): Promise<InsuranceOffer[]>;
  getById(id: InsuranceId): Promise<InsuranceOffer | null>;
}

export const INSURANCE_PORT = 'InsurancePort';
```

### 2. Service injects the PORT token, never a concrete adapter

```typescript
// application/insurance.service.ts
import { Inject, Injectable } from '@nestjs/common';
import { InsurancePort, INSURANCE_PORT } from '../domain/ports/insurance.port';
import { GetInsuranceOffersCommand } from '../domain/commands/get-insurance-offers.command';

@Injectable()
export class InsuranceService {
  constructor(
    @Inject(INSURANCE_PORT) private readonly insurancePort: InsurancePort,
  ) {}

  async getOffers(command: GetInsuranceOffersCommand): Promise<InsuranceOffer[]> {
    return this.insurancePort.getOffers(command);
  }
}
```

### 3. Adapter base: abstract class + Template Method per country

Country-specific logic lives in concrete subclasses. The base class defines the algorithm skeleton.

```typescript
// infrastructure/adapters/life-insurance.adapter.base.ts
export abstract class LifeInsuranceAdapterBase implements InsurancePort {
  constructor(protected readonly httpService: HttpService) {}

  async getOffers(command: GetInsuranceOffersCommand): Promise<InsuranceOffer[]> {
    const url = this.buildOffersUrl(command);
    const headers = this.buildHeaders(command.headers);
    const response = await this.httpService.get<RawInsuranceResponse>(url, { headers });
    return this.mapOffers(response.data);
  }

  protected abstract buildOffersUrl(command: GetInsuranceOffersCommand): string;
  protected abstract buildHeaders(headers: CommonHeadersRequestDTO): Record<string, string>;
  protected abstract mapOffers(raw: RawInsuranceResponse): InsuranceOffer[];
}

// infrastructure/adapters/life-insurance.adapter.cl.ts
@Injectable()
export class LifeInsuranceAdapterCL extends LifeInsuranceAdapterBase {
  protected buildOffersUrl(command: GetInsuranceOffersCommand): string {
    return `${process.env.EXTERNAL_API_BASE_URL}/cl/insurances/${command.productId}/offers`;
  }

  protected buildHeaders(headers: CommonHeadersRequestDTO): Record<string, string> {
    return { 'x-country': 'CL', 'x-channel': headers.channel };
  }

  protected mapOffers(raw: RawInsuranceResponse): InsuranceOffer[] {
    return raw.offers.map((o) => ({ id: o.id as InsuranceId, name: o.name, price: o.premium }));
  }
}
```

### 4. Controller orchestrates only — zero business logic

```typescript
// infrastructure/controllers/insurance.controller.ts
@Controller('insurances')
export class InsuranceController {
  constructor(private readonly insuranceService: InsuranceService) {}

  @Get('offers')
  async getOffers(
    @Headers() headers: CommonHeadersRequestDTO,
    @Query() query: GetInsuranceOffersQueryDTO,
  ): Promise<InsuranceOffersResponseDTO> {
    const command = new GetInsuranceOffersCommand(headers, query.productId);
    const offers = await this.insuranceService.getOffers(command);
    return InsuranceOffersMapper.toResponseDTO(offers);
  }
}
```

### 5. Module wires the country adapter via PORT_TOKEN

```typescript
// insurance.module.ts
@Module({
  imports: [HttpModule],
  controllers: [InsuranceController],
  providers: [
    InsuranceService,
    { provide: INSURANCE_PORT, useClass: LifeInsuranceAdapterCL },
  ],
})
export class InsuranceModule {}
```

### 6. DTO suffix: UPPERCASE

Always `DTO` (not `Dto`). This is a team convention enforced by linting.

```typescript
// CORRECT
export class CommonHeadersRequestDTO { ... }
export class GetInsuranceOffersQueryDTO { ... }
export class InsuranceOffersResponseDTO { ... }

// WRONG
export class CommonHeadersRequestDto { ... }   // lowercase suffix forbidden
```

### 7. File naming: kebab-case with descriptive suffix

```
life-insurance.adapter.base.ts
life-insurance.adapter.cl.ts
life-insurance.adapter.pe.ts
insurance.port.ts
insurance.service.ts
insurance.controller.ts
insurance.module.ts
get-insurance-offers.command.ts
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
@Get('offers')
async getOffers(@Query() query: GetInsuranceOffersQueryDTO) {
  if (query.productId.startsWith('LIFE')) {
    return this.httpService.get('/life-offers');  // logic in controller!
  }
  return this.httpService.get('/home-offers');
}
```

```typescript
// GOOD — controller delegates to service
@Get('offers')
async getOffers(@Headers() headers: CommonHeadersRequestDTO, @Query() query: GetInsuranceOffersQueryDTO) {
  const command = new GetInsuranceOffersCommand(headers, query.productId);
  return this.insuranceService.getOffers(command);
}
```

### BAD: Injecting concrete adapter instead of port
```typescript
// BAD
constructor(private readonly adapter: LifeInsuranceAdapterCL) {} // tight coupling
```

```typescript
// GOOD
constructor(@Inject(INSURANCE_PORT) private readonly port: InsurancePort) {}
```

### BAD: NestJS import in domain layer
```typescript
// BAD — domain/ports/insurance.port.ts
import { Injectable } from '@nestjs/common';  // framework leak into domain

@Injectable()
export interface InsurancePort { ... }
```

```typescript
// GOOD — domain is pure TypeScript
export interface InsurancePort {
  getOffers(command: GetInsuranceOffersCommand): Promise<InsuranceOffer[]>;
}
```

### BAD: Country conditionals in service or base adapter
```typescript
// BAD
async getOffers(command: GetInsuranceOffersCommand) {
  if (command.country === 'CL') { return this.getCLOffers(command); }
  if (command.country === 'PE') { return this.getPEOffers(command); }
}
```

```typescript
// GOOD — each country is its own adapter subclass wired at module level
// LifeInsuranceAdapterCL handles CL, LifeInsuranceAdapterPE handles PE
// The module decides which adapter is injected
```
