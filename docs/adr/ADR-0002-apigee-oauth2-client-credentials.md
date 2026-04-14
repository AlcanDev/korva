# ADR-0002: Use OAuth 2.0 Client Credentials via Apigee for all API authentication

**Status:** Accepted  
**Date:** 2025-01-15  
**Authors:** [@alcandev]

---

## Context

Multiple BFFs need to consume downstream APIs. Each API has its own authentication mechanism. Managing credentials per-API creates an exponential complexity problem: 6 BFFs × 10 APIs = 60 credential sets to maintain. Additionally, developers were hardcoding credentials in `.env` files committed to GitLab, causing a security incident where dev credentials leaked in a PR review.

---

## Options Considered

### Option A: OAuth 2.0 Client Credentials via Apigee

**Description:** All APIs are exposed through Apigee. Each consumer (BFF) gets a consumerKey/consumerSecret pair. Tokens are requested from Apigee's token endpoint and cached locally.

**Pros:**
- Single credential model for all APIs
- Token expiry and rotation managed by Apigee
- WAF, rate limiting, and security headers included automatically
- Credentials managed centrally in HashiCorp Vault
- Audit trail: Apigee logs which app consumed which API, when

**Cons:**
- Token caching logic must be implemented in every BFF
- Extra network hop to Apigee token endpoint on first request
- WAF header limit (40) constrains header design

### Option B: Per-API API Keys

**Description:** Each downstream API authenticates with its own API key.

**Pros:**
- Simpler token management (no expiry)

**Cons:**
- Different auth mechanisms per API
- No centralized revocation
- No rate limiting or security headers
- API keys don't expire → larger blast radius if leaked

---

## Decision

**We choose Apigee OAuth 2.0 Client Credentials (Option A).**

The centralized credential model reduces the attack surface and simplifies audit compliance. The WAF and security header enforcement eliminates a whole class of OWASP vulnerabilities at the gateway. Token caching at 15-minute TTL is a one-time implementation cost that every BFF inherits from a shared base class.

---

## Consequences

**Positive:**
- One credential model, one token service implementation, reused everywhere
- Leaked dev credentials expire in 15 minutes maximum
- WAF blocks known attack patterns without app-level code
- Full request/response logging at Apigee for audit purposes

**Negative / Trade-offs:**
- Token refresh logic must be implemented and tested
- Team dependency on Apigee availability (mitigated by circuit breaker)
- Developer onboarding requires understanding consumerKey/consumerSecret vs Bearer token

**Risks:**
- Risk: Developer credentials used in production BFF deployment
  - Mitigation: Sentinel rule + Vault path naming convention separates dev vs app credentials
- Risk: Token not refreshed before expiry — API calls fail in production
  - Mitigation: Refresh 30s before expiry (buffer implementation required in OAuthTokenService)

---

## Implementation Notes

See `lore/private/apigee-oauth2/SCROLL.md` for complete implementation guide.

Key patterns:
- `OAuthTokenService` in `infrastructure/auth/` — handles caching + refresh
- Token TTL: 899s in all environments — refresh at 869s (30s buffer)
- Credentials from HashiCorp Vault only — never `.env` files
- Vault paths: `secret/data/fif/apps/{app-name}/{env}/config`
