---
id: forge-sdd
version: 1.0.0
team: all
stack: Korva Forge, Korva Vault, AI-assisted development workflow
---

# Scroll: Forge SDD Workflow

## Triggers — load when:
- Files: `SPEC.md`, `DESIGN.md`, `sdd/**`, `specs/**`
- Keywords: Forge, SDD, spec-driven, exploration, specification, design, implementation, verification, vault_search, vault_save, approval, checkpoint, phase
- Tasks: starting a new feature, writing a spec, designing a solution, implementing from a spec, reviewing an implementation against its spec

## Context
Forge is Korva's spec-driven development orchestrator for Acme Financiero teams. It enforces a structured 5-phase workflow where AI and developer collaborate — the AI proposes, the developer approves at critical checkpoints before any code is generated. Vault is always consulted before designing, to reuse existing solutions and avoid duplication.

---

## Rules

### 1. The 5 phases — overview

| Phase | Name | AI action | Developer action |
|---|---|---|---|
| 1 | Exploration | Research codebase, ask clarifying questions | Provide context, answer questions |
| 2 | Specification | Draft spec document | **Approve or reject spec** |
| 3 | Design | Draft architecture/design | **Approve or reject design** |
| 4 | Implementation | Generate code | Review output |
| 5 | Verification | Run tests, check coverage | Accept or request fixes |

Phases 2 and 3 require explicit approval before proceeding. The AI must never advance past a checkpoint without an affirmative signal from the developer.

### 2. Phase 1 — Exploration

The AI searches the Vault and the codebase before asking questions. The goal is to understand what already exists.

```
AI workflow for Phase 1:
1. vault_search("GetInsuranceOffersCommand") — check for prior art
2. vault_search("InsurancePort") — check for existing ports
3. Search codebase for related files
4. Summarize findings to developer
5. Ask focused clarifying questions (max 5)
```

The developer provides context that could not be found automatically: business rules, deadlines, constraints, country-specific behavior.

### 3. Phase 2 — Specification (requires approval)

The spec document must follow this exact format:

```markdown
## Specification: [Feature Name]

**Objective**
One sentence. What this feature must accomplish.

**Inputs**
- GetInsuranceOffersCommand { customerId: CustomerId, productId: string, country: CountryCode }
- CommonHeadersRequestDTO { channel: string, sessionId: string }

**Outputs**
- Success: InsuranceOffer[]
- Failure: InsuranceError (OFFER_NOT_FOUND | PROVIDER_UNAVAILABLE)

**Constraints**
- Response time < 500ms (p95)
- Must support CL, PE, CO via country-specific adapters
- No database access — calls EXTERNAL_API via HttpService

**Affects**
- InsuranceModule, InsurancePort, InsuranceService
- New: LifeInsuranceAdapterCL, LifeInsuranceAdapterPE
- Existing: CommonHeadersRequestDTO (no change)
```

The AI must present this document and wait for explicit approval (`✅ approved` or equivalent) before moving to Phase 3.

### 4. Phase 3 — Design (requires approval)

After spec approval, the AI proposes the solution architecture. This includes:

- Directory structure with file names
- Class/interface/type definitions (signatures only, no implementation)
- Data flow diagram (text-based is fine)
- Dependency graph between new and existing components

```
Proposed structure for GetInsuranceOffers:

domain/
  ports/
    insurance.port.ts              (new — InsurancePort interface + INSURANCE_PORT token)
  commands/
    get-insurance-offers.command.ts (new — GetInsuranceOffersCommand value object)
  value-objects/
    insurance-offer.ts             (new — InsuranceOffer discriminated union)

application/
  insurance.service.ts             (new — InsuranceService, injects INSURANCE_PORT)

infrastructure/
  adapters/
    life-insurance.adapter.base.ts (new — abstract, Template Method)
    life-insurance.adapter.cl.ts   (new — CL concrete adapter)
    life-insurance.adapter.pe.ts   (new — PE concrete adapter)
  controllers/
    insurance.controller.ts        (new — orchestrates InsuranceService)

insurance.module.ts                (new — wires PORT_TOKEN → LifeInsuranceAdapterCL)
```

The AI must wait for explicit approval (`✅ approved`) before Phase 4.

### 5. Phase 4 — Implementation

Code is generated only after both spec and design are approved. Implementation follows all applicable Scrolls (nestjs-hexagonal, typescript, testing-jest).

```
AI workflow for Phase 4:
1. Generate files in dependency order (domain → application → infrastructure)
2. Each file is presented for review before the next is generated
3. Tests are generated alongside implementation files (not after)
4. No TODO comments — complete implementations only
```

### 6. Phase 5 — Verification

```
AI workflow for Phase 5:
1. Run: nx affected:test
2. Report coverage against 90% threshold
3. If tests fail: diagnose, fix, re-run (do not skip)
4. vault_save(featureName, specAndDesignSummary) — persist knowledge
5. Present completion summary to developer
```

### 7. Vault integration

```
Before Phase 2 starts:
  vault_search("<feature keywords>")
  vault_search("<domain entity names>")
  → Reuse existing solutions if found

After Phase 5 completes:
  vault_save({
    name: "GetInsuranceOffersFeature",
    summary: "...",
    spec: { objective, inputs, outputs, constraints, affects },
    design: { structure, keyDecisions },
    files: ["path/to/file1.ts", ...]
  })
```

---

## Anti-Patterns

### BAD: Skipping from exploration to implementation
```
User: "Add a get insurance offers endpoint"
AI: [immediately generates InsuranceController, InsuranceService, adapter files]
```

```
GOOD — follow the phases:
User: "Add a get insurance offers endpoint"
AI: [Phase 1] Searches Vault → asks 3 clarifying questions
AI: [Phase 2] Presents spec → waits for ✅
AI: [Phase 3] Presents design → waits for ✅
AI: [Phase 4] Generates code file by file
AI: [Phase 5] Runs tests, saves to Vault
```

### BAD: Proceeding after a spec rejection without revision
```
Developer: "The spec is missing the PE adapter constraint"
AI: [proceeds to Phase 3 anyway]
```

```
GOOD
Developer: "The spec is missing the PE adapter constraint"
AI: [revises spec, re-presents for approval]
AI: "Updated spec — does this address the PE adapter requirement? ✅ to proceed"
```

### BAD: Generating code without design approval
```
AI: [spec approved] "Great, I'll start generating the code now..."
[generates files without presenting design]
```

```
GOOD
AI: [spec approved] "Phase 3 — Design. Here is the proposed architecture:
[presents structure]
Please confirm with ✅ to proceed to implementation."
```

### BAD: Skipping vault_save after completion
```
AI: "Implementation complete, all tests pass."
[conversation ends — no knowledge persisted]
```

```
GOOD
AI: "Phase 5 complete — 94% coverage. Saving to Vault:
vault_save({ name: 'GetInsuranceOffersFeature', ... })
Knowledge persisted. Feature delivery summary: ..."
```
