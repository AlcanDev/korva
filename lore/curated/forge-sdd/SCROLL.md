---
id: forge-sdd
version: 2.0.0
team: all
stack: Korva Forge, Korva Vault, AI-assisted development workflow
---

# Scroll: Forge SDD Workflow

## Triggers — load when:
- Files: `SPEC.md`, `DESIGN.md`, `sdd/**`, `specs/**`
- Keywords: Forge, SDD, spec-driven, exploration, specification, design, implementation, verification, vault_search, vault_save, approval, checkpoint, phase
- Tasks: starting a new feature, writing a spec, designing a solution, implementing from a spec, reviewing an implementation against its spec

## Context
Forge enforces a structured 5-phase workflow where AI and developer collaborate — the AI proposes, the developer approves at critical checkpoints before any code is generated. This prevents the most common AI coding failure: jumping straight to implementation without understanding the problem, which produces technically functional but architecturally wrong code. Vault is always consulted before designing, to surface prior decisions and avoid reinventing patterns the team has already solved.

---

## Rules

### 1. The 5 phases — overview

| Phase | Name | AI action | Developer action |
|---|---|---|---|
| 1 | Exploration | Research codebase, ask clarifying questions | Provide context, answer questions |
| 2 | Specification | Draft spec document | **Approve or reject spec** |
| 3 | Design | Draft architecture/design | **Approve or reject design** |
| 4 | Implementation | Generate code | Review output |
| 5 | Verification | Run tests, check Sentinel | Accept or request fixes |

**Phases 2 and 3 require explicit approval before proceeding.** The AI must never advance past a checkpoint without an affirmative signal from the developer.

### 2. Phase 1 — Exploration

The AI searches the Vault and codebase before asking questions. The goal is to understand what already exists and avoid duplicating solutions.

```
AI workflow for Phase 1:
1. vault_context("project-name")   ← load active context
2. vault_search("<feature keywords>") ← check for prior art
3. vault_search("<domain entity names>") ← check for existing ports/services
4. Search codebase for related files
5. Summarize findings to developer
6. Ask focused clarifying questions (max 5)
```

The developer provides context that cannot be found automatically: business rules, deadlines, external constraints, non-obvious dependencies.

### 3. Phase 2 — Specification (requires approval)

The spec document must follow this exact format:

```markdown
## Specification: [Feature Name]

**Objective**
One sentence. What this feature must accomplish.

**Inputs**
- CreateNotificationCommand { userId: UserId, type: NotificationType, payload: Record<string, unknown> }
- Priority: low | normal | high | critical

**Outputs**
- Success: NotificationId
- Failure: NotificationError (INVALID_RECIPIENT | PROVIDER_UNAVAILABLE | RATE_LIMITED)

**Constraints**
- Delivery latency < 2s for high/critical priority
- Must support email, push, and in-app channels
- Idempotent: same (userId, type, deduplicationKey) within 5 minutes → no duplicate delivery

**Affects**
- NotificationModule (new)
- UserModule (read-only, no changes)
- New: NotificationPort, NotificationService, EmailAdapter, PushAdapter
```

The AI must present this document and wait for explicit approval (`✅ approved` or equivalent) before moving to Phase 3.

### 4. Phase 3 — Design (requires approval)

After spec approval, the AI proposes the solution architecture. This includes the directory structure, class/interface signatures, and data flow — no implementation code yet.

```
Proposed structure for Notification System:

domain/
  ports/
    notification.port.ts        (new — NotificationPort interface + NOTIFICATION_PORT token)
  commands/
    create-notification.command.ts  (new — CreateNotificationCommand value object)
  value-objects/
    notification-id.ts          (new — branded type)
    notification-type.ts        (new — union type: email | push | in-app)

application/
  notification.service.ts       (new — NotificationService, injects NOTIFICATION_PORT)

infrastructure/
  adapters/
    email.adapter.ts            (new — SendGrid implementation of NotificationPort)
    push.adapter.ts             (new — Firebase FCM implementation)
    in-app.adapter.ts           (new — WebSocket broadcast implementation)
  controllers/
    notification.controller.ts  (new — REST endpoint for creating notifications)

notification.module.ts          (new — wires NOTIFICATION_PORT → EmailAdapter by default)
```

The AI must wait for explicit approval (`✅ approved`) before Phase 4.

### 5. Phase 4 — Implementation

Code is generated only after both spec and design are approved. Files are generated in dependency order — domain first, infrastructure last.

```
AI workflow for Phase 4:
1. Generate files in dependency order (domain → application → infrastructure)
2. Each file is presented for review before the next is generated
3. Tests are generated alongside implementation files (not after)
4. No TODO comments — complete implementations only
5. Follow all applicable scrolls (nestjs-hexagonal, typescript, testing-jest, security-patterns)
```

### 6. Phase 5 — Verification

```
AI workflow for Phase 5:
1. Run: npm test (or nx affected:test)
2. Run: korva-sentinel --staged (or korva sentinel run)
3. Report coverage against team threshold (default: 90%)
4. If tests fail: diagnose, fix, re-run (never skip)
5. If Sentinel flags violations: fix before proceeding
6. vault_save({ feature, spec, design, files }) — persist knowledge
7. Present completion summary to developer
```

The Vault save at the end is mandatory — it is what makes Forge compound over time. The next developer to work on this domain will find the spec, design decisions, and key constraints already in the vault.

### 7. Vault integration

```
Before Phase 2 starts:
  vault_context("project-name")        ← active context
  vault_search("notification system")  ← prior art
  vault_search("deduplication")        ← relevant patterns
  → Reuse existing solutions if found

After Phase 5 completes:
  vault_save({
    type: "forge-session",
    title: "Notification System — v1",
    summary: "Added multi-channel notifications with idempotency...",
    spec: { objective, inputs, outputs, constraints },
    design: { structure, keyDecisions: ["EmailAdapter for initial release", "..."] },
    files: [
      "src/notification/domain/ports/notification.port.ts",
      "src/notification/application/notification.service.ts",
      ...
    ]
  })
```

---

## Anti-Patterns

### BAD: Skipping from exploration to implementation

```
User: "Add a notifications system"
AI:  [immediately generates NotificationController, NotificationService, EmailAdapter...]
```

```
✅ GOOD — follow the phases:
User: "Add a notifications system"
AI:  [Phase 1] vault_context → vault_search("notifications") → asks 3 clarifying questions
AI:  [Phase 2] Presents spec → waits for ✅
AI:  [Phase 3] Presents directory structure and interfaces → waits for ✅
AI:  [Phase 4] Generates code file by file, tests alongside
AI:  [Phase 5] All tests pass, Sentinel clean, knowledge saved to vault
```

### BAD: Proceeding past a rejected checkpoint

```
Developer: "The spec is missing the rate limiting constraint"
AI:        [proceeds to Phase 3 anyway]
```

```
✅ GOOD
Developer: "The spec is missing the rate limiting constraint"
AI:        [revises spec, adds constraint: "Max 100 notifications per user per minute"]
AI:        "Updated spec — does this address the rate limiting requirement? ✅ to proceed"
```

### BAD: Generating code without design approval

```
AI: [spec ✅] "Great, I'll start generating the code now..."
    [generates NotificationController, NotificationService without showing design]
```

```
✅ GOOD
AI: [spec ✅] "Phase 3 — Design. Here is the proposed structure:
    [presents directory tree and interface signatures]
    Please confirm with ✅ to proceed to implementation."
```

### BAD: Skipping vault_save after completion

```
AI: "All tests pass, implementation complete."
    [conversation ends — no knowledge persisted for future developers]
```

```
✅ GOOD
AI: "Phase 5 complete — 92% coverage, Sentinel clean. Saving to vault:
    vault_save({ title: 'Notification System v1', spec, design, files })
    
    Knowledge persisted. Next developer working on notifications will have:
    → The original spec and constraints
    → The design decisions (why EmailAdapter first, why idempotency key format)
    → The file list for navigation"
```
