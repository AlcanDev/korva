---
applyTo: "**/*.controller.ts,**/*.dto.ts,**/*.swagger.ts,openapi/**,docs/api/**"
---

# API Design — REST, OpenAPI, Versioning

## URL design rules

```
GET    /insurance/v1/offers              # list
GET    /insurance/v1/offers/{id}         # single resource
POST   /insurance/v1/offers              # create
PUT    /insurance/v1/offers/{id}         # full replace
PATCH  /insurance/v1/offers/{id}         # partial update
DELETE /insurance/v1/offers/{id}         # delete

# ✅ Nouns, plural, kebab-case
GET /credit-cards/v1/credit-cards/{id}/transactions

# ❌ Verbs in URL
GET /insurance/v1/getOffers
POST /insurance/v1/calculatePrice
```

## Versioning (mandatory)

All APIs must be versioned from day 1. Version in the URL path (not header):
- Current: `/v1/`
- Breaking change: `/v2/` (keep v1 alive for at least 1 deprecation cycle)
- Non-breaking additions: same version, documented in changelog

## Standard response envelope

```typescript
// Success list
{
  "data": [...],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "total": 100,
    "totalPages": 5
  },
  "meta": { "requestId": "ulid-here" }
}

// Success single
{
  "data": { ... },
  "meta": { "requestId": "ulid-here" }
}

// Error (never expose stack traces or internal service names)
{
  "error": {
    "code": "INSURANCE_NOT_FOUND",
    "message": "Insurance plan not found",
    "requestId": "ulid-here"
  }
}
```

## OpenAPI / Swagger annotations (mandatory on all DTOs)

```typescript
export class InsuranceOfferResponseDTO {
  @ApiProperty({ description: 'Unique offer ID', example: '01HX...' })
  id!: string;

  @ApiProperty({ description: 'Monthly premium in local currency', example: 15990 })
  monthlyPremium!: number;

  @ApiProperty({ enum: ['CLP', 'PEN', 'COP'], example: 'CLP' })
  currency!: string;

  @ApiPropertyOptional({ description: 'Promotional price valid until date' })
  promoUntil?: Date;
}

@ApiTags('insurance')
@Controller('insurance/v1')
export class InsuranceController {
  @Get('offers')
  @ApiOperation({ summary: 'List available insurance offers by country' })
  @ApiHeader({ name: 'X-Country', required: true, example: 'CL' })
  @ApiResponse({ status: 200, type: InsuranceOfferListResponseDTO })
  @ApiResponse({ status: 400, description: 'Missing required headers' })
  @ApiResponse({ status: 401, description: 'Invalid or expired token' })
  async getOffers(@RequestHeader() headers: CommonHeadersRequestDTO) {}
}
```

## Pagination (all list endpoints)

```typescript
export class PaginationQueryDTO {
  @IsOptional()
  @IsInt()
  @Min(1)
  @Transform(({ value }) => parseInt(value, 10))
  page: number = 1;

  @IsOptional()
  @IsInt()
  @Min(1)
  @Max(100)
  @Transform(({ value }) => parseInt(value, 10))
  pageSize: number = 20;
}
```

## Required headers (all BFF endpoints)

```typescript
export class CommonHeadersRequestDTO {
  @IsString()
  @IsIn(['CL', 'PE', 'CO', 'MX', 'AR'])
  'x-country'!: string;

  @IsString()
  @IsNotEmpty()
  'x-commerce'!: string;

  @IsString()
  @IsIn(['Web', 'Mobile', 'IVR', 'API'])
  'x-channel'!: string;
}
```

## HTTP status codes — correct usage

| Status | When |
|--------|------|
| 200 | Successful GET, PATCH, PUT |
| 201 | Successful POST that creates a resource |
| 204 | Successful DELETE or update with no response body |
| 400 | Validation error (bad input from client) |
| 401 | Not authenticated (missing/invalid token) |
| 403 | Authenticated but not authorized |
| 404 | Resource not found |
| 409 | Conflict (duplicate, state mismatch) |
| 422 | Semantically invalid (valid format, invalid business logic) |
| 500 | Internal error (never expose details) |
| 503 | Service unavailable (circuit breaker open, maintenance) |

## Forbidden patterns

```typescript
// ❌ Business logic in controller
@Get('offers') async getOffers() {
  const raw = await this.http.get('/api');
  return raw.data.filter(x => x.active && x.price > 0);
}

// ❌ Non-versioned endpoints in production
@Controller('insurance')  // → should be 'insurance/v1'

// ❌ 200 for errors
return res.status(200).json({ error: 'Not found' });

// ❌ Exposing internal IDs or service names in errors
throw new Error('PostgreSQL: relation "users" does not exist');

// ❌ No OpenAPI decorators on controllers/DTOs
// ❌ Pagination without limits (can return millions of rows)
```
