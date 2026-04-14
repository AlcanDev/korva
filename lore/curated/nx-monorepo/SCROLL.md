---
id: nx-monorepo
version: 1.0.0
team: backend
stack: Nx, NestJS, TypeScript, Node 20
---

# Scroll: Nx Monorepo Rules

## Triggers — load when:
- Files: `nx.json`, `project.json`, `tsconfig.base.json`, `apps/**`, `libs/**`
- Keywords: nx, affected, workspace, library, import path, @home-api, scope, generate, executor, cache
- Tasks: adding a new library, running builds, creating a new app, importing between projects, CI pipeline setup

## Context
The home-insurance platform is an Nx monorepo scoped under `@home-api`. Three apps (CL, PE, CO) each have their own Dockerfile and Vault HCL files. Shared infrastructure (auth, logging, redis, etc.) lives in libs. Business logic is owned by apps, not libs — libs are cross-cutting concerns only. CI always uses `affected` commands to avoid building the entire workspace on every change.

---

## Rules

### 1. Workspace scope and app structure

```
apps/
  cl/           @home-api/cl   — Chile BFF
    Dockerfile
    vault/
      qa.hcl
      prod.hcl
  pe/           @home-api/pe   — Peru BFF
    Dockerfile
    vault/
      qa.hcl
      prod.hcl
  co/           @home-api/co   — Colombia BFF
    Dockerfile
    vault/
      qa.hcl
      prod.hcl

libs/
  auth/         @home-api/auth
  otel/      @home-api/otel
  exceptions/   @home-api/exceptions
  flagr/        @home-api/flagr
  logger/       @home-api/logger
  redis/        @home-api/redis
  swagger/      @home-api/swagger
  util/         @home-api/util
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

### 3. Import via @home-api alias — never relative paths between projects

```typescript
// CORRECT — alias import
import { LoggerService }     from '@home-api/logger';
import { AuthGuard }         from '@home-api/auth';
import { RedisService }      from '@home-api/redis';
import { GlobalExceptionFilter } from '@home-api/exceptions';

// WRONG — relative import crossing project boundaries
import { LoggerService } from '../../../../libs/logger/src'; // forbidden
import { AuthGuard }     from '../../../auth/src/lib/auth.guard'; // forbidden
```

Relative imports are only allowed within the same project (same `apps/` or `libs/` folder).

### 4. Generating a new shared library

```bash
nx generate @nrwl/nest:library <name> \
  --importPath=@home-api/<name> \
  --buildable \
  --publishable=false
```

After generation, verify `tsconfig.base.json` has the path alias:

```json
{
  "compilerOptions": {
    "paths": {
      "@home-api/<name>": ["libs/<name>/src/index.ts"]
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

apps/cl/src/
  domain/            — InsurancePort, InsuranceId        (OK — app owns domain)
  application/       — InsuranceService                  (OK — app owns use cases)
  infrastructure/    — LifeInsuranceAdapterCL            (OK — app owns adapters)
```

If a domain concept needs to be shared between CL, PE, and CO, use the Template Method pattern in an abstract base class within the app, not by moving it to a lib.

### 6. project.json executor configuration

Each app's `project.json` must declare build, serve, test, and lint targets:

```json
{
  "name": "cl",
  "$schema": "../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "apps/cl/src",
  "projectType": "application",
  "targets": {
    "build": {
      "executor": "@nrwl/node:build",
      "options": {
        "outputPath": "dist/apps/cl",
        "main": "apps/cl/src/main.ts",
        "tsConfig": "apps/cl/tsconfig.app.json",
        "assets": ["apps/cl/src/assets"]
      }
    },
    "test": {
      "executor": "@nrwl/jest:jest",
      "options": {
        "jestConfig": "apps/cl/jest.config.ts",
        "passWithNoTests": false
      }
    }
  },
  "tags": ["scope:cl", "type:app"]
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
      { "sourceTag": "scope:cl",  "onlyDependOnLibsWithTags": ["scope:cl", "scope:shared"] },
      { "sourceTag": "scope:pe",  "onlyDependOnLibsWithTags": ["scope:pe", "scope:shared"] }
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
import { RedisService } from '@home-api/redis';
```

### BAD: Domain logic in a lib
```typescript
// BAD — libs/insurance-domain/src/lib/insurance.service.ts
// Business logic shared across all countries in a lib
export class InsuranceService {
  getOffers(country: 'CL' | 'PE' | 'CO') { ... }
}
```

```typescript
// GOOD — each app owns its InsuranceService
// apps/cl/src/application/insurance.service.ts — CL-specific
// apps/pe/src/application/insurance.service.ts — PE-specific
// Shared algorithm lives in an abstract base class inside each app
```

### BAD: Missing @home-api path alias after generating a lib
```json
// BAD — tsconfig.base.json missing new lib
{
  "compilerOptions": {
    "paths": {
      "@home-api/logger": ["libs/logger/src/index.ts"]
      // @home-api/new-lib missing!
    }
  }
}
```

```json
// GOOD — alias added after nx generate
{
  "compilerOptions": {
    "paths": {
      "@home-api/logger":   ["libs/logger/src/index.ts"],
      "@home-api/new-lib":  ["libs/new-lib/src/index.ts"]
    }
  }
}
```
