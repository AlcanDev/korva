---
id: nestjs-bff
version: 1.0.0
team: backend
stack: NestJS, Fastify, TypeScript, HashiCorp Vault, Docker, Kubernetes
---

# Scroll: NestJS BFF Patterns

## Triggers — load when:
- Files: `main.ts`, `Dockerfile`, `vault/*.hcl`, `*.module.ts`, `fif-http*.ts`
- Keywords: HttpService, FastifyAdapter, NestFastifyApplication, vault, hcl, registry.your-company.com, EXTERNAL_API, TRO, Apigee, stateless, BFF
- Tasks: bootstrapping an app, adding an external API call, writing Dockerfile, configuring secrets

## Context
All BFF services at Acme Financiero are stateless NestJS apps using the Fastify platform. They never own a database — they orchestrate calls to downstream systems (EXTERNAL_API, TRO, Apigee gateways). HTTP calls go through `HttpService` from `@internal/libs/rest-client`, which provides tracing, logging, and circuit-breaking. Secrets are managed by HashiCorp Vault Agent and injected as Kubernetes environment variables at runtime — they are never stored in `.gitlab-ci.yml` or image layers.

---

## Rules

### 1. Bootstrap: Fastify platform, not Express

```typescript
// main.ts
import { NestFactory } from '@nestjs/core';
import { FastifyAdapter, NestFastifyApplication } from '@nestjs/platform-fastify';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create<NestFastifyApplication>(
    AppModule,
    new FastifyAdapter({ logger: true }),
  );

  app.setGlobalPrefix('api/v1');
  await app.listen(3000, '0.0.0.0');
}

bootstrap();
```

### 2. HttpService: always inject, never instantiate

`HttpService` is a managed wrapper. It must be provided via `FifHttpModule` and injected — never `new HttpService()`.

```typescript
// insurance.module.ts
import { FifHttpModule } from '@internal/libs/rest-client';

@Module({
  imports: [FifHttpModule.register({ serviceName: 'example-bff' })],
  providers: [InsuranceService, { provide: INSURANCE_PORT, useClass: LifeInsuranceAdapterCL }],
  controllers: [InsuranceController],
})
export class InsuranceModule {}
```

```typescript
// life-insurance.adapter.cl.ts
import { HttpService } from '@internal/libs/rest-client';

@Injectable()
export class LifeInsuranceAdapterCL extends LifeInsuranceAdapterBase {
  constructor(
    protected readonly httpService: HttpService,  // injected
    private readonly configService: ConfigService,
  ) {
    super(httpService);
  }

  protected buildOffersUrl(command: GetInsuranceOffersCommand): string {
    const base = this.configService.get<string>('EXTERNAL_API_BASE_URL');
    return `${base}/cl/v2/insurances/${command.productId}/offers`;
  }
}
```

### 3. ConfigService for all environment variables

Never read `process.env` directly inside service or adapter classes.

```typescript
// CORRECT
@Injectable()
export class InsuranceService {
  constructor(private readonly config: ConfigService) {}

  private get external-apiUrl(): string {
    return this.config.getOrThrow<string>('EXTERNAL_API_BASE_URL');
  }
}
```

### 4. HashiCorp Vault secrets layout

Runtime secrets are defined in HCL files and injected as K8s env vars by Vault Agent. Never put secret values in pipeline YAML.

```hcl
# vault/qa.hcl
path "secret/data/home-api/qa" {
  capabilities = ["read"]
}
```

```hcl
# vault/prod.hcl
path "secret/data/home-api/prod" {
  capabilities = ["read"]
}
```

GitLab CI variables (`$CI_*`, `$HARBOR_USER`) are build-time only — for image tagging, Docker login, etc. They must never carry application secrets.

### 5. Dockerfile: multi-stage, registry.your-company.com images, npm ci, USER node

```dockerfile
# Dockerfile
FROM registry.your-company.com/base/node:20-latest AS development
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM registry.your-company.com/base/node:20-latest AS production
WORKDIR /app
ENV NODE_ENV=production
COPY package*.json ./
RUN npm ci --only=production
COPY --from=development /app/dist ./dist
USER node
EXPOSE 3000
CMD ["node", "dist/main.js"]
```

Rules:
- `registry.your-company.com/base/node:20-latest` — internal Harbor registry, never `docker.io/node`
- `npm ci` — deterministic, uses `package-lock.json`; never `npm install`
- `USER node` in production stage — never run as root
- Two stages minimum: `development` (build) and `production` (runtime)

### 6. No database rule — BFF is purely stateless

The BFF must not own any persistent storage. If data needs to be cached, use the `@home-api/redis` shared library (injected, not raw `ioredis`).

```typescript
// NEVER in a BFF service
import { InjectRepository } from '@nestjs/typeorm'; // forbidden
import { TypeOrmModule } from '@nestjs/typeorm';    // forbidden

// ALLOWED for ephemeral caching only
import { RedisService } from '@home-api/redis';
```

### 7. No Express-specific API surface

Fastify and Express have different request/response shapes. Never use Express patterns.

```typescript
// BAD — Express pattern, breaks on Fastify
@Get('offers')
getOffers(@Req() req: Request) {
  return req.body;  // Express-specific
}

// GOOD — NestJS decorators work on both, but prefer typed DTOs
@Get('offers')
getOffers(@Headers() headers: CommonHeadersRequestDTO, @Query() query: GetInsuranceOffersQueryDTO) {
  return this.insuranceService.getOffers(new GetInsuranceOffersCommand(headers, query.productId));
}
```

---

## Anti-Patterns

### BAD: Direct process.env access in a service
```typescript
// BAD
const url = process.env.EXTERNAL_API_BASE_URL + '/offers';
```

```typescript
// GOOD
const url = this.configService.getOrThrow<string>('EXTERNAL_API_BASE_URL') + '/offers';
```

### BAD: Instantiating HttpService manually
```typescript
// BAD
const http = new HttpService();  // bypasses tracing, logging, circuit breaker
```

```typescript
// GOOD — import module, inject service
constructor(protected readonly httpService: HttpService) {}
```

### BAD: Secrets in pipeline YAML
```yaml
# BAD — .gitlab-ci.yml
variables:
  DB_PASSWORD: "supersecret"
  EXTERNAL_API_API_KEY: "abc123"
```

```hcl
# GOOD — vault/prod.hcl declares the Vault path, Agent injects at runtime
path "secret/data/home-api/prod" {
  capabilities = ["read"]
}
```

### BAD: Running container as root
```dockerfile
# BAD — no USER directive, defaults to root
FROM registry.your-company.com/base/node:20-latest
COPY . .
CMD ["node", "dist/main.js"]
```

```dockerfile
# GOOD
USER node
CMD ["node", "dist/main.js"]
```
