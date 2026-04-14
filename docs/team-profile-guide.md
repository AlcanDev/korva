# Team Profile Guide

Korva's **Team Profile** system lets any organization add private, team-specific knowledge
to Korva without modifying the public repo. All proprietary information — internal API
patterns, architecture decisions, company conventions — stays in a private Git repository
that only your team can access.

---

## The Three Kingdoms

```
Kingdom 1: Public repo      github.com/you/korva          (MIT, world)
Kingdom 2: Private profile  github.com/org/korva-profile  (your team only)
Kingdom 3: Local machine    ~/.korva/                     (runtime, secrets)
```

- **Kingdom 1** (this repo) contains zero company-specific information.
- **Kingdom 2** is your team's private profile repo. It adds knowledge on top of Kingdom 1.
- **Kingdom 3** is the runtime combination of 1 + 2, plus local-only secrets (`admin.key`).

Data flows **one way**: Kingdom 2 → Kingdom 3. Nothing ever flows back to Kingdom 1.

---

## What goes in a Team Profile repo

```
korva-my-profile/
├── team-profile.json          ← required: profile metadata + overrides
├── scrolls/
│   └── <scroll-id>/
│       └── SCROLL.md          ← private knowledge scrolls
├── instructions/
│   ├── copilot-extensions.md  ← appended to .github/copilot-instructions.md
│   └── claude-extensions.md  ← appended to CLAUDE.md
└── sentinel/
    └── my-rules.md            ← additional Sentinel rules for your conventions
```

### `team-profile.json` structure

```json
{
  "profile": {
    "id": "my-org",
    "version": "1.0.0",
    "owner": "arch-team@my-org.com",
    "team": "My Team",
    "source_repo": "git@github.com:my-org/korva-profile.git"
  },
  "overrides": {
    "vault": {
      "sync_repo": "git@github.com:my-org/korva-vault-sync.git",
      "auto_sync": true
    },
    "sentinel": {
      "rules_path": "sentinel/my-rules.md",
      "block_on_violation": true
    },
    "lore": {
      "scroll_priority": "private_first",
      "active_scrolls": ["forge-sdd", "my-arch-scroll"]
    },
    "instructions": {
      "copilot_extensions": "instructions/copilot-extensions.md",
      "claude_extensions": "instructions/claude-extensions.md",
      "merge_strategy": "append"
    }
  },
  "access": {
    "require_ssh_key": true,
    "allowed_domains": ["my-org.com"]
  }
}
```

### Allowed overrides (whitelist)

Team profiles can **only** override these fields. Core system fields (`version`, `module`,
binary paths) are immutable from profiles — this prevents supply-chain attacks.

| Section | What you can override |
|---|---|
| `vault` | `sync_repo`, `sync_branch`, `auto_sync`, `sync_interval_minutes`, `private_patterns` |
| `sentinel` | `rules_path`, `block_on_violation`, `hooks` |
| `lore` | `scroll_priority`, `active_scrolls`, `private_scrolls_path` |
| `instructions` | `copilot_extensions`, `claude_extensions`, `merge_strategy` |

---

## Creating a Team Profile

### Step 1 — Create the private repo

Create a **private** repository in your GitHub organization.

```bash
mkdir korva-my-profile && cd korva-my-profile
git init
```

### Step 2 — Add `team-profile.json`

Copy the template above, customize it, and save as `team-profile.json`.

### Step 3 — Add private Scrolls

Create a `scrolls/` directory and add your team's knowledge:

```
scrolls/
└── my-api-patterns/
    └── SCROLL.md
```

A Scroll is a Markdown file with a YAML frontmatter header:

```markdown
---
id: my-api-patterns
version: 1.0.0
triggers:
  - API integration
  - authentication
  - downstream service
---

# My API Patterns

... your team's internal API knowledge here ...
```

### Step 4 — Add instruction extensions (optional)

`instructions/copilot-extensions.md` is appended to `.github/copilot-instructions.md`
with idempotent markers so it's safe to run multiple times:

```markdown
## Team context

### Stack
- Framework: Express + TypeScript
- Auth: OAuth2 via internal gateway
...
```

### Step 5 — Push and share access

```bash
git add .
git commit -m "feat: initial team profile"
git remote add origin git@github.com:my-org/korva-my-profile.git
git push -u origin main
```

**GitHub access levels:**
| Role | GitHub access | Has admin.key |
|---|---|---|
| Team Admin | Owner | Yes (`korva init --admin`) |
| Tech Leads | Write | No |
| Developers | Read | No |
| QA / UX | Read | No |
| External contributors | Fork only (public repo) | No |

---

## Installing a Team Profile (developer setup)

```bash
# Install Korva CLI
brew install alcandev/tap/korva   # macOS
# or: winget install korva        # Windows

# Initialize Korva + install team profile
korva init
korva init --profile git@github.com:my-org/korva-my-profile.git

# Install pre-commit hooks in your project repo
cd ~/repos/my-project
korva sentinel install
```

### What `korva init --profile` does

1. Clones the private profile repo to `~/.korva/profiles/<id>/`
2. Validates `team-profile.json` against the allowed-overrides whitelist
3. Merges overrides onto the base `korva.config.json`
4. Copies private Scrolls to `~/.korva/lore/<scroll-id>/`
5. Appends instruction extensions to `.github/copilot-instructions.md` and `CLAUDE.md`
   using idempotent `<!-- korva:team-extensions:<id>:begin/end -->` markers

### Keeping the profile up to date

```bash
# Pull latest profile changes and re-apply
korva sync --profile
```

---

## Writing Private Scrolls

Scrolls are the AI's deep knowledge base. When you write `vault_search "scroll:my-api-patterns"`,
the AI fetches and reads the full Scroll content.

### Scroll structure

```markdown
---
id: my-api-patterns
version: 1.0.0
description: Internal API integration patterns
triggers:
  - API call
  - HTTP client
  - authentication
  - downstream service
---

# My API Patterns

## Quick reference
...

## Code patterns
...

## Common errors
...
```

### What makes a good Scroll

- **Decision rationale** — not just what, but WHY. "We use circuit breakers because our
  downstream SLA is 99.9% and we experienced 3 cascading failures in Q1."
- **Code patterns with context** — runnable examples of the right pattern, not abstractions.
- **Anti-patterns** — what NOT to do and why, with the failure mode explained.
- **Operational notes** — error codes, monitoring queries, runbook links.

---

## Security guidelines

- **Never put credentials in Scrolls** — even in `lore/private/`. Use Vault paths as references:
  `secret/data/my-org/apps/{app}/{env}/config`
- **Rotate access regularly** — if a developer leaves, remove their GitHub access to the profile repo.
  Their local `~/.korva/` still has the scrolls but `korva sync` will fail → they lose updates.
- **The `admin.key` is local-only** — it never enters any Git repo, not even the private one.
  Only the person who ran `korva init --admin` has it on their machine.
- **Audit with `korva doctor`** — checks that all local paths are consistent with the active profile.
