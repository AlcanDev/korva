# Korva Sentinel — Architecture Rules

These rules are validated by `korva sentinel run` on every commit.
Add `// korva-ignore: <reason>` to suppress a specific violation on a single line.

Each rule block lists:
- **Trigger** — the pattern Sentinel scans for in staged files
- **Message** — what the developer sees when the rule fires
- **Why** — the engineering principle behind the rule
- **Fix** — the fastest path to a passing commit

---

## Architecture Violations

### [ARC-001] Domain layer imports framework/infrastructure dependency

**Trigger:** Any import of a framework package (`express`, `fastapi`, `nestjs`, `gin`, `django`, `typeorm`, `prisma`, `mongoose`, `knex`) inside a path matching `*/domain/**` or `*/core/**`

**Message:**
```
ARC-001: Domain layer must not import framework or infrastructure packages.
  File: src/orders/domain/order.service.ts
  Found: import { Repository } from 'typeorm'

  The domain is the innermost ring — it must remain framework-free so it can be
  tested in isolation and ported to any runtime.

  Fix: define a port interface (OrderRepository) in the domain, then inject the
  TypeORM implementation from the infrastructure layer.
```

**Why:** When your domain imports TypeORM, it becomes coupled to a specific persistence technology. Swapping databases, writing unit tests without a live DB, and running domain logic in a different context (CLI, worker, etc.) all become painful. Ports and adapters exist precisely to avoid this.

**Fix hint:** Create `src/orders/domain/ports/order-repository.port.ts` with an interface. Let the infrastructure adapter implement it. Use constructor injection.

---

### [ARC-002] HTTP handler contains business logic

**Trigger:** Functions longer than 25 lines inside files matching `*.controller.ts`, `*.router.ts`, `*.handler.ts`, `*_view.py`, `*.route.go` — or direct calls to DB clients (`db.query`, `prisma.`, `repository.find`) from within those same files.

**Message:**
```
ARC-002: HTTP handler contains business logic or direct data access.
  File: src/payments/payments.controller.ts
  Found: prisma.payment.findMany(...)

  Controllers/handlers are responsible only for HTTP concerns: parsing the
  request, calling a use-case/service, and serializing the response.

  Fix: move the query into a PaymentService or PaymentRepository, then call
  that from the controller.
```

**Why:** Business logic in handlers cannot be reused by workers, CLIs, or tests that do not spin up an HTTP server. A handler that does "real work" is both hard to unit-test and hard to change without touching routing code.

**Fix hint:** Extract logic into a service/use-case class. The handler should read: parse → call service → return response — three lines of substance.

---

### [ARC-003] Direct database access from non-repository layer

**Trigger:** Direct use of `db.query(`, `prisma.`, `mongoose.`, `sqlalchemy.`, `sql.Open(`, `pgx.Connect(` in files that are NOT under `*/repository/**`, `*/store/**`, `*/persistence/**`, or `*/infrastructure/**`

**Message:**
```
ARC-003: Database access outside the repository/store layer.
  File: src/notifications/notification.service.ts
  Found: prisma.notification.create(...)

  Data access must be isolated in repository/store classes. Services should call
  a repository method — they must not hold DB references directly.

  Fix: inject a NotificationRepository and delegate persistence through it.
```

**Why:** Scattered DB calls make it impossible to mock storage in tests, track all queries in one place, or swap ORMs without touching business logic.

---

## Security

### [SEC-001] Hardcoded secret detected

**Trigger:** Patterns (case-insensitive) in non-test files:
- `password\s*=\s*["'][^"']{4,}["']`
- `token\s*=\s*["'][^"']{8,}["']`
- `api_key\s*=\s*["']`
- `sk_live_`, `sk_test_`, `AKIA[0-9A-Z]{16}`, `ghp_`, `glpat-`
- `Bearer [A-Za-z0-9\-_]{20,}` as a string literal

**Message:**
```
SEC-001: Hardcoded secret detected.
  File: src/integrations/stripe.client.ts
  Found: apiKey = "sk_live_4xT..."

  Committing a secret exposes it permanently in git history — rotating the key
  does not remove it from previous commits or forks.

  Fix:
    1. Revoke the exposed credential immediately.
    2. Move the value to an environment variable or secrets manager.
    3. Read it via process.env.STRIPE_SECRET_KEY / os.environ["STRIPE_KEY"] / os.Getenv("STRIPE_KEY").
    4. Add the file to .gitignore if it holds local overrides.
```

**Why:** Hardcoded secrets are the single most common cause of credential leaks. Once a key lands in a commit it must be considered compromised — git history is forever.

---

### [SEC-002] Sensitive data in log statement

**Trigger:** Calls to any logger (`console.log`, `logger.info/debug/warn/error`, `logging.info`, `log.Printf`, `slog.Info`) where the argument string or interpolated variable name contains: `password`, `passwd`, `token`, `secret`, `card`, `cvv`, `ssn`, `credit_card`, `pan`

**Message:**
```
SEC-002: Potentially sensitive field passed to a log statement.
  File: src/auth/login.handler.ts
  Found: logger.info(`Login attempt for ${user.email} pwd=${body.password}`)

  Log aggregation systems (Datadog, Splunk, ELK) are often less restricted than
  production databases. A password or token in a log line may be readable by
  many engineers and retained for months.

  Fix: log only non-sensitive identifiers (user ID, request ID). If you need to
  debug auth issues, log a boolean flag: passwordProvided: true.
```

---

### [SEC-003] Timing attack vulnerability

**Trigger:** Direct equality comparison (`==`, `===`, `!=`, `!==`) applied to variables whose names contain `token`, `secret`, `hash`, `hmac`, `signature`, `key`, `password` — outside of test files

**Message:**
```
SEC-003: Constant-time comparison required for secrets.
  File: src/webhooks/verify.ts
  Found: if (receivedSignature === expectedSignature)

  String equality in most languages short-circuits on the first differing byte.
  An attacker can measure response times to guess the secret one character at a
  time (timing attack).

  Fix:
    Node.js:  crypto.timingSafeEqual(Buffer.from(a), Buffer.from(b))
    Python:   hmac.compare_digest(a, b)
    Go:       subtle.ConstantTimeCompare([]byte(a), []byte(b))
```

**Why:** Timing attacks on webhook secrets and HMAC signatures are practical. They have been used against real production systems. A constant-time comparison costs essentially nothing extra.

---

### [SEC-004] CORS wildcard in production

**Trigger:** `origin:\s*["']\*["']`, `Access-Control-Allow-Origin:\s*\*`, `allow_origins=\["?\*"?\]` in files NOT matching `*.test.*`, `*.spec.*`, `*_test.*`

**Message:**
```
SEC-004: CORS wildcard origin detected in non-test code.
  File: src/app.ts
  Found: origin: "*"

  A wildcard CORS policy allows any website to make credentialed cross-origin
  requests to your API. This is safe for fully public, unauthenticated APIs only.

  Fix: enumerate allowed origins explicitly, or read them from an environment
  variable (CORS_ORIGINS=https://app.example.com,https://admin.example.com).

  // korva-ignore: public CDN endpoint, no credentials
```

---

### [SEC-005] SQL injection via string interpolation

**Trigger:** SQL keywords (`SELECT`, `INSERT`, `UPDATE`, `DELETE`, `WHERE`, `FROM`) appearing inside template literals or string concatenation:
- `` `SELECT * FROM ${` `` (JS/TS template literal)
- `"SELECT * FROM " + ` (string concatenation with `+`)
- `f"SELECT * FROM {` (Python f-string)
- `fmt.Sprintf("SELECT * FROM %s"` (Go — unless the argument is a whitelisted table constant)

**Message:**
```
SEC-005: SQL query built via string interpolation — SQL injection risk.
  File: src/users/user.repository.ts
  Found: `SELECT * FROM users WHERE id = ${userId}`

  String-interpolated queries allow an attacker to inject arbitrary SQL if
  userId ever originates from user input (directly or through a chain of calls).

  Fix: use parameterized queries / prepared statements.
    // BAD
    db.query(`SELECT * FROM users WHERE id = ${userId}`)

    // GOOD
    db.query('SELECT * FROM users WHERE id = $1', [userId])
    // or with an ORM
    userRepository.findOne({ where: { id: userId } })
```

**Why:** SQL injection remains the most exploited web vulnerability class (OWASP A03). Parameterized queries eliminate it entirely at zero performance cost.

---

### [SEC-006] Missing authentication on sensitive route

**Trigger:** Route definitions (Express `app.get/post/put/delete`, Fastify `fastify.route`, FastAPI `@app.get`, Gin `r.GET`, etc.) where the path contains `/admin`, `/internal`, `/users/:id`, `/config`, `/debug` and no auth middleware/decorator is visible within the same function call or decorator chain.

**Message:**
```
SEC-006: Sensitive route registered without visible authentication middleware.
  File: src/admin/admin.router.ts
  Found: router.get('/admin/users', getAllUsersHandler)

  Routes under /admin, /internal, or that expose user data must require
  authentication. Unauthenticated admin endpoints have caused high-profile breaches.

  Fix: attach your auth guard/middleware before the handler.
    router.get('/admin/users', requireAdmin, getAllUsersHandler)
    // or with a decorator
    @UseGuards(AdminGuard)

  If this route is intentionally public, suppress with:
    // korva-ignore: health check endpoint, no sensitive data
```

---

## Code Quality

### [QC-001] Debug statement in production code

**Trigger:** `console.log(`, `console.debug(`, `print(`, `fmt.Println(`, `pp.Println(`, `debugger;`, `breakpoint()` in `src/**` or `cmd/**` files — excluding test and fixture files

**Message:**
```
QC-001: Debug statement found in production code.
  File: src/checkout/checkout.service.ts
  Found: console.log('cart items:', items)

  Debug output can leak sensitive context in production logs, pollutes log
  aggregation, and often indicates unfinished code.

  Fix: replace with a structured logger call at the appropriate level.
    logger.debug('cart_items_loaded', { count: items.length, userId })
  Or remove if it was temporary debugging.
```

---

### [QC-002] TypeScript `any` without justification

**Trigger:** `: any` or `as any` in `*.ts` or `*.tsx` files (not in `.d.ts`, `*.test.ts`, `*.spec.ts`) without a `// korva-ignore` comment on the same line

**Message:**
```
QC-002: TypeScript 'any' used without justification.
  File: src/payments/mapper.ts
  Found: const payload: any = response.data

  'any' disables type checking for the variable and everything downstream.
  A single 'any' can silence real bugs that TypeScript would otherwise catch.

  Fix:
    - Use 'unknown' + a type guard or Zod schema to validate external data.
    - Use a specific interface or type alias.
    - If 'any' is genuinely necessary (e.g., dynamic plugin system), document why:
        const plugin: any = loadPlugin(name) // korva-ignore: dynamic plugin loader, no static type available
```

---

## Naming Conventions

### [NAM-001] File naming inconsistency

**Trigger:** Source files under `src/`, `lib/`, `pkg/`, `internal/` that use PascalCase (`OrderService.ts`), camelCase (`orderService.ts`), or mixed casing — instead of the project-configured convention. Default expected convention: `kebab-case` for TypeScript/JavaScript/Python, `snake_case` for Go/Python modules.

**Message:**
```
NAM-001: File name does not follow the project naming convention.
  File: src/orders/OrderService.ts
  Expected convention: kebab-case

  Inconsistent naming makes glob patterns unreliable, confuses auto-import
  tools on case-sensitive filesystems (Linux CI vs macOS local), and signals
  a lack of review.

  Fix: rename to order.service.ts
  In Go: rename to order_service.go
```

**Why:** Case-insensitive filesystems (macOS) allow `OrderService.ts` and `orderservice.ts` to coexist locally but collide catastrophically in Linux CI/CD. Consistent naming is a production stability concern, not just aesthetics.

---

## Testing

### [TEST-001] Missing test for new module/service

**Trigger:** A newly added file matching `*.service.ts`, `*.use-case.ts`, `*.handler.ts`, `*_service.py`, `*_handler.go` has no corresponding test file (`*.spec.ts`, `*.test.ts`, `*_test.go`, `test_*.py`) in the same directory or a sibling `__tests__/` directory.

**Message:**
```
TEST-001: New service/handler added without a co-located test file.
  File: src/subscriptions/subscription.service.ts
  Missing: src/subscriptions/subscription.service.spec.ts

  Untested services accumulate silently. By the time a bug surfaces in production
  the original author may be unavailable and the intent of the code is unclear.

  Fix: create a co-located test file before merging.
    // Minimum bar: one happy-path and one failure-path test.
    // Use an in-memory store or mock repository — no live database needed.

  If this is an intentional exception (e.g., a thin façade with zero logic):
    // korva-ignore: pure delegation, logic lives in OrderRepository (tested separately)
```

---

## Dependencies

### [DEPS-001] Known vulnerable package pattern

**Trigger:** Direct import or require of packages with well-known historical vulnerabilities that are commonly pinned to old versions:
- `lodash` below 4.17.21 (prototype pollution — CVE-2019-10744)
- `moment` (any version — signals a likely date library that should be replaced with `date-fns` or `Temporal`)
- `node-serialize` (any version — arbitrary code execution — CVE-2017-5941)
- `eval(` or `new Function(untrustedInput` in non-sandbox files
- `__import__('os').system(` in Python template strings

**Message:**
```
DEPS-001: Potentially vulnerable dependency or dangerous pattern detected.
  File: src/utils/date.helper.ts
  Found: import moment from 'moment'

  'moment' is in maintenance mode and its large bundle size and mutable API
  have caused numerous subtle bugs. For new code use date-fns, Luxon, or the
  native Temporal API.

  If you see lodash < 4.17.21: upgrade immediately — prototype pollution allows
  attackers to modify Object.prototype and affect all objects in the process.

  Fix:
    - Replace moment with date-fns: import { format } from 'date-fns'
    - Run: npm audit fix  (or pnpm audit --fix)
    - Pin safe versions in package.json and lock file.

  // korva-ignore: legacy integration test, scheduled for removal in v3
```

---

## Notes

- Rules are evaluated against `git diff --cached` (staged files only). Unstaged changes are not checked.
- Files matching `*.spec.ts`, `*.test.ts`, `*_test.go`, `test_*.py`, `__fixtures__/**`, `testdata/**` are excluded from SEC and QC checks unless explicitly noted.
- The `korva-ignore` comment must be on the **same line** as the violation and must include a reason: `// korva-ignore: <reason>`. A bare `// korva-ignore` without a reason is itself a violation of QC-002's spirit.
- Rule severity: `ARC-*` and `SEC-*` rules block the commit. `QC-*`, `NAM-*`, `TEST-*`, and `DEPS-001` emit warnings by default. Configure severity overrides in `.korva/sentinel.yaml`.
- To add a project-specific rule, create `.korva/rules/<rule-id>.yaml`. Custom rules follow the same format and integrate with the same suppression mechanism.
