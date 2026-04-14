---
id: apigee-oauth
version: 1.0.0
team: backend
stack: Apigee, OAuth2, REST, NestJS, Axios, Node.js
---

# Scroll: API Gateway — OAuth 2.0 Client Credentials

## Triggers — load when:
- Files: `*.service.ts`, `*.adapter.ts`, `http-client*`, `auth*`, `token*`
- Keywords: apigee, oauth, bearer, token, client_credentials, api_key, authorization, api gateway
- Tasks: consuming an external API, setting up HTTP client authentication, token management, debugging 401/403 errors

## Context

When your BFF consumes internal APIs through an API gateway (Apigee, Kong, AWS API Gateway, etc.), you need OAuth 2.0 Client Credentials flow. The most common mistake is requesting a new token per API call — this overloads the authorization server and is rejected by rate limiting. Tokens must be cached and refreshed proactively.

---

## Rules

### 1. Token caching — MANDATORY pattern

Never request a new token for every API call. Cache it and refresh proactively before it expires.

```typescript
// infrastructure/auth/oauth-token.service.ts
@Injectable()
export class OAuthTokenService {
  private cachedToken: string | null = null;
  private expiresAt = 0;
  // Refresh 30 seconds before actual expiry to avoid race conditions
  private readonly BUFFER_MS = 30_000;

  constructor(
    private readonly httpService: HttpService,
    private readonly configService: ConfigService,
  ) {}

  async getToken(): Promise<string> {
    const hasValidToken = this.cachedToken && Date.now() < this.expiresAt - this.BUFFER_MS;
    if (hasValidToken) return this.cachedToken!;
    return this.refreshToken();
  }

  private async refreshToken(): Promise<string> {
    const key = this.configService.get<string>('OAUTH_CLIENT_KEY');
    const secret = this.configService.get<string>('OAUTH_CLIENT_SECRET');
    const tokenUrl = this.configService.get<string>('OAUTH_TOKEN_URL');

    // Client Credentials: Basic auth = Base64(key:secret)
    const basicAuth = Buffer.from(`${key}:${secret}`).toString('base64');

    const response = await this.httpService.post<TokenResponse>(tokenUrl, {
      body: 'grant_type=client_credentials',
      headers: {
        'Authorization': `Basic ${basicAuth}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
    });

    const { access_token, expires_in } = response.data;
    this.cachedToken = access_token;
    // expires_in may come as string or number
    this.expiresAt = Date.now() + parseInt(String(expires_in), 10) * 1_000;
    return this.cachedToken;
  }
}

interface TokenResponse {
  access_token: string;
  expires_in: string | number;
  token_type: 'Bearer';
}
```

---

### 2. Inject OAuthTokenService into adapters, never services

The token concern lives in the infrastructure layer. Services only call ports.

```typescript
// infrastructure/adapters/insurance.adapter.base.ts
@Injectable()
export abstract class InsuranceAdapterBase implements InsurancePort {
  constructor(
    protected readonly httpService: HttpService,
    protected readonly tokenService: OAuthTokenService,
    protected readonly configService: ConfigService,
  ) {}

  async getOffers(command: GetInsuranceOffersCommand): Promise<InsuranceOffer[]> {
    const token = await this.tokenService.getToken();
    const response = await this.httpService.get<InsuranceOfferListDTO>(
      this.getOffersUrl(command),
      { headers: this.buildHeaders(token, command) },
    );
    return this.mapOffers(response.data);
  }

  protected abstract getOffersUrl(command: GetInsuranceOffersCommand): string;
  protected abstract buildHeaders(token: string, command: GetInsuranceOffersCommand): Record<string, string>;
  protected abstract mapOffers(dto: InsuranceOfferListDTO): InsuranceOffer[];
}
```

---

### 3. Credentials belong in the secrets manager, never in code

```typescript
// ❌ Hardcoded
const key = 'abc123';

// ✅ Runtime secret via ConfigService (backed by HashiCorp Vault, AWS Secrets Manager, etc.)
const key = this.configService.get<string>('OAUTH_CLIENT_KEY');
```

Environment variables needed (document in vault/qa.hcl and vault/prod.hcl):
```
OAUTH_TOKEN_URL      = token endpoint URL
OAUTH_CLIENT_KEY     = consumerKey / clientId
OAUTH_CLIENT_SECRET  = consumerSecret / clientSecret
```

---

### 4. Building the Basic Auth header correctly

```typescript
// Correct: Base64(key:secret) — colon between them
const basicAuth = Buffer.from(`${key}:${secret}`).toString('base64');
const header = `Basic ${basicAuth}`;

// Common mistake: encoding key and secret separately — WRONG
const wrong = `Basic ${Buffer.from(key).toString('base64')}:${Buffer.from(secret).toString('base64')}`;
```

---

### 5. Token expiry — use expires_in, not a fixed timer

```typescript
// ❌ Fixed 15-minute timer — ignores what the server actually sent
setInterval(() => this.refreshToken(), 15 * 60 * 1000);

// ✅ Dynamic based on server response
this.expiresAt = Date.now() + parseInt(String(expires_in), 10) * 1_000;
```

---

### 6. Handle token errors with graceful retry

```typescript
async callWithTokenRetry<T>(fn: (token: string) => Promise<T>): Promise<T> {
  const token = await this.tokenService.getToken();
  try {
    return await fn(token);
  } catch (err) {
    // If 401 — token may have expired mid-request, refresh once and retry
    if (err?.response?.status === 401) {
      this.tokenService.invalidate(); // expose invalidate() in OAuthTokenService
      const freshToken = await this.tokenService.getToken();
      return fn(freshToken);
    }
    throw err;
  }
}
```

---

### 7. Testing — mock the token service, not the HTTP call

```typescript
describe('InsuranceAdapterCL', () => {
  let adapter: InsuranceAdapterCL;
  let tokenMock: jest.Mocked<OAuthTokenService>;
  let httpMock: jest.Mocked<HttpService>;

  beforeEach(async () => {
    tokenMock = { getToken: jest.fn().mockResolvedValue('mock-bearer-token') };
    httpMock = { get: jest.fn() };

    const module = await Test.createTestingModule({
      providers: [
        InsuranceAdapterCL,
        { provide: OAuthTokenService, useValue: tokenMock },
        { provide: HttpService, useValue: httpMock },
        { provide: ConfigService, useValue: { get: jest.fn().mockReturnValue('test') } },
      ],
    }).compile();

    adapter = module.get(InsuranceAdapterCL);
  });

  it('should pass bearer token in Authorization header', async () => {
    httpMock.get.mockResolvedValue({ data: { offers: [] } });
    await adapter.getOffers(validCommand);
    expect(httpMock.get).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer mock-bearer-token' }),
      }),
    );
  });
});
```

---

## Anti-patterns

```typescript
// ❌ New token per call
async getOffers() {
  const res = await this.http.post(TOKEN_URL, ...); // every time!
  return this.http.get(API_URL, { headers: { Authorization: `Bearer ${res.data.access_token}` } });
}

// ❌ Token in application layer
// application/insurance.service.ts
async getOffers(command) {
  const token = await this.http.post(TOKEN_URL, ...); // wrong layer
}

// ❌ Ignoring expires_in
const TOKEN_TTL = 900_000; // hardcoded — breaks when server changes expiry

// ❌ Catching all errors silently
try {
  return await this.callApi();
} catch {
  return []; // silent failure masks auth problems
}
```
