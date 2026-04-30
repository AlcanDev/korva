# ADR-0001: Use hexagonal architecture for all BFF services

**Status:** Accepted  
**Date:** 2025-01-15  
**Authors:** [@alcandev]

---

## Context

Our BFF services started as simple NestJS controllers that called external APIs directly. As business requirements grew, we accumulated controllers with 200+ lines of business logic, making them impossible to unit test without mocking HTTP calls. Adding a second country (PE after CL) required copying entire modules. The lack of clear layer boundaries made onboarding new developers slow — it was unclear where to add new logic.

---

## Options Considered

### Option A: Hexagonal Architecture (Ports & Adapters)

**Description:** Separate code into Domain (business logic), Application (orchestration), and Infrastructure (I/O). Business logic is expressed through interfaces (ports). Country-specific implementations are adapters.

**Pros:**
- Domain layer testable with zero framework setup
- Adding a country = adding one adapter class, zero changes to service
- Clear rules: "if it imports Fastify, it belongs in Infrastructure"
- Forces separation of concerns — no accidental coupling

**Cons:**
- More files per feature (port, service, adapter, module, DTO)
- Steeper learning curve for developers new to the pattern
- Verbose dependency injection wiring in modules

### Option B: Layered Architecture (Controller → Service → Repository)

**Description:** Traditional N-tier: controller calls service, service calls repository or HTTP client.

**Pros:**
- Familiar to most developers
- Less boilerplate

**Cons:**
- No clear rule against calling HTTP directly in service
- Country-specific logic ends up as conditionals
- Hard to test without real HTTP infrastructure

---

## Decision

**We choose hexagonal architecture (Option A).**

The team adds a country every 6-12 months and has 15+ developers working in parallel. The Template Method pattern in infrastructure adapters makes country expansion predictable and safe. The ability to test all business logic without mocking HTTP calls is worth the additional file count. The clear layer rules make code review and onboarding faster in the long run.

---

## Consequences

**Positive:**
- Domain layer tests run in milliseconds, no HTTP stubs needed
- Adding CL, PE, CO adapters follows a clear, repeatable pattern
- New developers have an explicit rule: "never import infrastructure from domain"
- Bugs in country-specific behavior are isolated to adapter classes

**Negative / Trade-offs:**
- Each feature requires 5-7 files (port, command, service, adapter base, adapter CL/PE/CO, module)
- NestJS DI token pattern (`INSURANCE_PORT = 'InsurancePort'`) is non-obvious to newcomers

**Risks:**
- Risk: developers bypass ports by importing adapters directly in services
  - Mitigation: Korva Sentinel rule HEX-002 catches this in pre-commit hooks

---

## Implementation Notes

```
src/
  domain/
    ports/       ← interfaces + DI tokens
    entities/    ← business objects
    commands/    ← CQRS command objects
    errors/      ← domain-specific errors
  application/
    services/    ← orchestration via ports only
  infrastructure/
    adapters/    ← HTTP clients implementing ports
      base/      ← Template Method base class
      cl/        ← CL-specific override
      pe/        ← PE-specific override
    controllers/ ← HTTP entry points
    dto/         ← validation DTOs
```

See `.github/instructions/backend.instructions.md` for naming conventions and code examples.

*Last updated: 2026-04-30*
