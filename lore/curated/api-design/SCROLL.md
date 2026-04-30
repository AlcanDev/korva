---
id: api-design
version: 1.0.0
team: backend
stack: NestJS, OpenAPI, Swagger, REST, TypeScript, class-validator
last_updated: 2026-04-30
---

# Scroll: API Design — REST, OpenAPI, Versioning, Response Patterns

## Triggers — load when:
- Files: `*.controller.ts`, `*.dto.ts`, `openapi/**`, `docs/api/**`
- Keywords: REST, API design, versioning, OpenAPI, Swagger, endpoint, URL design, pagination, response envelope, HTTP status, DTO, validation

## Rules

### 1. URL design — nouns, versioned, kebab-case

```
# Structure
/{resource}/{version}/{collection}[/{id}][/{sub-collection}]

# Examples
GET    /insurance/v1/plans              # list plans
GET    /insurance/v1/plans/{id}         # single plan
POST   /insurance/v1/plans              # create
PATCH  /insurance/v1/plans/{id}         # partial update
DELETE /insurance/v1/plans/{id}         # delete

GET    /insurance/v1/plans/{id}/coverages   # sub-collection

# ❌ Verbs in URL
GET /insurance/v1/getPlans
POST /insurance/v1/calculatePremium
# ✅ Use nouns + HTTP method semantics
GET /insurance/v1/plans?country=CL
POST /insurance/v1/premium-calculations
```

### 2. Standard response envelope

```typescript
// Success — list with pagination
{
  "data": [
    { "id": "01HX...", "name": "Plan Básico", "monthlyPremium": 9990 }
  ],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "total": 47,
    "totalPages": 3,
    "hasNext": true,
    "hasPrev": false
  },
  "meta": {
    "requestId": "01HX...",
    "timestamp": "2025-04-14T10:00:00Z"
  }
}

// Success — single resource
{
  "data": { "id": "01HX...", ... },
  "meta": { "requestId": "01HX..." }
}

// Error — never expose internals
{
  "error": {
    "code": "INSURANCE_PLAN_NOT_FOUND",
    "message": "The requested insurance plan was not found",
    "requestId": "01HX..."
  }
}
```

### 3. DTO design — full validation + Swagger

```typescript
// Request DTO
export class GetInsurancePlansQueryDTO {
  @IsIn(['CL', 'PE', 'CO', 'MX', 'AR'])
  @ApiProperty({ enum: ['CL', 'PE', 'CO', 'MX', 'AR'], example: 'CL' })
  country!: string;

  @IsOptional()
  @IsIn(['BANCO', 'CMR', 'SEGUROS', 'LOYALTY'])
  @ApiPropertyOptional({ example: 'BANCO' })
  commerce?: string;

  @IsOptional()
  @IsInt()
  @Min(1)
  @Transform(({ value }) => parseInt(value, 10))
  @ApiPropertyOptional({ example: 1, default: 1 })
  page?: number = 1;

  @IsOptional()
  @IsInt()
  @Min(1)
  @Max(100)
  @Transform(({ value }) => parseInt(value, 10))
  @ApiPropertyOptional({ example: 20, default: 20 })
  pageSize?: number = 20;
}

// Response DTO
export class InsurancePlanResponseDTO {
  @ApiProperty({ example: '01HWKZEC7ZT9GJA6E5M5Y3Q6G5' })
  id!: string;

  @ApiProperty({ example: 'Plan Vida Básico' })
  name!: string;

  @ApiProperty({ example: 9990, description: 'Monthly premium in local currency' })
  monthlyPremium!: number;

  @ApiProperty({ enum: ['CLP', 'PEN', 'COP', 'MXN'] })
  currency!: string;

  @ApiPropertyOptional({ example: '2025-12-31' })
  promoUntil?: string;
}
```

### 4. Controller — thin orchestrator

```typescript
@ApiTags('insurance-plans')
@Controller('insurance/v1/plans')
@UseGuards(BearerAuthGuard)
export class InsurancePlansController {
  constructor(private readonly plansService: InsurancePlansService) {}

  @Get()
  @ApiOperation({ summary: 'List insurance plans by country' })
  @ApiHeader({ name: 'X-Country', required: true })
  @ApiHeader({ name: 'X-Commerce', required: false })
  @ApiResponse({ status: 200, type: InsurancePlanListResponseDTO })
  @ApiResponse({ status: 400, description: 'Invalid query parameters' })
  @ApiResponse({ status: 401, description: 'Missing or invalid Bearer token' })
  async findAll(
    @Query() query: GetInsurancePlansQueryDTO,
    @RequestHeader() headers: CommonHeadersRequestDTO,
  ): Promise<InsurancePlanListResponseDTO> {
    const command = GetInsurancePlansCommand.fromRequest(query, headers);
    const result = await this.plansService.findAll(command);
    return InsurancePlanListResponseDTO.fromDomain(result);
  }

  @Get(':id')
  @ApiOperation({ summary: 'Get insurance plan by ID' })
  @ApiResponse({ status: 200, type: InsurancePlanResponseDTO })
  @ApiResponse({ status: 404, description: 'Plan not found' })
  async findOne(@Param('id') id: string): Promise<InsurancePlanResponseDTO> {
    const plan = await this.plansService.findById(id);
    return InsurancePlanResponseDTO.fromDomain(plan);
  }
}
```

### 5. HTTP status codes

| Code | Meaning | When |
|------|---------|------|
| 200 | OK | GET, PATCH, PUT success |
| 201 | Created | POST that creates a resource |
| 204 | No Content | DELETE, or update with no body |
| 400 | Bad Request | Validation error |
| 401 | Unauthorized | Missing/invalid/expired token |
| 403 | Forbidden | Authenticated but not authorized |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Duplicate, concurrent modification |
| 422 | Unprocessable | Valid format, invalid business rule |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Error | Unexpected server error (no details) |
| 503 | Unavailable | Circuit breaker open, maintenance |

### 6. Versioning and deprecation

```typescript
// Add deprecation headers when v1 is being phased out
@Get()
getPlans(@Res({ passthrough: true }) res: Response) {
  if (this.featureFlags.isV2Ready) {
    res.setHeader('Deprecation', 'true');
    res.setHeader('Sunset', 'Sat, 31 Dec 2025 23:59:59 GMT');
    res.setHeader('Link', '</insurance/v2/plans>; rel="successor-version"');
  }
  // ...
}
```

**Versioning rules:**
- Non-breaking additions (new optional field): same version
- Removing a field or changing type: new version + 90-day deprecation window
- Never remove a version without confirming zero active consumers (check analytics)

---

## Anti-patterns

```typescript
// ❌ Business logic in controller
@Get() async getPlans() {
  const raw = await this.http.get('/api');
  return raw.filter(p => p.active && p.price > 0).map(p => ({ ...p, tax: p.price * 0.19 }));
}

// ❌ Returning 200 for errors
return { success: false, error: 'Not found' };  // → use 404

// ❌ No pagination on list endpoints (can return millions of rows)
@Get() async findAll() { return this.service.findAll(); }

// ❌ Exposing internal details in errors
throw new Error('Connection to insurance-provider-svc:8080 timed out');

// ❌ Missing API versioning from day 1
@Controller('insurance/plans')  // → should be 'insurance/v1/plans'

// ❌ Incomplete Swagger docs (DTOs without @ApiProperty)
```
