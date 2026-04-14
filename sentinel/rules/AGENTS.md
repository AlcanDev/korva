# Korva Sentinel — Architecture Rules

These rules are validated by `korva sentinel run` on every commit.
Add `// korva-ignore: <reason>` to suppress a specific violation.

---

## Hexagonal Architecture Violations

### [HEX-001] Domain layer imports framework dependencies
**Trigger:** `import { Injectable } from '@nestjs/common'` in `*/domain/**/*.ts`
**Message:** Domain layer must not import framework packages. Move to application or infrastructure.

### [HEX-002] Application layer imports concrete adapter
**Trigger:** `import.*Adapter` in `*/application/**/*.ts`
**Message:** Application layer must only depend on port interfaces, not concrete adapters.

### [HEX-003] Direct HTTP client in application layer
**Trigger:** `import.*axios|import.*FifHttpService|import.*HttpService` in `*/application/**/*.ts`
**Message:** Application layer must not make HTTP calls. Use a port interface instead.

---

## Code Quality

### [QC-001] console.log in source code
**Trigger:** `console.log` in `*/src/**/*.ts` (not in `*.spec.ts`)
**Message:** Use the team logger (@df-libs/logger or LoggerInterceptor) instead of console.log.

### [QC-002] TypeScript `any` without justification
**Trigger:** `: any` or `as any` in `*.ts` without `// korva-ignore`
**Message:** Avoid `any`. Use `unknown` with type guards, or add `// korva-ignore: <reason>`.

### [QC-003] Hardcoded secrets
**Trigger:** `password=`, `token=`, `SECRET_ID=`, `ROLE_ID=` (non-environment, non-test files)
**Message:** Never hardcode secrets. Use process.env.* + ConfigModule.

---

## Naming Conventions

### [NAME-001] DTO with lowercase suffix
**Trigger:** `class .*Dto` (not `DTO`) in `*/dtos/**/*.ts`
**Message:** Use uppercase DTO suffix: `CommonHeadersRequestDTO` not `CommonHeadersRequestDto`.

### [NAME-002] PascalCase filename
**Trigger:** File named with PascalCase in `src/` (e.g., `InsuranceService.ts`)
**Message:** Use kebab-case filenames: `insurance.service.ts` not `InsuranceService.ts`.

---

## Testing

### [TEST-001] Missing spec file for service
**Trigger:** New `*.service.ts` file without corresponding `*.service.spec.ts`
**Message:** Add a co-located spec file: `insurance.service.spec.ts` next to `insurance.service.ts`.

---

## Notes
- Rules are evaluated against `git diff --cached` (staged files only)
- Files matching `*.spec.ts`, `*.test.ts`, `__fixtures__/**` are excluded from most checks
- The `korva-ignore` comment suppresses a violation for a single line
