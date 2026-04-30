---
id: skill-authoring
version: 1.0.0
team: all
stack: Markdown, YAML, Korva Skills
last_updated: 2026-04-30
---

# Scroll: Skill Authoring — How to Write a Korva Skill

## Triggers — load when:
- Files: `lore/curated/**/SCROLL.md`, `lore/private/**/SCROLL.md`, `*.skill.md`
- Keywords: skill, scroll, frontmatter, triggers, lore, knowledge, prompt engineering
- Tasks: creating a new skill, refining an existing scroll, defining triggers, writing AI-readable knowledge

## Context
Skills are the unit of curated knowledge that Korva injects into AI sessions. A well-authored skill has three properties: it loads only when needed (precise triggers), it answers a single coherent question (single responsibility), and it is short enough that the LLM reads every word (token-efficient). Bad skills drown the model in noise; good skills make the difference between a hallucinated answer and a correct one.

---

## Rules

### 1. YAML frontmatter — required keys

Every skill MUST start with frontmatter:

```yaml
---
id: my-skill              # kebab-case, unique within the lore tree
version: 1.0.0            # semver — bump when behaviour-impacting content changes
team: backend             # all | backend | frontend | devops | security | qa
stack: NestJS, Postgres   # comma-separated technologies the skill applies to
last_updated: 2026-04-30  # ISO date — auditable freshness
---
```

Optional keys:
- `tags: [api, rest, openapi]` — for search/filter UIs
- `requires: [other-skill-id]` — load order hints
- `deprecated: true` — flag for removal in next minor

### 2. Triggers section — three axes

Skills auto-load when **any** axis matches the current task. Be specific; over-broad triggers force the model to skim.

```markdown
## Triggers — load when:
- Files: `*.controller.ts`, `*.service.ts`, `apps/**/main.ts`
- Keywords: NestJS, Fastify, decorator, dependency injection, module
- Tasks: creating a controller, refactoring a service, adding a new endpoint
```

Rules of thumb:
- **Files**: glob patterns the host editor exposes. Three to seven globs is usually right.
- **Keywords**: words that appear in the user's message or a code snippet. Five to twelve.
- **Tasks**: imperative phrases describing what the user is trying to accomplish.

### 3. Single-responsibility rule

A skill answers ONE question. If you find yourself writing three H2 sections that each could stand alone, split them into three skills.

```markdown
# GOOD
Scroll: Stripe — Webhook idempotency
  → 1 H2: "Make webhooks idempotent"

# BAD
Scroll: Stripe
  → H2: Webhooks
  → H2: Subscriptions
  → H2: Refunds
  → H2: Disputes
```

Multi-topic scrolls force the model to read 8000 tokens to find the 200 it needs.

### 4. Structure — Context, Rules, Anti-Patterns

```markdown
## Context
2-4 sentences explaining the problem domain and why the rules below matter.
Mention the most common failure modes. No filler.

## Rules
### 1. Rule with a verb in the title
Code example showing the rule in action.

### 2. Next rule
…

## Anti-Patterns
### BAD: thing the user might do
```code
broken example
```

```code
fixed example
```
```

### 5. Token efficiency — measure before publishing

Every scroll has a token budget. Run the scroll through a tokenizer and target:
- **Hot path** (loaded every session): under 1500 tokens
- **Common** (loaded by trigger): under 4000 tokens
- **Reference** (loaded by explicit ask): under 8000 tokens

When you exceed the budget, split into multiple skills with cross-links rather than truncating mid-explanation.

### 6. Examples must be runnable

Code in a skill is read as ground truth by the LLM. Either:
- Paste a snippet that compiles/runs as-is, OR
- Mark it with a `// pseudo-code` comment

Half-broken examples teach broken patterns. Fully-working ones get cargo-culted in the right way.

### 7. Anti-patterns belong in the same file

Always pair good patterns with their bad counterparts. The model learns the contrast more reliably than rules alone.

```markdown
### BAD: hardcoded secret
const apiKey = "sk-12345"

### GOOD: from environment
const apiKey = process.env.STRIPE_SECRET_KEY
```

---

## Anti-Patterns

### BAD: skill with no triggers
```yaml
---
id: random-tips
---
# A bunch of stuff
```
The model has no signal to load this. It ends up either always-loaded (wasting tokens) or never-loaded (wasted skill).

### BAD: skill that quotes the framework docs
```markdown
## Rules
### 1. NestJS uses decorators
@Controller()
export class FooController { ... }
```
The model already knows NestJS exists. A skill should add what's specific to your team — naming conventions, project structure, opinions.

### BAD: skill larger than the codebase it describes
A 6000-token skill explaining a 200-line service. Compress aggressively; if the example IS the service, link to it instead.

### BAD: stale `last_updated`
A skill claiming to describe how the team writes code in 2024 when the team rewrote everything in 2026. Bump `last_updated` whenever you re-validate the content; remove the skill when no longer accurate.

---

## Reference — minimum-viable skill

```markdown
---
id: jwt-rotation
version: 1.0.0
team: backend
stack: Node.js, JWT, Redis
last_updated: 2026-04-30
---

# Scroll: JWT — Rotating Signing Keys

## Triggers — load when:
- Files: `auth/**/*.ts`, `middleware/jwt.ts`
- Keywords: JWT, signing key, kid, jwks, rotate
- Tasks: rotating a JWT signing key, implementing key rollover

## Context
Long-lived JWT signing keys are a security liability — a leaked key invalidates every active token until rotation. Use `kid` header + JWKS endpoint so verifiers can pick the right key during rollover.

## Rules

### 1. Embed `kid` in every token header
…

## Anti-Patterns

### BAD: hard-coded single key
…
```
