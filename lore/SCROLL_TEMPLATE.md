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
[2–4 sentences. Describe the team, project, or technology context that makes this scroll relevant. Be specific to Falabella Financiero: mention the team, the systems involved (CIGO, TRO, Apigee, Vault), and the constraints that drive the rules below. Do not explain general concepts — assume the reader knows the technology.]

---

## Rules

### 1. [Rule title — short, imperative]

[One or two sentences explaining the rule and why it exists.]

```typescript
// Concrete code example using real domain names:
// InsurancePort, InsuranceService, LifeInsuranceAdapterBase/CL/PE
// GetInsuranceOffersCommand, CommonHeadersRequestDTO, InsuranceOffer, InsuranceId
```

### 2. [Rule title]

[Explanation.]

```typescript
// Code example
```

### 3. [Rule title]

[Explanation.]

```typescript
// Code example
```

<!-- Add as many numbered rules as needed. Aim for 5–10 rules per scroll. -->
<!-- Each rule should have a code example using the real domain when applicable. -->

---

## Anti-Patterns

<!-- Each anti-pattern block: BAD code first, then GOOD code. -->
<!-- Keep examples minimal — show only the relevant lines. -->

### BAD: [Short description of the mistake]
```typescript
// BAD — explain why this is wrong in a comment
const wrong = doItWrongly();
```

```typescript
// GOOD — what to do instead
const correct = doItCorrectly();
```

### BAD: [Short description of the second mistake]
```typescript
// BAD
```

```typescript
// GOOD
```

<!-- Add 3–5 anti-pattern pairs. More is not always better — pick the most common mistakes. -->

---

<!--
INSTRUCTIONS FOR SCROLL AUTHORS
================================

1. FRONTMATTER
   - id:       kebab-case, matches the directory name (e.g., nestjs-hexagonal)
   - version:  start at 1.0.0; bump minor for new rules, major for breaking changes
   - team:     who this scroll targets (backend | frontend | devops | all)
   - stack:    comma-separated list of relevant technologies

2. TRIGGERS
   - Files:    glob patterns. Be specific enough to avoid false positives.
   - Keywords: terms an AI will encounter in conversation that suggest loading this scroll.
   - Tasks:    action-oriented phrases (e.g., "writing a Dockerfile", "creating a port interface").

3. CONTEXT
   - Mention Falabella Financiero context explicitly.
   - Name the downstream systems (CIGO, TRO, Apigee) when relevant.
   - State the team convention or constraint that explains why the rules exist.

4. RULES
   - Number them. Keep each rule focused on one decision.
   - Always include a code example with real domain names from the seguros project:
       InsurancePort, INSURANCE_PORT, InsuranceService,
       LifeInsuranceAdapterBase, LifeInsuranceAdapterCL, LifeInsuranceAdapterPE,
       GetInsuranceOffersCommand, CommonHeadersRequestDTO,
       InsuranceOffer, InsuranceId, PolicyId
   - Tables are allowed for comparing alternatives.
   - File trees are useful for structure rules.

5. ANTI-PATTERNS
   - Always provide both BAD and GOOD versions.
   - Add a comment on the BAD example explaining why it is wrong.
   - Keep each pair under 20 lines total.

6. LENGTH
   - Target: 100–200 lines of Markdown (excluding these instructions).
   - Do not pad with explanations of general concepts — focus on team-specific decisions.

7. NAMING
   - File name: SCROLL.md (always uppercase)
   - Directory: /lore/curated/<category>/SCROLL.md
   - id in frontmatter must match the directory name exactly.
-->
