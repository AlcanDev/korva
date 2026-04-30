# Skills / Lore

> How knowledge scrolls work and how to author your own.

*Last updated: 2026-04-30*

---

## What is a scroll?

A **scroll** is a Markdown file with YAML frontmatter that captures a single coherent piece of architectural knowledge — a pattern, a checklist, an anti-pattern, an opinion. The vault's skill matcher loads scrolls into your AI session **only when their triggers match the current task**.

Result: your assistant gets exactly the context it needs without drowning in 100KB of generic guidance.

---

## Where they live

| Path | Visibility |
|------|-----------|
| `lore/curated/` | Public, MIT-licensed, ships in every Korva binary |
| `lore/private/` | Per-team, gitignored, lives only on your machine (Teams+) |

Both trees follow the same format:

```
lore/curated/<scroll-id>/
└── SCROLL.md         # frontmatter + body
```

---

## Frontmatter — required keys

```yaml
---
id: my-skill              # kebab-case, unique within the lore tree
version: 1.0.0            # semver — bump on behaviour-impacting edits
team: backend             # all | backend | frontend | devops | security | qa
stack: NestJS, Postgres   # comma-separated technologies
last_updated: 2026-04-30  # ISO date — auditable freshness
---
```

Optional keys:
- `tags: [api, rest, openapi]`
- `requires: [other-scroll-id]`
- `deprecated: true`

---

## Triggers

Each scroll declares **three axes** of triggers. The skill matcher loads the scroll when ANY axis matches:

```markdown
## Triggers — load when:
- Files: `*.controller.ts`, `*.service.ts`, `apps/**/main.ts`
- Keywords: NestJS, Fastify, decorator, dependency injection
- Tasks: creating a controller, refactoring a service, adding an endpoint
```

| Axis | Source |
|------|--------|
| Files | the editor's currently-open files |
| Keywords | words in the user's last message + recent code |
| Tasks | imperative phrases describing intent |

Be specific — over-broad triggers force the model to skim. Five to ten entries per axis is usually right.

---

## Structure

Every scroll has the same four sections after the frontmatter:

```markdown
# Scroll: <topic>

## Triggers — load when:
…

## Context
2-4 sentences explaining the problem domain. No filler.

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

This shape is enforced by `korva lore validate` — scrolls missing sections are rejected.

---

## Token budget

Every scroll has a budget. Run it through a tokenizer and target:

| Tier | Budget | When loaded |
|------|--------|-------------|
| Hot path | < 1500 tokens | Every session (rare; only the "core" scrolls) |
| Common | < 4000 tokens | When a trigger matches |
| Reference | < 8000 tokens | Explicit user request |

Exceed the budget → split into multiple scrolls with cross-links.

---

## Authoring workflow

### 1. Scaffold

```bash
korva lore new my-skill
# creates lore/private/my-skill/SCROLL.md with the standard skeleton
```

### 2. Write triggers first

The most common authoring mistake is writing the body before the triggers. Triggers force you to commit to *when* the scroll is useful. If you can't write a tight trigger list, the scroll is too broad — split it.

### 3. Pair every Rule with an Anti-Pattern

The model learns the contrast better than rules alone. For every "do X" rule, write "don't do Y" with a code example.

### 4. Validate

```bash
korva lore validate my-skill
# - frontmatter complete? ✓
# - triggers present?      ✓
# - rules + anti-patterns? ✓
# - token count under budget? ✓
```

### 5. Test it loads

```bash
korva skills match "<task description that should match>"
# returns ranked list — your scroll should be in the top 3
```

### 6. Promote to curated (PR)

If the scroll is generic enough to help every Korva user, open a PR adding it to `lore/curated/`. Otherwise leave it in `lore/private/`.

---

## Auto-loading vs explicit loading

Two modes:

| Mode | Behaviour |
|------|-----------|
| Auto-load (default) | Skill matcher injects the scroll when triggers match |
| Explicit | The user types `/load my-skill` or `vault_lore_get` is called directly |

Switch in the scroll's frontmatter:

```yaml
auto_load: false   # require explicit load
```

Useful for scrolls that contain detailed reference material that's only occasionally needed.

---

## Cross-references

Inside a scroll body, link to other scrolls with the standard Markdown link syntax:

```markdown
See [`error-handling`](../error-handling/SCROLL.md) for the error mapping rules.
```

The vault rewrites these links into proper IDs at injection time, so the AI sees `[error-handling](skill://error-handling)` and can call `lore_get` to load the target.

---

## Curated scrolls — what ships in the binary

| ID | Team | Topic |
|----|------|-------|
| `nestjs-hexagonal` | backend | Hexagonal architecture with NestJS + Fastify |
| `nestjs-bff` | backend | BFF patterns — stateless, typed HTTP, Vault secrets |
| `typescript` | all | Strict TypeScript — branded types, Zod, Result<T,E> |
| `testing-jest` | all | Co-located specs, port mocking, coverage thresholds |
| `nx-monorepo` | all | Nx — affected, libs, module boundaries |
| `gitlab-ci` | devops | GitLab CI pipelines, Docker, HashiCorp Vault |
| `docker-k8s` | devops | Production Dockerfiles, Helm, Kubernetes |
| `angular-wc` | frontend | Angular 20 Elements + host bridge |
| `react-nextjs` | frontend | React 19 + Next.js patterns |
| `frontend-design` | frontend | Design system, tokens, utility CSS |
| `playwright-e2e` | qa | E2E testing strategy |
| `forge-sdd` | all | 5-phase Spec-Driven Development |
| `mcp-builder` | all | Building MCP servers (Python, TS, Go) |
| `claude-api` | all | Anthropic API best practices |
| `api-design` | backend | REST / gRPC API design conventions |
| `payments-stripe` | backend | Stripe integration — webhooks, idempotency |
| `security-patterns` | security | OWASP-aligned patterns |
| `token-efficiency` | all | Prompt engineering for context windows |
| `skill-authoring` | all | **Meta** — how to write a Korva scroll |
| `release-engineering` | devops | Conventional Commits, semver, release-please |
| `sqlite-concurrency` | backend | SQLite under load — write queue, WAL |
| `observability` | backend | Structured logs, RED/USE metrics, OTel traces |
| `plugin-architecture` | backend | Manifests, capabilities, host versioning |
| `error-handling` | backend | Sentinel errors, wrapping, Result, panic recovery |
| `cloud-sync` | backend | Content-addressed chunks, outbox, idempotent uploads |

→ Browse the source in [`lore/curated/`](../lore/curated/).

---

## Per-team private scrolls

Teams license unlocks `lore/private/` — gitignored, never synced to GitHub.

```bash
# Initialise a team profile
korva init --profile my-team

# Add a private scroll
mkdir -p lore/private/auth-jwks-rotation
$EDITOR lore/private/auth-jwks-rotation/SCROLL.md

# Validate + load
korva lore validate auth-jwks-rotation
korva lore add auth-jwks-rotation
```

Private scrolls follow the same format as curated ones — only the directory differs.

---

## Smart Skill Loader (Teams+)

Vanilla matcher: substring match on triggers → load.
Smart matcher: TF-IDF + recent-task weighting → ranked load.

Enable with a Teams license:

```bash
korva license activate <teams-key>
korva skills index
```

Inspect:

```bash
korva skills match "rotate JWT keys with JWKS"
# top 3:
#   1. error-handling          score 0.74
#   2. security-patterns       score 0.68
#   3. auth-jwks-rotation      score 0.62 (private)
```

The matcher is deterministic and runs locally — no cloud round-trip.
