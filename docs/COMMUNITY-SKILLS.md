# Korva Community Skills Guide

> Community skills deliver best practices for every library in your stack. Korva injects team knowledge and enforces architecture patterns. Together they give your AI the full picture.

---

## What are community skills?

Community skills are curated markdown files hosted on [skills.sh](https://skills.sh). Each skill covers best practices, patterns, and guidelines for a specific technology — exactly like Korva scrolls, but maintained by the open-source community.

Install a skill for Claude Code:
```bash
npx skills add owner/repo --skill skill-id -a claude-code
```

Skills live in `.claude/skills/` and are automatically loaded at session start.

---

## Why use both community skills and Korva Lore?

| | **Community Skills** | **Korva Lore** |
|---|---|---|
| **Scope** | Community knowledge | Community + team-private knowledge |
| **Detection** | Installed once at init | Always-on — fires when you open a file |
| **Delivery** | Static `.md` files in `.claude/skills/` | Dynamic MCP injection at session time |
| **Team patterns** | Not supported | Core feature — private Team Profiles |
| **Memory** | None — install-time only | Vault persists decisions across sessions |
| **Guardrails** | None | Sentinel validates at commit time |
| **Customization** | None | Full — write your own scrolls |
| **Sync** | Re-install manually | `korva sync --profile` |

**The sweet spot**: community skills provide the broad foundation (Next.js, Stripe, Playwright patterns). Korva provides the deep team layer (your architecture decisions, your patterns, your incidents, your rules).

---

## Setup — Use Both in One Project

### Step 1: Install Korva

```bash
# macOS / Linux
curl -fsSL https://korva.dev/install.sh | bash

korva init
korva setup    # auto-configures VS Code, Claude Code, Cursor
```

### Step 2: Install community skills for your stack

```bash
# Next.js + React
npx skills add vercel-labs/next-skills --skill next-best-practices -a claude-code
npx skills add vercel-labs/agent-skills --skill vercel-react-best-practices -a claude-code

# Stripe
npx skills add stripe/ai --skill stripe-best-practices -a claude-code

# Playwright (testing)
npx skills add currents-dev/playwright-best-practices-skill --skill playwright-best-practices -a claude-code

# TypeScript
npx skills add wshobson/agents --skill typescript-advanced-types -a claude-code
```

See [`lore/community/autoskills/SCROLL.md`](../lore/community/autoskills/SCROLL.md) for the full technology → skills mapping table (60+ technologies).

### Step 3: Add your team profile (optional)

```bash
korva init --profile git@github.com:YOUR-ORG/korva-team-profile.git
```

This adds your team's private scrolls, Sentinel rules, and AI instructions on top of the community skills.

### Step 4: Start working

Open any AI session. Your AI now has:
- Community skills from skills.sh (in `.claude/skills/`)
- Team architecture patterns from Korva Lore (injected via MCP)
- Project-specific memory from Korva Vault
- Commit-time guardrails from Sentinel

---

## How Skills and Scrolls Complement Each Other

**Community skills** answer: *"What does the community consider best practice for this library?"*

**Korva scrolls** answer: *"What has OUR team decided about this domain?"*

Example for a Next.js + Stripe project:

```
Community skills installed:
  .claude/skills/vercel-labs__next-skills__next-best-practices.md
  .claude/skills/stripe__ai__stripe-best-practices.md

Korva Lore injects when you open src/payments/checkout.ts:
  📜 payments-stripe   Your team's race condition fix (Redis lock, 30s TTL)
  📜 security-patterns Your team's PCI-DSS decisions
  📜 forge-sdd         Reminder to follow the 5-phase workflow

Korva Vault adds:
  ✓ Incident #07 — double-charge race condition (Oct 2024)
  ✓ Decision #23 — idempotency key format: payment:{orderId}:{amount}
  ✓ Decision #31 — Decimal.js for all money, never floats
```

The AI sees the full picture. No repeated explanations. No forgotten decisions.

---

## Installing Skills for Specific Technologies

```bash
# Frontend
npx skills add vercel-labs/next-skills --skill next-best-practices -a claude-code
npx skills add vercel-labs/next-skills --skill next-cache-components -a claude-code
npx skills add vercel-labs/agent-skills --skill vercel-react-best-practices -a claude-code
npx skills add angular/skills --skill angular-developer -a claude-code
npx skills add hyf0/vue-skills --skill vue-best-practices -a claude-code
npx skills add antfu/skills --skill nuxt -a claude-code
npx skills add ejirocodes/agent-skills --skill svelte5-best-practices -a claude-code

# Backend
npx skills add kadajett/agent-nestjs-skills --skill nestjs-best-practices -a claude-code
npx skills add stripe/ai --skill stripe-best-practices -a claude-code

# Testing
npx skills add currents-dev/playwright-best-practices-skill --skill playwright-best-practices -a claude-code
npx skills add antfu/skills --skill vitest -a claude-code

# TypeScript / Language
npx skills add wshobson/agents --skill typescript-advanced-types -a claude-code
npx skills add pproenca/dot-skills --skill zod -a claude-code

# Python
npx skills add wshobson/agents --skill python-testing-patterns -a claude-code
```

---

## `korva lore skills` — Coming in v0.2.0

The next version of Korva integrates community skill detection natively:

```bash
# Detect your stack and install matching skills as Korva community scrolls
korva lore skills

# Dry run — see what would be installed without installing
korva lore skills --dry-run

# Install skills for specific technologies
korva lore skills --tech nextjs,stripe,playwright

# Update previously installed community skills
korva lore skills --update
```

This command will:
1. Scan your project files (`package.json`, `go.mod`, `pyproject.toml`, etc.)
2. Pull matching skills from [skills.sh](https://skills.sh)
3. Store them as Korva community scrolls in `~/.korva/lore/community/`
4. Make them searchable via `vault_search "skills:nextjs"`
5. Auto-load them via the Lore engine when you open relevant files

---

## Where Skills Live

```
.claude/
  skills/                              ← community skills install here
    vercel-labs__next-skills__next-best-practices.md
    stripe__ai__stripe-best-practices.md
    currents-dev__playwright-best-practices-skill__playwright-best-practices.md
    ...
  CLAUDE.md                            ← your project's AI instructions
  settings.json                        ← Korva MCP server config

~/.korva/
  lore/
    community/                         ← korva lore skills installs here (v0.2.0)
      nextjs.md
      stripe.md
      playwright.md
    private/                           ← your team profile scrolls
```

---

## Updating Skills

```bash
# Update community skills
npx skills add owner/repo --skill skill-id -a claude-code   # re-install to update

# Update Korva team scrolls
korva sync --profile
```

---

## Troubleshooting

**Skills not appearing in Claude Code**

Restart Claude Code after installing. Skills in `.claude/skills/` are picked up on session start.

**Korva Lore not loading**

Make sure `korva-vault` is running: `korva-vault --mode=mcp`. Check that your editor's MCP configuration points to it. Run `korva doctor` for a full diagnostic.

**Skills conflicting with team scrolls**

Community skills and Korva scrolls both load at session start — there is no conflict. The AI uses both. If they give contradictory advice, team scrolls take precedence (they appear later in the context and are more specific).

---

## Contributing Skills

If you've written a Korva scroll for a technology not yet in the community registry, publish it to [skills.sh](https://skills.sh):

1. Write the scroll following the [SCROLL_TEMPLATE.md](../lore/SCROLL_TEMPLATE.md) format
2. Convert to a skills.sh-compatible format (plain markdown, no frontmatter)
3. Publish as a GitHub repo with `skills.json` metadata
4. Submit to the [skills.sh registry](https://skills.sh) to make it discoverable

Your team's knowledge contributes to the community — everyone benefits.
