# Korva Roadmap

> Evolving from AI memory tool to The OS for AI-driven Engineering Teams.

Status: `v0.1.0` released · Phase 1 in progress

---

## Phase 1 — Solid Foundation ✅ → 🔨
*Target: v0.2.0 — Q2 2026*

The goal of Phase 1 is to make the four core components (Vault, Sentinel, Lore, Forge) polished, well-documented, and adopted by early teams.

### Vault — Persistent Memory (v0.1.0 ✅)
- [x] SQLite + FTS5 local storage
- [x] MCP protocol (works with Copilot, Claude, Cursor)
- [x] 10 MCP tools: `vault_save`, `vault_context`, `vault_search`, `vault_timeline`...
- [x] Privacy filter (auto-redacts secrets before storage)
- [x] Admin key with 0600 permissions and constant-time auth
- [ ] **Decision tree versioning** — track why decisions changed over time
- [ ] **Knowledge decay alerts** — flag observations older than N months
- [ ] **`vault_why` tool** — natural language explanation of past decisions
- [ ] **Timeline visualization** — web UI for browsing vault history

### Sentinel — Architecture Guardrails (v0.1.0 ✅)
- [x] Pre-commit hook integration
- [x] 10 built-in rules: HEX, NAM, SEC, TEST series
- [x] Custom rule support via YAML
- [ ] **SEC-005: SQL injection detection** — catch string interpolation in queries
- [ ] **SEC-006: Timing attack detection** — flag non-constant-time comparisons
- [ ] **DEPS-001: Dependency audit** — flag known vulnerable packages
- [ ] **VS Code extension** — live red underlines on violations (no commit needed)
- [ ] **GitHub Actions integration** — run Sentinel on every PR automatically

### Lore — Knowledge Injection (v0.1.0 ✅)
- [x] 18 curated scrolls (React/Next.js, NestJS, TypeScript, Payments/Stripe, Security, CI/CD, Docker/K8s, token-efficiency, MCP builder, frontend-design, claude-api, etc.)
- [x] Auto-load by file context
- [x] Team Profile scroll distribution
- [x] Community scroll layer (`lore/community/`)
- [ ] **`korva lore skills`** — detect project stack, install matching community skills from [skills.sh](https://skills.sh). Covers 60+ technologies: React, Next.js, NestJS, Go, Python, Flutter, Stripe, Cloudflare, and more. See [docs/COMMUNITY-SKILLS.md](docs/COMMUNITY-SKILLS.md).
- [ ] **Scroll versioning** — `scroll@2.1` for breaking changes
- [ ] **Stack auto-detection** — analyze package.json/go.mod and suggest relevant scrolls
- [ ] **Community scroll registry** — `korva lore search "react performance"`
- [ ] **5 new scroll packs**: Python FastAPI, Go microservices, Rust, Ruby on Rails, React Native

### Forge — Structured Workflow (v0.1.0 ✅)
- [x] 5-phase SDD workflow documentation
- [x] Phase prompts and Forge scroll
- [ ] **`korva forge init`** — interactive session setup with phase guidance
- [ ] **Phase validation gates** — can't proceed to Phase 3 without a spec document
- [ ] **Session recording** — replay how a feature was built phase by phase

### Infrastructure & DX
- [ ] **`korva onboard`** — analyze codebase, auto-generate initial vault observations
- [ ] **`korva doctor`** — diagnose configuration issues (vault reachable, hooks installed, profile valid, admin.key permissions)
- [ ] **`korva update`** — self-update CLI and vault binary
- [ ] **Windows installer improvements** — silent install, MSI package
- [ ] **ARM64 binaries** — Apple Silicon native, Raspberry Pi support

---

## Phase 2 — Intelligence Layer 🔮
*Target: v0.3.0 — Q3 2026*

Phase 2 adds intelligence on top of the foundation. The system starts understanding intent and providing runtime protection.

### Vault → Decision Engine
```
vault_context("auth") → loads 47 observations
vault_why "we chose JWT over sessions"
→ Decision #23 (Jan 2024): JWT for stateless scaling.
  Trade-off: No server-side revocation.
  Owner: @maria. Revisit when we hit 1M users.
```

**Tasks:**
- [ ] **Decision indexing** — separate storage layer for architecture decisions vs. observations
- [ ] **`vault_why` NLP** — natural language querying of decision history
- [ ] **Explainability** — every AI suggestion tagged with the vault observation that triggered it
- [ ] **Contradiction detection** — alert when a new observation contradicts an older one
- [ ] **Organizational memory** — business decisions, trade-offs, postmortems (not just code)
- [ ] **`vault_timeline` interactive** — visual history of all decisions for a component

### Context → Intent Detection
```
# AI detects you're debugging a crash
korva context → "debug mode" detected (0.91 confidence)
→ Auto-loading: crash history, recent incidents, related fixes
→ Suppressing: architectural enforcement (debug mode)
```

**Tasks:**
- [ ] **Intent classifier** — detect debug/feature/refactor/spike from git diff + file patterns
- [ ] **Mode-based behavior** — different context loading and rule enforcement per mode
- [ ] **Team learning** — intent model fine-tunes on team-specific commit patterns
- [ ] **`korva mode [debug|feature|refactor|spike]`** — manual override

### Sentinel → Runtime Guardian
```
# Real-time feedback in VS Code
# You type: const secret = "sk_live_..."
# Sentinel highlights it red BEFORE you save:
⚠ SEC-001: Hardcoded secret detected. Use process.env.JWT_SECRET
```

**Tasks:**
- [ ] **VS Code Language Server** — real-time SEC/ARC rule validation
- [ ] **GitHub PR bot** — `@korva-sentinel review` comment triggers automatic review
- [ ] **Runtime hooks** — intercept AI suggestions and pre-validate before insertion
- [ ] **Compliance packs** — GDPR, SOC2, HIPAA rule sets as installable packages
- [ ] **Severity tiers** — ERROR (block), WARN (flag), INFO (suggest)

### Lore → Knowledge Graph
```
# Graph shows connections between knowledge
query: "what affects our payment flow?"
→ Scrolls: stripe-webhooks, pci-dss, retry-patterns
→ Decisions: #23 (idempotency), #31 (Decimal.js)
→ Incidents: #07 (race condition Oct 2024)
→ People: @carlos (domain expert)
```

**Tasks:**
- [ ] **Graph storage** — SQLite with graph traversal queries
- [ ] **Entity extraction** — auto-extract people, systems, decisions from observations
- [ ] **Cross-repo links** — a decision in repo A can reference a pattern in repo B
- [ ] **`korva graph query`** — explore the knowledge graph from CLI
- [ ] **Conflict resolution** — when two scrolls give contradictory advice

### Infrastructure
- [ ] **Cognitive Profiles** — per-team AI behavior config (strictness, style, domain focus)
- [ ] **Optional cloud sync** — self-hosted vault sharing with encryption
- [ ] **Audit log** — complete record of every AI suggestion, acceptance, and rejection
- [ ] **`korva export`** — export vault to Markdown/JSON for backup or migration

---

## Phase 3 — Autonomous Systems 🚀
*Target: v1.0.0 — Q4 2026*

Phase 3 introduces semi-autonomous execution and full organizational intelligence.

### Forge → Autonomous Execution
```
# Simulation before implementation
korva forge simulate "add real-time notifications"
→ Architecture impact: HIGH (new WebSocket layer)
→ Performance estimate: +12ms p99 latency
→ Risk: Race condition in session management
→ Recommended: Review incident #07 first
→ Proceed? [y/N]
```

**Tasks:**
- [ ] **Simulation mode** — static analysis to predict architecture, performance, and cost impact
- [ ] **Multi-agent validation** — Agent A builds, Agent B reviews against vault and Sentinel
- [ ] **`korva forge plan`** — AI generates a Forge-structured implementation plan
- [ ] **Autonomous onboarding** — `korva onboard` generates 50+ observations from a new codebase in minutes
- [ ] **Cross-phase learning** — vault learns from completed Forge sessions

### Cross-Repo Intelligence
```
# One command, entire organization
korva org scan
→ Analyzing 12 repositories...
→ Found: 3 conflicting auth patterns across teams
→ Found: 2 deprecated library versions with CVEs
→ Recommendation: Unify auth under shared scroll
```

**Tasks:**
- [ ] **Org-level vault** — aggregate observations across repositories
- [ ] **Pattern harmonization** — detect and resolve conflicting patterns between teams
- [ ] **Dependency graph** — understand how changes in one repo affect others
- [ ] **`korva org report`** — executive summary of AI usage, violations, patterns

### Compliance & Audit Layer
```
# Full audit trail
korva audit --from 2025-01-01 --regulation gdpr
→ 847 AI interactions reviewed
→ 3 PII handling issues flagged
→ 12 data retention decisions documented
→ Audit report: korva-gdpr-audit-2025-q1.pdf
```

**Tasks:**
- [ ] **GDPR pack** — PII detection, data retention rules, deletion tracking
- [ ] **SOC2 pack** — access controls, change management, monitoring rules
- [ ] **Full AI audit trail** — every vault_save, Sentinel check, Lore injection logged
- [ ] **`korva audit`** — generate compliance reports from the audit log

### Ecosystem
- [ ] **JetBrains plugin** — IntelliJ IDEA, GoLand, Rider support
- [ ] **Neovim/Emacs** — MCP integration for terminal-first developers
- [ ] **Korva Cloud** (optional, privacy-preserving) — hosted vault for teams that want zero ops
- [ ] **Marketplace** — community scrolls, Sentinel rules, Forge templates

---

## Versioning Strategy

| Version | Phase | Focus |
|---|---|---|
| v0.1.x | 1 | Core stability, bug fixes |
| v0.2.0 | 1 | `korva doctor`, decision engine, VS Code extension, 5 new scrolls |
| v0.3.0 | 2 | Intent detection, Runtime guardian, Knowledge graph |
| v0.4.0 | 2 | Cognitive profiles, Audit log, Cloud sync |
| v1.0.0 | 3 | Autonomous Forge, Cross-repo, Compliance |

---

## How to Contribute

Every item on this roadmap is an opportunity to contribute. Check the [GitHub Issues](https://github.com/AlcanDev/korva/issues) for items tagged `help wanted` and `good first issue`.

The highest-impact contributions right now:
1. **Write a Lore scroll** for your stack (Next.js, Laravel, Rust, Django, Go...)
2. **Add a Sentinel rule** for patterns your team enforces
3. **Report bugs** with clear reproduction steps
4. **Share your Team Profile** structure (sanitized) to help others set up

See [CONTRIBUTING.md](./CONTRIBUTING.md) for how to get started.

---

*This roadmap is a living document. It evolves based on community feedback, usage patterns, and the broader AI tooling landscape. Submit a GitHub Issue to propose additions or changes.*

---

*Last updated: 2026-04-30*
