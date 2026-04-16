# Korva Vision — The OS for AI-driven Engineering Teams

> **One sentence:** Korva is the cognitive operating system that sits between your engineering team and your AI agents, turning ephemeral AI sessions into accumulated institutional knowledge.

---

## The Problem We're Solving

AI coding assistants are powerful but stateless. Every session starts from zero. The senior developer who spent three days debugging a race condition in the payment processor — the AI doesn't know. The architectural decision to adopt event sourcing six months ago — the AI doesn't know. The team rule that says "never access the database from a controller" — the AI doesn't know.

The result is AI that generates technically functional but architecturally wrong code. Developers spend 15 minutes explaining context before every session. Teams develop "AI fatigue" — the overhead of babysitting AI suggestions cancels out the productivity gain.

**This is not a problem with the AI models. It's a missing infrastructure layer.**

---

## What Korva Is

Korva is the infrastructure layer that gives AI agents:

| Without Korva | With Korva |
|---|---|
| Session starts from zero | Session loads N team memories automatically |
| AI invents patterns | AI respects your established patterns |
| Decisions get forgotten | Decisions are versioned and replayable |
| Violations reach production | Violations are caught at commit time |
| Knowledge lives in heads | Knowledge lives in a searchable vault |
| New dev takes weeks to ramp | New dev AI is productive in hours |

---

## The 5-Layer Architecture

Korva is built as five integrated layers that can be adopted independently or as a full system:

```
┌─────────────────────────────────────────────────────────────┐
│                    LAYER 5 — Forge                          │
│          Structured AI Execution & Workflow                 │
│    5-phase SDD · Multi-agent validation · Audit trail       │
├─────────────────────────────────────────────────────────────┤
│                    LAYER 4 — Lore                           │
│              Living Knowledge Graph                         │
│    Scrolls · Stack rules · Auto-context injection           │
├─────────────────────────────────────────────────────────────┤
│                    LAYER 3 — Sentinel                       │
│            Runtime Guardrails Engine                        │
│    Pre-commit · Live validation · PR review · SEC rules     │
├─────────────────────────────────────────────────────────────┤
│                    LAYER 2 — Context                        │
│             Intent Understanding System                     │
│    Debug · Feature · Refactor · Spike detection             │
├─────────────────────────────────────────────────────────────┤
│                    LAYER 1 — Vault                          │
│              Persistent Decision Engine                     │
│    Memory · Decision replay · Explainability · Timeline     │
└─────────────────────────────────────────────────────────────┘
```

### Layer 1 — Vault: From Memory to Decision Engine

Today Vault stores observations. Tomorrow it becomes a decision engine:

- **Decision replay** — "Why was this decision made?" answers in seconds
- **Explainability** — every AI suggestion can be traced to a vault observation
- **Decision trees** — version-controlled architecture decisions (Git for decisions)
- **Knowledge decay** — detects observations that are stale or contradicted by newer ones
- **Organizational memory** — business context, trade-offs, postmortems, not just code

```
vault_why "we use Redis for sessions"
→ Decision #47 (Mar 2024): "Evaluated Memcached vs Redis.
  Chose Redis for pub/sub support needed by notifications.
  Owner: @sarah. Revisit when notifications team migrates."
```

### Layer 2 — Context: Intent Understanding

The system detects what the developer is trying to do:

- **Debug mode** → surfaces related incidents and past fixes
- **Feature mode** → loads relevant architecture patterns and constraints
- **Refactor mode** → shows impact analysis and dependent components
- **Spike mode** → disables guardrails, enables experimental suggestions

```
korva context → detected: "refactor" (confidence 0.94)
→ Loading: refactor patterns, impact analysis, test coverage
→ Suppressing: strict architecture rules (refactor mode)
```

### Layer 3 — Sentinel: From Commit Hook to Runtime Guardian

Sentinel evolves from pre-commit to always-on:

- **Live validation** in VS Code — red underlines on violations before you save
- **PR reviewer** — automatic review bot on every pull request
- **Runtime hooks** — validate AI suggestions before they're inserted into files
- **Compliance layer** — GDPR, SOC2, HIPAA rule sets

### Layer 4 — Lore: From Scrolls to Living Knowledge Graph

Lore becomes a dynamic graph that connects:

- Code ↔ Decisions ↔ Incidents ↔ People ↔ External knowledge
- Auto-updating: when a decision changes, all dependent scrolls update
- Cross-repo: a security rule defined in one repo applies to all
- Community: public scrolls grow via open-source contributions

### Layer 5 — Forge: From Workflow to Autonomous Execution

Forge becomes a semi-autonomous multi-agent system:

- **Simulation mode** — test architecture, performance, and costs before writing code
- **Multi-agent validation** — one agent builds, another reviews against vault
- **Autonomous onboarding** — `korva onboard` analyzes codebase and generates knowledge
- **Cognitive profiles** — per-team AI behavior configuration

---

## The Public/Private Model

Korva is designed with a clear separation of public and private concerns:

```
┌─────────────────────────────────────────────────────────┐
│  KINGDOM 1 — Public (MIT)                               │
│  github.com/AlcanDev/korva                              │
│                                                         │
│  Core engine · CLI · Vault · Sentinel · Lore engine     │
│  Community scrolls · Generic architecture rules         │
│  Zero knowledge of your team's data                     │
└─────────────────────────────────────────────────────────┘
         ↓ can reference, never merges ↑
┌─────────────────────────────────────────────────────────┐
│  KINGDOM 2 — Private Team Repo (your GitHub)            │
│  github.com/YOUR_ORG/korva-team-profile                 │
│                                                         │
│  Team-specific scrolls · Custom Sentinel rules          │
│  AI instructions · Architecture decisions               │
│  Cognitive profiles · Onboarding scripts                │
└─────────────────────────────────────────────────────────┘
         ↓ syncs to local, never to cloud ↑
┌─────────────────────────────────────────────────────────┐
│  KINGDOM 3 — Local Machine (~/.korva/)                  │
│                                                         │
│  vault.db (your observations) · admin.key (0600)        │
│  Profiles · Secrets · Runtime state                     │
│  NEVER leaves this machine by default                   │
└─────────────────────────────────────────────────────────┘
```

### Optional Cloud Sync

Teams can optionally deploy a shared vault for collaboration:

```
Local vault ──sync──▶ Self-hosted korva-vault (VPS/Railway/Fly.io)
                      │
                      ├── Shares: non-sensitive observations
                      ├── Excludes: secrets, PII, private keys
                      └── Controls: who can read/write per project
```

This is **opt-in, self-hosted, and fully auditable**. Korva never connects to our servers.

---

## Why Korva Wins

### vs. Static instruction files (.cursorrules, CLAUDE.md)
Static files are manually maintained, globally applied, and don't accumulate knowledge. Korva is dynamic, context-aware, and grows over time.

### vs. General memory tools (MemGPT, Letta, etc.)
General memory tools aren't designed for engineering teams. They lack architecture enforcement, commit integration, stack-specific knowledge, and organizational memory.

### vs. Enterprise AI tools (GitHub Copilot Enterprise, Cursor Teams)
These live in the cloud, are expensive, and control your data. Korva is local-first, free, and you own everything.

### vs. Building it yourself
Teams spend weeks building context files, prompt libraries, and custom hooks. Korva is that infrastructure out of the box, in 30 seconds, for any stack.

---

## The Community Flywheel

```
Individual developer installs Korva
  → Discovers value, saves observations
  → Contributes a Lore scroll for their stack
  → Team adopts, creates Team Profile
  → Team contributes Sentinel rules
  → Community benefits, more developers install
  → Flywheel accelerates
```

The more teams use Korva, the richer the community knowledge base. Every scroll contributed, every Sentinel rule added, every pattern documented makes the tool better for everyone.

---

## Competitive Positioning Summary

| Dimension | Korva | Static files | Cloud AI tools |
|---|---|---|---|
| Memory | Persistent, growing | Manual, static | Session-based |
| Privacy | 100% local | 100% local | Cloud-dependent |
| Enforcement | Commit-time guardrails | None | Suggestions only |
| Knowledge | Context-injected | Global | None |
| Cost | Free forever | Free | $$$  |
| Team scale | Profiles + sync | Copy-paste | Managed |
| Stack coverage | Any (community) | Manual | Vendor-defined |

---

*Korva is MIT licensed. The code is yours. The knowledge is yours. The future is collaborative.*
