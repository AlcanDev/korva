---
id: scroll-id-kebab-case
version: 1.0.0
team: backend | frontend | devops | all
stack: [Technology1, Technology2, Framework]
---

# Scroll: [Human-Readable Title]

## Triggers — load when:
- Files: `*.ts`, `*.config.ts`, `specific-file.ts`  — glob patterns that activate this scroll
- Keywords: term1, term2, term3  — concepts or identifiers that indicate this scroll is relevant
- Tasks: task type 1, task type 2  — kinds of work this scroll supports

## Context
[2–4 sentences describing the technology context and the team conventions that make this scroll necessary. Explain *why* these rules exist — the engineering principle or the production incident that motivated them. Don't explain general concepts — assume the reader knows the technology. State what goes wrong without these rules.]

---

## Rules

### 1. [Rule title — short, imperative]

[One or two sentences explaining the rule and why it exists.]

```typescript
// ✅ Concrete code example with real-world domain names
// Use realistic entity names: OrderService, PaymentPort, UserRepository
// Show the pattern you want — not a toy example
```

### 2. [Rule title]

[Explanation.]

```typescript
// ✅ Code example
```

### 3. [Rule title]

[Explanation.]

```typescript
// ✅ Code example
```

<!-- Add as many numbered rules as needed. Aim for 5–10 rules per scroll. -->
<!-- Each rule must have a code example. Tables and file trees are encouraged. -->

---

## Anti-Patterns

<!-- Each anti-pattern block: BAD code first (clearly marked ❌), then GOOD code (✅). -->
<!-- Keep examples minimal — show only the lines that illustrate the mistake. -->

### BAD: [Short description of the mistake]
```typescript
// ❌ BAD — explain why this is wrong in a comment
const wrong = doItWrongly();
```

```typescript
// ✅ GOOD — what to do instead
const correct = doItCorrectly();
```

### BAD: [Short description of the second mistake]
```typescript
// ❌ BAD
```

```typescript
// ✅ GOOD
```

<!-- Add 3–5 anti-pattern pairs. Pick the most common, high-impact mistakes. -->

---

<!--
INSTRUCTIONS FOR SCROLL AUTHORS
================================

1. FRONTMATTER
   - id:       kebab-case, must match the directory name exactly (e.g., payments-stripe)
   - version:  start at 1.0.0; bump minor for new rules, major for breaking changes
   - team:     who this scroll targets (backend | frontend | devops | all)
   - stack:    comma-separated list of relevant technologies

2. TRIGGERS
   - Files:    glob patterns — specific enough to avoid false positives.
               Good: `checkout.ts`, `*.payment.ts`, `stripe.client.ts`
               Bad:  `*.ts` (too broad)
   - Keywords: terms an AI will encounter in conversation that suggest this scroll is relevant.
               Think about what developers say when they're working in this domain.
   - Tasks:    action-oriented phrases that describe when the AI should proactively load this scroll.

3. CONTEXT
   - Describe the domain or technical area: payments, auth, infrastructure, etc.
   - Name the real risks: what goes wrong in production without these rules?
   - Reference real incidents, CVEs, or failure modes if possible — it makes the rules memorable.
   - DO NOT write generic tutorial content — assume the reader knows the technology.

4. RULES
   - Number them. Keep each rule focused on one decision.
   - Always include a working code example with realistic names:
       PaymentService, OrderRepository, CheckoutController
       AuthGuard, UserToken, SessionStore
       DeploymentConfig, HealthCheck, RetryPolicy
   - Tables are useful for comparing alternatives (e.g., caching strategies).
   - File trees are useful for structure rules (e.g., domain/application/infrastructure layout).

5. ANTI-PATTERNS
   - Always provide both ❌ BAD and ✅ GOOD versions.
   - Add a comment on the BAD example explaining exactly why it is wrong.
   - Keep each pair under 20 lines total — brevity is clarity.

6. LENGTH
   - Target: 100–300 lines of Markdown (excluding these instructions).
   - Prefer depth over breadth: 5 well-explained rules beat 15 superficial ones.
   - Don't pad with explanations of general concepts — focus on team-specific decisions.

7. NAMING
   - File name: SCROLL.md (always uppercase)
   - Directory: /lore/curated/<scroll-id>/SCROLL.md
   - The `id` in frontmatter must match the directory name exactly.

8. CONTRIBUTING A SCROLL
   - Fork the korva repo
   - Create /lore/curated/<your-scroll-id>/SCROLL.md using this template
   - Test it: open a file that should trigger it and verify the AI loads relevant context
   - Submit a PR — see CONTRIBUTING.md for the review checklist
-->
