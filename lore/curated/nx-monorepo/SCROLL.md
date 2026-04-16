---
id: nx-monorepo
version: 1.1.0
team: backend
stack: Nx, NestJS, TypeScript, Node 20
---

# Scroll: Nx Monorepo Rules

## Triggers — load when:
- Files: `nx.json`, `project.json`, `tsconfig.base.json`, `apps/**`, `libs/**`
- Keywords: nx, affected, workspace, library, import path, @acme, scope, generate, executor, cache
- Tasks: adding a new library, running builds, creating a new app, importing between projects, CI pipeline setup

## Context
An Nx monorepo scopes all projects under a single workspace alias (e.g., `@acme`). Multiple apps can share infrastructure libs while each owning their own domain logic. Shared infrastructure (auth, logging, cache, etc.) lives in libs. Business logic is owned by apps, not libs — libs are cross-cutting concerns only. CI always uses `affected` commands to avoid building the entire workspace on every change.

---

## Rules

### 1. Workspace scope and app structure

```
apps/
  api-us/         @acme/api-us   — US region BFF
    Dockerfile
    vault/
      staging.hcl
      prod.hcl
  api-eu/         @acme/api-eu   — EU region BFF
    Dockerfile
    vault/
      staging.hcl
      prod.hcl
  api-apac/       @acme/api-apac — APAC region BFF
    Dockerfile
    vault/
      staging.hcl
      prod.hcl

libs/
  auth/           @acme/auth
  otel/        @acme/otel
  exceptions/     @acme/exceptions
  feature-flags/  @acme/feature-flags
  logger/         @acme/logger
  redis/          @acme/redis
  swagger/        @acme/swagger
  util/           @acme/util
```

### 2. Always use affected commands

Never run `nx build all` or `nx test all` in CI. Use `affected` to run only what changed.

```bash
# CI — build only affected projects
nx affected:build --base=origin/main

# CI — test only affected projects
nx affected:test --base=origin/main

# Local — see what would be affected by current changes
nx affected:graph
```

### 3. Import via @acme alias — never relative paths between projects

```typescript
// CORRECT — alias import
import { LoggerService }          from '@acme/logger';
import { AuthGuard }              from '@acme/auth';
import { RedisService }           from '@acme/redis';
import { GlobalExceptionFilter }  from '@acme/exceptions';

// WRONG — relative import crossing project boundaries
import { LoggerService } from '../../../../libs/logger/src'; // forbidden
import { AuthGuard }     from '../../../auth/src/lib/auth.guard'; // forbidden
```

Relative imports are only allowed within the same project (same `apps/` or `libs/` folder).

### 4. Generating a new shared library

```bash
nx generate @nrwl/nest:library <name> \
  --importPath=@acme/<name> \
  --buildable \
  --publishable=false
```

After generation, verify `tsconfig.base.json` has the path alias:

```json
{
  "compilerOptions": {
    "paths": {
      "@acme/<name>": ["libs/<name>/src/index.ts"]
    }
  }
}
```

### 5. Business logic belongs in apps, not libs

Libs contain only cross-cutting infrastructure. Domain logic, ports, adapters, and commands belong to the specific app.

```
libs/logger/         — structured logging wrapper        (OK)
libs/auth/           — JWT guard, decorators             (OK)
libs/redis/          — Redis connection + cache helpers  (OK)

apps/api-us/src/
  domain/            — PaymentPort, PaymentId            (OK — app owns domain)
  application/       — PaymentService                    (OK — app owns use cases)
  infrastructure/    — StripePaymentAdapterUS            (OK — app owns adapters)
```

If a domain concept needs to be shared between region apps, use the Template Method pattern in an abstract base class within each app, not by moving it to a lib.

### 6. project.json executor configuration

Each app's `project.json` must declare build, serve, test, and lint targets:

```json
{
  "name": "api-us",
  "$schema": "../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "apps/api-us/src",
  "projectType": "application",
  "targets": {
    "build": {
      "executor": "@nrwl/node:build",
      "options": {
        "outputPath": "dist/apps/api-us",
        "main": "apps/api-us/src/main.ts",
        "tsConfig": "apps/api-us/tsconfig.app.json",
        "assets": ["apps/api-us/src/assets"]
      }
    },
    "test": {
      "executor": "@nrwl/jest:jest",
      "options": {
        "jestConfig": "apps/api-us/jest.config.ts",
        "passWithNoTests": false
      }
    }
  },
  "tags": ["scope:us", "type:app"]
}
```

### 7. Enforce module boundaries with tags

`nx.json` or `.eslintrc.json` enforces that `type:app` cannot import from another `type:app`, and `type:lib` cannot import `type:app`.

```json
{
  "@nrwl/nx/enforce-module-boundaries": ["error", {
    "depConstraints": [
      { "sourceTag": "type:app",  "onlyDependOnLibsWithTags": ["type:lib"] },
      { "sourceTag": "type:lib",  "onlyDependOnLibsWithTags": ["type:lib"] },
      { "sourceTag": "scope:us",  "onlyDependOnLibsWithTags": ["scope:us", "scope:shared"] },
      { "sourceTag": "scope:eu",  "onlyDependOnLibsWithTags": ["scope:eu", "scope:shared"] }
    ]
  }]
}
```

---

## Anti-Patterns

### BAD: Building the entire workspace in CI
```bash
# BAD — rebuilds everything on every commit
nx run-many --target=build --all
```

```bash
# GOOD — only affected projects
nx affected:build --base=origin/main
```

### BAD: Cross-project relative imports
```typescript
// BAD
import { RedisService } from '../../../libs/redis/src/lib/redis.service';
```

```typescript
// GOOD
import { RedisService } from '@acme/redis';
```

### BAD: Domain logic in a lib
```typescript
// BAD — libs/payment-domain/src/lib/payment.service.ts
// Business logic shared across all regions in a lib
export class PaymentService {
  getPlans(region: 'US' | 'EU' | 'APAC') { ... }
}
```

```typescript
// GOOD — each app owns its PaymentService
// apps/api-us/src/application/payment.service.ts   — US-specific
// apps/api-eu/src/application/payment.service.ts   — EU-specific
// Shared algorithm lives in an abstract base class inside each app
```

### BAD: Missing @acme path alias after generating a lib
```json
// BAD — tsconfig.base.json missing new lib
{
  "compilerOptions": {
    "paths": {
      "@acme/logger": ["libs/logger/src/index.ts"]
      // @acme/new-lib missing!
    }
  }
}
```

```json
// GOOD — alias added after nx generate
{
  "compilerOptions": {
    "paths": {
      "@acme/logger":   ["libs/logger/src/index.ts"],
      "@acme/new-lib":  ["libs/new-lib/src/index.ts"]
    }
  }
}
```
