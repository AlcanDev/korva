---
id: security-patterns
version: 1.0.0
team: backend
stack: Node.js, TypeScript, Python, Go, any
---

# Scroll: Security Patterns — Production Hardening

## Triggers — load when:
- Files: `auth.ts`, `auth.py`, `auth.go`, `middleware.ts`, `guard.ts`, `jwt.ts`, `session.ts`, `password.ts`, `crypto.ts`, `security.ts`, `cors.ts`, `helmet.ts`, `rate-limit.ts`
- Keywords: `authentication`, `authorization`, `security`, `password`, `token`, `jwt`, `bcrypt`, `secret`, `rbac`, `cors`, `csrf`, `xss`, `injection`, `sanitize`, `rate limit`, `session`
- Tasks: implementing login, writing auth middleware, securing endpoints, password hashing, configuring CORS, adding rate limiting

## Context
Security failures are not hypothetical. The OWASP Top 10 changes slowly because the same categories of vulnerabilities — broken authentication, injection, misconfigured security controls — appear in production systems year after year. These patterns encode the decisions that are easy to get wrong under deadline pressure: JWT algorithm confusion, timing attacks on secret comparison, logging passwords by accident, CORS wildcards in production configs.

---

## Rules

### 1. Hash passwords with bcrypt, scrypt, or Argon2 — never MD5/SHA-*/plain

One-way adaptive hashing is non-negotiable for stored passwords. MD5 and SHA-* are fast by design — a modern GPU can test billions of MD5 hashes per second. bcrypt with a work factor of 12+ is the minimum bar; Argon2id is the current recommendation for new systems.

```typescript
import bcrypt from 'bcrypt';

const BCRYPT_ROUNDS = 12; // ~250ms on a modern server — tune to ~200-300ms

async function hashPassword(plaintext: string): Promise<string> {
  return bcrypt.hash(plaintext, BCRYPT_ROUNDS);
}

async function verifyPassword(plaintext: string, hash: string): Promise<boolean> {
  // bcrypt.compare is already constant-time — safe against timing attacks
  return bcrypt.compare(plaintext, hash);
}
```

```python
# Python — use passlib or bcrypt
from passlib.context import CryptContext

pwd_context = CryptContext(schemes=["argon2", "bcrypt"], deprecated="auto")

def hash_password(plaintext: str) -> str:
    return pwd_context.hash(plaintext)

def verify_password(plaintext: str, hashed: str) -> bool:
    return pwd_context.verify(plaintext, hashed)
```

**Never:** `md5(password)`, `sha256(password)`, `sha256(salt + password)` for stored credentials. These are all crackable offline.

---

### 2. JWT: validate algorithm, expiry, and audience explicitly

JWT libraries that trust the `alg` header can be exploited with algorithm confusion attacks (`alg: none`, RS256→HS256 confusion). Always pin the expected algorithm and validate `exp`, `iss`, and `aud` claims.

```typescript
import jwt from 'jsonwebtoken';

const JWT_SECRET = process.env.JWT_SECRET!; // minimum 256-bit random value
const JWT_EXPIRY = '15m'; // short-lived access tokens
const REFRESH_EXPIRY = '7d';

function signAccessToken(userId: string, roles: string[]): string {
  return jwt.sign(
    { sub: userId, roles, iss: 'api.example.com', aud: 'web' },
    JWT_SECRET,
    { algorithm: 'HS256', expiresIn: JWT_EXPIRY }
  );
}

function verifyAccessToken(token: string): jwt.JwtPayload {
  return jwt.verify(token, JWT_SECRET, {
    algorithms: ['HS256'],    // NEVER ['RS256', 'HS256'] together — algorithm confusion
    issuer: 'api.example.com',
    audience: 'web',
  }) as jwt.JwtPayload;
}

// Express middleware
function requireAuth(req: Request, res: Response, next: NextFunction) {
  const authHeader = req.headers.authorization;
  if (!authHeader?.startsWith('Bearer ')) {
    return res.status(401).json({ error: 'Missing authorization header' });
  }

  try {
    req.user = verifyAccessToken(authHeader.slice(7));
    next();
  } catch {
    // Do NOT distinguish between expired and invalid — information leakage
    res.status(401).json({ error: 'Invalid or expired token' });
  }
}
```

**Access token lifespan:** 15 minutes. Use a refresh token (7–30 days, stored HttpOnly cookie) to issue new access tokens. Short access token lifetimes limit the blast radius of a stolen token.

---

### 3. Use constant-time comparison for all secret equality checks

Every `===` comparison on a secret value is a timing oracle. The fix is one line, has zero cost, and is required for HMAC validation, API key verification, and any custom auth token check.

```typescript
import { timingSafeEqual } from 'crypto';

// BAD — timing attack
if (receivedHmac === expectedHmac) { ... }

// GOOD — constant time
function safeCompare(a: string, b: string): boolean {
  const bufA = Buffer.from(a);
  const bufB = Buffer.from(b);
  // Must be same length for timingSafeEqual — pad or hash to a fixed digest first
  if (bufA.length !== bufB.length) return false;
  return timingSafeEqual(bufA, bufB);
}

// For HMAC validation (webhook signature check):
function verifyHmac(payload: Buffer, receivedSig: string, secret: string): boolean {
  const expected = createHmac('sha256', secret).update(payload).digest('hex');
  return safeCompare(receivedSig, expected);
}
```

```go
// Go
import "crypto/subtle"

func safeCompare(a, b string) bool {
    return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
```

```python
# Python
import hmac
def safe_compare(a: str, b: str) -> bool:
    return hmac.compare_digest(a.encode(), b.encode())
```

---

### 4. RBAC — check permissions, not just roles

Role-Based Access Control is a starting point, not an end state. Checking `user.role === 'admin'` is coarse. Checking `can(user, 'read', 'invoice')` is resilient to role creep and supports audit trails.

```typescript
// Simple RBAC with explicit permission matrix
type Permission = 'invoices:read' | 'invoices:write' | 'users:read' | 'users:write' | 'admin:*';
type Role = 'viewer' | 'editor' | 'admin';

const PERMISSIONS: Record<Role, Permission[]> = {
  viewer: ['invoices:read', 'users:read'],
  editor: ['invoices:read', 'invoices:write', 'users:read'],
  admin: ['admin:*'],
};

function can(userRoles: Role[], permission: Permission): boolean {
  return userRoles.some(role => {
    const perms = PERMISSIONS[role] ?? [];
    return perms.includes(permission) || perms.includes('admin:*');
  });
}

// Route-level guard
function requirePermission(permission: Permission) {
  return (req: Request, res: Response, next: NextFunction) => {
    if (!can(req.user.roles, permission)) {
      return res.status(403).json({ error: 'Insufficient permissions' });
    }
    next();
  };
}

// Usage
router.delete('/invoices/:id', requireAuth, requirePermission('invoices:write'), deleteInvoice);
```

**Object-level authorization:** Always check that the authenticated user owns or has access to the specific resource, not just that they have the role. `GET /users/456` should verify `req.user.id === '456' || can(req.user.roles, 'users:read')`.

---

### 5. Input validation and sanitization — validate at the boundary

Never trust data from HTTP request bodies, query parameters, headers, or external APIs. Validate shape and type at the entry point. This prevents injection, crashes from unexpected types, and unexpected behavior from malformed input.

```typescript
import { z } from 'zod';

const CreateUserSchema = z.object({
  email: z.string().email().max(255),
  name: z.string().min(1).max(100).regex(/^[\p{L}\p{N}\s'-]+$/u), // unicode-aware
  age: z.number().int().min(13).max(120).optional(),
  role: z.enum(['viewer', 'editor']), // strict enum — never accept arbitrary strings for role
});

async function createUser(req: Request, res: Response) {
  const result = CreateUserSchema.safeParse(req.body);
  if (!result.success) {
    return res.status(400).json({ errors: result.error.flatten() });
  }
  const data = result.data; // fully typed and validated
  await userService.create(data);
}
```

**SQL injection:** Use parameterized queries. Never build SQL with string interpolation (see SEC-005).

**XSS:** Escape HTML output server-side if rendering HTML. In REST APIs, set `Content-Type: application/json` — this prevents the browser treating JSON as HTML.

**SSRF (Server-Side Request Forgery):** If your code fetches URLs supplied by users, validate against an allowlist of domains. Block private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8, 169.254.0.0/16).

```typescript
import { URL } from 'url';

const ALLOWED_DOMAINS = new Set(['api.partner.com', 'cdn.assets.com']);

function validateExternalUrl(rawUrl: string): URL {
  const url = new URL(rawUrl); // throws if malformed
  if (!ALLOWED_DOMAINS.has(url.hostname)) {
    throw new Error(`Domain not allowed: ${url.hostname}`);
  }
  return url;
}
```

---

### 6. Secrets management — env vars, secret stores, never in code

```typescript
// Twelve-factor: read secrets from environment at startup — never from config files committed to git
const config = {
  jwtSecret: requireEnv('JWT_SECRET'),           // throw at startup if missing
  databaseUrl: requireEnv('DATABASE_URL'),
  stripeKey: requireEnv('STRIPE_SECRET_KEY'),
  redisUrl: process.env.REDIS_URL ?? 'redis://localhost:6379', // optional with default
};

function requireEnv(key: string): string {
  const value = process.env[key];
  if (!value) throw new Error(`Required environment variable ${key} is not set`);
  return value;
}
```

**Secret length requirements:**
- JWT secrets: minimum 256 bits (32 bytes) of random data — `openssl rand -hex 32`
- HMAC keys: minimum 256 bits
- Session secrets: minimum 256 bits

**Secret rotation:** Design systems to accept two valid secrets simultaneously during rotation. Rotate secrets using a key-versioning strategy rather than a cut-over.

**Never:**
- Secrets in `.env` files committed to git (even private repos)
- Secrets in code comments
- Secrets in environment variable names that are logged (`LOG_LEVEL=info DATABASE_URL=postgres://...`)

---

### 7. Rate limiting — protect authentication endpoints especially

Without rate limiting, authentication endpoints are open to brute force and credential stuffing attacks. The rate limit on `/auth/login` should be significantly stricter than general API limits.

```typescript
import rateLimit from 'express-rate-limit';
import RedisStore from 'rate-limit-redis';

// Strict limit for auth endpoints — 5 attempts per 15 minutes per IP
const authLimiter = rateLimit({
  windowMs: 15 * 60 * 1000,
  max: 5,
  standardHeaders: true,
  legacyHeaders: false,
  store: new RedisStore({ client: redisClient }), // use Redis in distributed deployments
  message: { error: 'Too many login attempts. Try again in 15 minutes.' },
  skipSuccessfulRequests: true, // only count failed attempts
});

// General API limit — 100 req/min per authenticated user
const apiLimiter = rateLimit({
  windowMs: 60 * 1000,
  max: 100,
  keyGenerator: (req) => req.user?.id ?? req.ip, // per-user, not just per-IP
});

app.use('/auth/login', authLimiter);
app.use('/api/', apiLimiter);
```

**Account lockout:** Consider a progressive lockout on login failures (5 failures → 15 min lockout, 10 → 1 hour) tracked per user ID, not just IP (VPNs share IPs).

---

### 8. Security headers — configure with Helmet

A single middleware call applies the most important security response headers. Never ship without these in production.

```typescript
import helmet from 'helmet';

app.use(helmet({
  contentSecurityPolicy: {
    directives: {
      defaultSrc: ["'self'"],
      scriptSrc: ["'self'"],        // no inline scripts, no CDN without explicit allowlist
      styleSrc: ["'self'", "'unsafe-inline'"], // relax for styled-components if needed
      imgSrc: ["'self'", 'data:', 'https:'],
      connectSrc: ["'self'"],
      frameSrc: ["'none'"],
      objectSrc: ["'none'"],
      upgradeInsecureRequests: [],
    },
  },
  hsts: {
    maxAge: 31_536_000, // 1 year in seconds
    includeSubDomains: true,
    preload: true,
  },
  xFrameOptions: { action: 'deny' },
  noSniff: true,       // X-Content-Type-Options: nosniff
  referrerPolicy: { policy: 'strict-origin-when-cross-origin' },
}));
```

**CORS — explicit allowlist:**
```typescript
import cors from 'cors';

const allowedOrigins = (process.env.CORS_ORIGINS ?? '').split(',').map(s => s.trim());

app.use(cors({
  origin: (origin, callback) => {
    if (!origin || allowedOrigins.includes(origin)) {
      callback(null, true);
    } else {
      callback(new Error(`Origin ${origin} not allowed by CORS`));
    }
  },
  credentials: true,
  methods: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'],
  allowedHeaders: ['Content-Type', 'Authorization'],
}));
```

---

### 9. Session security — HttpOnly, Secure, SameSite

```typescript
import session from 'express-session';

app.use(session({
  secret: requireEnv('SESSION_SECRET'),
  name: '__Host-sid',  // __Host- prefix: forces Secure + path=/ — browser enforced
  resave: false,
  saveUninitialized: false,
  cookie: {
    httpOnly: true,           // inaccessible to JavaScript — prevents XSS token theft
    secure: process.env.NODE_ENV === 'production', // HTTPS only in production
    sameSite: 'lax',          // CSRF protection — use 'strict' for sensitive forms
    maxAge: 24 * 60 * 60 * 1000, // 24 hours
  },
}));
```

**JWT in cookies vs Authorization header:**
- HttpOnly cookie: immune to XSS but requires CSRF protection (use `SameSite=Strict/Lax` + CSRF token for state-changing requests)
- Authorization header (localStorage/memory): immune to CSRF but vulnerable to XSS

For most apps: JWT in HttpOnly cookie with `SameSite=Lax` is the better tradeoff.

---

## Anti-Patterns

### BAD: MD5/SHA-1 for password hashing
```typescript
// BAD — GPU can crack 10 billion MD5 hashes/second
import crypto from 'crypto';
const hash = crypto.createHash('md5').update(password).digest('hex');
```

```typescript
// GOOD — adaptive slow hash
const hash = await bcrypt.hash(password, 12);
```

### BAD: JWT algorithm confusion
```typescript
// BAD — allows algorithm confusion: attacker sets alg: "none"
jwt.verify(token, secret); // uses algorithm from token header
```

```typescript
// GOOD — algorithm pinned by the verifier
jwt.verify(token, secret, { algorithms: ['HS256'] });
```

### BAD: CORS wildcard with credentials
```typescript
// BAD — wildcard + credentials is blocked by browsers AND insecure
app.use(cors({ origin: '*', credentials: true }));
```

```typescript
// GOOD — explicit origin with credentials
app.use(cors({ origin: 'https://app.example.com', credentials: true }));
```

### BAD: Direct string comparison for HMAC signatures
```typescript
// BAD — timing oracle
if (req.headers['x-webhook-sig'] === computedSig) { ... }
```

```typescript
// GOOD — constant time
if (timingSafeEqual(Buffer.from(received), Buffer.from(expected))) { ... }
```

### BAD: Secrets in config files
```typescript
// BAD — committed to git, readable by everyone with repo access
export const config = {
  jwtSecret: 'super-secret-key-123',
  dbPassword: 'postgres123',
};
```

```typescript
// GOOD — from environment, validated at startup
export const config = {
  jwtSecret: requireEnv('JWT_SECRET'),
  dbPassword: requireEnv('DB_PASSWORD'),
};
```

---

## Quick Reference

| Concern | Pattern |
|---|---|
| Password storage | bcrypt (rounds ≥ 12) or Argon2id |
| JWT verification | Pin algorithm + validate exp, iss, aud |
| Secret comparison | `timingSafeEqual` / `hmac.compare_digest` / `subtle.ConstantTimeCompare` |
| Input validation | Zod / Pydantic / `go-playground/validator` at the HTTP boundary |
| SQL injection | Parameterized queries — never string interpolation |
| CORS | Explicit origin allowlist from env var |
| Security headers | Helmet (Node.js) — CSP, HSTS, X-Frame-Options |
| Rate limiting | `express-rate-limit` with Redis store — strict on `/auth/*` |
| Session cookies | `HttpOnly + Secure + SameSite=Lax` |
| Secrets | Environment variables — validated at startup, never in code |
