---
id: nestjs-bff
version: 1.1.0
team: backend
stack: NestJS, Fastify, TypeScript, HashiCorp Vault, Docker, Kubernetes
last_updated: 2026-04-30
---

# Scroll: NestJS BFF Patterns

## Triggers — load when:
- Files: `main.ts`, `Dockerfile`, `vault/*.hcl`, `*.module.ts`, `*http*.service.ts`
- Keywords: FastifyAdapter, NestFastifyApplication, vault, hcl, stateless, BFF, HttpService, circuit breaker
- Tasks: bootstrapping an app, adding an external API call, writing Dockerfile, configuring secrets

## Context
BFF (Backend-for-Frontend) services are stateless NestJS apps using the Fastify platform. They never own a database — they orchestrate calls to downstream systems (APIs, gateways). HTTP calls go through a managed `HttpService` that provides tracing, logging, and circuit-breaking. Secrets are managed by HashiCorp Vault Agent and injected as Kubernetes environment variables at runtime — they are never stored in CI/CD pipeline files or image layers.

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

The HTTP service is a managed wrapper. It must be provided via its module and injected — never `new HttpService()`.

```typescript
// payment.module.ts
import { HttpModule } from '@nestjs/axios';

@Module({
  imports: [HttpModule.register({ timeout: 5000 })],
  providers: [PaymentService, { provide: PAYMENT_PORT, useClass: StripePaymentAdapterUS }],
  controllers: [PaymentController],
})
export class PaymentModule {}
```

```typescript
// stripe-payment.adapter.us.ts
import { HttpService } from '@nestjs/axios';

@Injectable()
export class StripePaymentAdapterUS extends StripePaymentAdapterBase {
  constructor(
    protected readonly httpService: HttpService,  // injected
    private readonly configService: ConfigService,
  ) {
    super(httpService);
  }

  protected buildPlansUrl(command: GetPaymentPlansCommand): string {
    const base = this.configService.get<string>('STRIPE_BASE_URL');
    return `${base}/us/v2/payments/${command.productId}/plans`;
  }
}
```

### 3. ConfigService for all environment variables

Never read `process.env` directly inside service or adapter classes.

```typescript
// CORRECT
@Injectable()
export class PaymentService {
  constructor(private readonly config: ConfigService) {}

  private get stripeUrl(): string {
    return this.config.getOrThrow<string>('STRIPE_BASE_URL');
  }
}
```

### 4. HashiCorp Vault secrets layout

Runtime secrets are defined in HCL files and injected as K8s env vars by Vault Agent. Never put secret values in pipeline YAML.

```hcl
# vault/staging.hcl
path "secret/data/my-api/staging" {
  capabilities = ["read"]
}
```

```hcl
# vault/prod.hcl
path "secret/data/my-api/prod" {
  capabilities = ["read"]
}
```

CI/CD variables (`$CI_*`, `$REGISTRY_USER`) are build-time only — for image tagging, Docker login, etc. They must never carry application secrets.

### 5. Dockerfile: multi-stage, official images, npm ci, USER node

```dockerfile
# Dockerfile
FROM node:20-alpine AS development
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine AS production
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
- Use official or internally approved base images — never use arbitrary third-party images
- `npm ci` — deterministic, uses `package-lock.json`; never `npm install`
- `USER node` in production stage — never run as root
- Two stages minimum: `development` (build) and `production` (runtime)

### 6. No database rule — BFF is purely stateless

The BFF must not own any persistent storage. If data needs to be cached, use a shared Redis library (injected, not raw `ioredis`).

```typescript
// NEVER in a BFF service
import { InjectRepository } from '@nestjs/typeorm'; // forbidden
import { TypeOrmModule } from '@nestjs/typeorm';    // forbidden

// ALLOWED for ephemeral caching only
import { RedisService } from '@acme/redis';
```

### 7. No Express-specific API surface

Fastify and Express have different request/response shapes. Never use Express patterns.

```typescript
// BAD — Express pattern, breaks on Fastify
@Get('plans')
getPlans(@Req() req: Request) {
  return req.body;  // Express-specific
}

// GOOD — NestJS decorators work on both, but prefer typed DTOs
@Get('plans')
getPlans(@Headers() headers: CommonHeadersRequestDTO, @Query() query: GetPaymentPlansQueryDTO) {
  return this.paymentService.getPlans(new GetPaymentPlansCommand(headers, query.productId));
}
```

---

## Anti-Patterns

### BAD: Direct process.env access in a service
```typescript
// BAD
const url = process.env.STRIPE_BASE_URL + '/plans';
```

```typescript
// GOOD
const url = this.configService.getOrThrow<string>('STRIPE_BASE_URL') + '/plans';
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
# BAD — .gitlab-ci.yml or .github/workflows/*.yml
variables:
  DB_PASSWORD: "supersecret"
  STRIPE_API_KEY: "sk_live_abc123"
```

```hcl
# GOOD — vault/prod.hcl declares the Vault path, Agent injects at runtime
path "secret/data/my-api/prod" {
  capabilities = ["read"]
}
```

### BAD: Running container as root
```dockerfile
# BAD — no USER directive, defaults to root
FROM node:20-alpine
COPY . .
CMD ["node", "dist/main.js"]
```

```dockerfile
# GOOD
USER node
CMD ["node", "dist/main.js"]
```
