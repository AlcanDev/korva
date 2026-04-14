---
applyTo: "src/**/*.ts,apps/**/*.ts,libs/**/*.ts"
---

# Backend — NestJS Hexagonal Architecture

## Layer rules (enforced — never skip)

**DOMAIN** (`domain/`) — pure TypeScript, zero framework imports
- Entities, value objects, CQRS commands, port interfaces, domain errors
- If it imports `@nestjs/*`, `fastify`, `axios`, or `cache-manager` → it's in the wrong layer

**APPLICATION** (`application/`) — orchestrates domain through ports only
- One service per bounded context
- DI via token: `@Inject(INSURANCE_PORT) private readonly port: InsurancePort`
- Never import a concrete adapter class

**INFRASTRUCTURE** (`infrastructure/`) — all I/O (HTTP, cache, DB, adapters, controllers, DTOs)
- Template Method per country: `Base → CL → PE → CO`
- DTOs validate at the boundary with class-validator
- Controllers orchestrate — never contain business logic

## Naming conventions (hard rules)

| What | Convention | Example |
|------|-----------|---------|
| Files | kebab-case + suffix | `life-insurance.adapter.base.ts` |
| Classes | PascalCase + suffix | `LifeInsuranceAdapterBase` |
| DTOs | `…DTO` uppercase | `CommonHeadersRequestDTO` (never `…Dto`) |
| Port tokens | SCREAMING_SNAKE_CASE const | `export const INSURANCE_PORT = 'InsurancePort'` |
| Commands | NounVerb + `Command` | `GetInsuranceOffersCommand` |
| Domain errors | Descriptive + `Error` | `InsuranceNotFoundError` |

## Error handling (mandatory pattern)

```typescript
// Domain — pure error, zero framework
export class InsuranceNotFoundError extends Error {
  constructor(public readonly id: InsuranceId) {
    super(`Insurance ${id.value} not found`);
    this.name = 'InsuranceNotFoundError';
  }
}

// Application — re-throw domain errors, wrap infrastructure errors
async getOffers(cmd: GetOffersCommand) {
  try {
    return await this.port.getOffers(cmd);
  } catch (err) {
    if (err instanceof InsuranceNotFoundError) throw err;
    this.logger.error({ message: 'Adapter failure', error: err.message }, InsuranceService.name);
    throw new InsuranceUnavailableError(err.message);
  }
}

// Infrastructure — exception filter maps domain → HTTP
@Catch(InsuranceNotFoundError)
export class InsuranceNotFoundFilter implements ExceptionFilter {
  catch(ex: InsuranceNotFoundError, host: ArgumentsHost) {
    host.switchToHttp().getResponse().status(404).json({ error: ex.message });
  }
}
```

## Forbidden patterns

```typescript
// ❌ console.log — use injected Logger
// ❌ new ConcreteAdapter() outside .module.ts
// ❌ any without // korva-ignore: <reason>
// ❌ hardcoded credentials — use ConfigService + Vault
// ❌ direct HTTP call in application layer — use ports
// ❌ business logic in controller
// ❌ npm install in Dockerfile — use npm ci
// ❌ root user in container — use USER node
```

## HTTP Client

Use `FifHttpService` from `@df-libs/rest-client` — never raw `axios` or `HttpModule`.

## Logging

```typescript
// Always inject NestJS Logger with class name as context
private readonly logger = new Logger(InsuranceService.name);
this.logger.error({ message: 'Error', error: err.message }, InsuranceService.name);
```

## Caching (Redis via cache-manager v4)

Cache in infrastructure layer only, never in domain or application.
TTL in seconds from ConfigService — never hardcoded.
