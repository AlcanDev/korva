---
applyTo: "**/*.ts,**/*.yaml,**/*.yml,Dockerfile,**/*.env*,**/*.json"
---

# Security — Secrets, Auth, OWASP, WAF

## Rule 0 — Secrets never in code

If it's a credential, token, key, or password: it belongs in HashiCorp Vault.
Not in `.env`. Not in `config.json`. Not in CI variables (except Vault token).

```typescript
// ❌ Any of these patterns → immediate rejection
const API_KEY = 'sk-prod-abc123';
const password = 'Admin1234!';
const connectionString = 'postgresql://user:pass@host/db';
process.env.TOKEN = 'hardcoded';

// ✅ Runtime injection via ConfigService (backed by Vault)
const apiKey = this.configService.get<string>('API_CONSUMER_KEY');
```

**Detection:** gitleaks scans every commit. Secret = build failure. No exceptions.

## Input validation (all external inputs)

```typescript
// Every DTO on a public endpoint must validate ALL fields
export class GetOffersRequestDTO {
  @IsString()
  @IsIn(['CL', 'PE', 'CO', 'MX', 'AR'])
  @ApiProperty({ example: 'CL' })
  country!: string;

  @IsString()
  @IsNotEmpty()
  @MaxLength(50)
  @Matches(/^[A-Z0-9-]+$/, { message: 'Invalid format' })
  commerce!: string;

  // Never expose internal error details
  // ValidationPipe with exceptionFactory hides stack traces
}

// Global ValidationPipe — always enabled
app.useGlobalPipes(new ValidationPipe({
  whitelist: true,           // strips undeclared properties
  forbidNonWhitelisted: true, // rejects unknown fields
  transform: true,
  exceptionFactory: (errors) => new BadRequestException(
    errors.map(e => Object.values(e.constraints ?? {})).flat()
  ),
}));
```

## Authentication and authorization

```typescript
// Every non-public route needs a guard
@Controller('insurance')
@UseGuards(BearerAuthGuard)  // validates JWT/Bearer token
export class InsuranceController {}

// Public routes must be explicitly marked
@Get('health')
@Public()  // custom decorator + guard that skips auth
healthCheck() { return { status: 'ok' }; }
```

## SQL injection prevention

```typescript
// ❌ String interpolation in any query
db.query(`SELECT * FROM users WHERE id = '${userId}'`);

// ✅ Parameterized always
db.query('SELECT * FROM users WHERE id = ?', [userId]);
// or TypeORM QueryBuilder:
repo.createQueryBuilder('u').where('u.id = :id', { id: userId });
```

## Response security

```typescript
// Sensitive fields excluded from serialization
export class UserResponseDTO {
  @Expose() id: string;
  @Expose() email: string;
  @Exclude() passwordHash: string;  // never in response
  @Exclude() internalId: string;
}

// Use ClassSerializerInterceptor globally
app.useGlobalInterceptors(new ClassSerializerInterceptor(app.get(Reflector)));
```

## API Gateway security headers (set by proxy — do not duplicate in app)

The API Gateway proxy adds these automatically. If your app sets them too, they'll be duplicated:
- `Content-Security-Policy: frame-ancestors 'none'`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `X-Content-Type-Options: nosniff`

Your app's responsibility: don't leak stack traces, internal service names, or versions in responses.

## WAF — request limits

Max 40 headers per request (8 auto-added by Cloudflare = 32 custom headers available).
If a feature requires many custom headers, review with Architecture first.

## Rate limiting

```typescript
// Use ThrottlerModule for public-facing endpoints
@Module({
  imports: [
    ThrottlerModule.forRoot([{
      name: 'short',
      ttl: 1000,   // 1 second
      limit: 10,
    }, {
      name: 'medium',
      ttl: 60_000, // 1 minute
      limit: 100,
    }]),
  ],
})
```

## Security checklist (Phase 5 — Verification)

Before marking a feature complete:
- [ ] No secrets in source code (`gitleaks detect` passes)
- [ ] All external inputs validated at DTO boundary
- [ ] No SQL string interpolation
- [ ] User-facing errors never expose stack traces or internal names
- [ ] Auth guards applied to all non-public routes
- [ ] Sensitive fields excluded from serialization (`@Exclude()`)
- [ ] Rate limiting on all public endpoints
- [ ] npm audit: no HIGH/CRITICAL vulnerabilities
- [ ] Docker: non-root USER, no hardcoded secrets in build args
- [ ] `vault/qa.hcl` and `vault/prod.hcl` updated for new secrets
- [ ] OTel alert configured for anomalous error rate spikes

## OWASP Top 10 — must-know for every developer

| Risk | Mitigation already in place |
|------|---------------------------|
| A01 Broken Access Control | BearerAuthGuard + role guards on every route |
| A02 Cryptographic Failures | Secrets in Vault, TLS enforced by Istio mesh |
| A03 Injection | Parameterized queries, DTO whitelist validation |
| A05 Security Misconfiguration | Docker hardening, K8s network policies, Istio |
| A06 Vulnerable Components | `npm audit` in CI, `gitleaks` in CI |
| A07 Auth Failures | OAuth2 client credentials, token expiry enforced, no sharing |
| A09 Logging Failures | OTel APM, LoggerInterceptor, no sensitive data in logs |
