# Korva for Teams — Commercial License

**Effective date:** 2025-01-01  
**Last updated:** 2026-05-15  
**Licensor:** Felipe Alcántara García, trading as Korva / AlcanDev  
**Contact:** felipe.alcantara.gar@gmail.com  
**Website:** https://korva.dev

---

## 1. License Scope

This Commercial License grants the purchaser (the "Licensee") the right to
use the Korva for Teams software ("Software") for internal business purposes
under the terms below.

The core Korva vault, CLI, sentinel, and IDE integrations are separately
available under the MIT License at no cost. This Commercial License covers
only the Teams-tier features listed in Section 3.

---

## 2. Pricing

### Launch Pricing (50 % discount — locked in for 12 months from first activation)

| Tier | Regular | Launch Price | Billing |
|------|---------|-------------|---------|
| **Korva for Teams** | $19 / user / month | **$9 / user / month** | Annual, prepaid |

Minimum seat count: **3 users**.

Launch pricing is locked in for 12 months from the date of first activation.
Renewal is at the then-current published price with 30-day advance notice.

For organizations requiring SLA guarantees or SSO/SAML integration, contact
sales for a custom arrangement.

> **Note on previous tiers:** Korva previously offered separate Business and
> Enterprise tiers. These have been consolidated into the single Teams tier.
> Existing Business and Enterprise license keys continue to work and are
> automatically mapped to the Teams tier at renewal.

---

## 3. Feature Set

### Community (MIT — free forever)

- Vault MCP server (local SQLite, 23 MCP tools)
- `vault_save`, `vault_search`, `vault_context`, `vault_timeline`,
  `vault_get`, `vault_hint`, `vault_compress`, `vault_ping`,
  `vault_bulk_save`, `vault_session_start`, `vault_session_end`,
  `vault_scroll_list`, `vault_scroll_get`
- Sentinel pre-commit hooks (21 built-in rules)
- IDE integrations (Claude Code, Cursor, Windsurf, GitHub Copilot, Codex,
  Gemini CLI, OpenCode, VS Code)
- Lore scrolls (25 curated, public)
- Team Profile via private Git repo
- Korva Hive — opt-in community cloud sync
- Korva Beacon — local web dashboard
- Forge — 5-phase Spec-Driven Development workflow
- Community support (GitHub Issues)

### Teams ($9 / user / month at launch, $19 regular)

Everything in Community, plus:

- **Skills Hub** — team skill editor in Beacon dashboard
- **`vault_skill_match`** — explicit skill resolver with scored matches
- **Smart Skill Auto-Loader** — auto-injects team skills into `vault_context`
  based on project, prompt, and file patterns
- **`vault_team_context`** — load team skills and private scrolls
- **`vault_qa_checklist`** / **`vault_qa_checkpoint`** — SDD quality gates
- **`vault_code_health`** — composite 0–100 code health score (A–F grade)
  with trend detection and breakdown by type
- **`vault_pattern_mine`** — surface emerging implicit patterns from observations
- **`vault_skill_list`** / **`vault_skill_get`** — versioned team AI capabilities
- **Private Scrolls** — team-managed knowledge base (no Git repo required,
  managed directly in Beacon)
- **Multi-profile workspaces** — frontend / backend / devops profiles with
  atomic switch (Lore + Sentinel + Skills swap together)
- **Private Hive sync** — encrypted cross-team knowledge sync
- **RBAC** — role-based access control + invite tokens
- **Audit Log** — immutable admin change trail (who saved what, when, from where)
- **Beacon analytics** — sessions, phase gates, skill usage, code health panels
- **`korva connect`** — one-command connection to the Teams portal
- Email support (48 h SLA)

For SLA guarantees (99.9% uptime), SSO/SAML, dedicated vault instances,
or SOC 2-aligned audit exports, contact sales.

---

## 4. License Grant

Subject to payment of applicable fees, Licensor grants Licensee a
non-exclusive, non-transferable, non-sublicensable license to:

a. Install and use the Software on Licensee's own infrastructure (on-premises
   or private cloud).  
b. Allow access by the number of "seats" (named users) purchased.  
c. Modify configuration, team profiles, and scrolls for internal use.

---

## 5. Restrictions

Licensee may **not**:

a. Resell, sublicense, or redistribute the Software or its Teams features
   to third parties.  
b. Reverse-engineer the licensing validation mechanism or the signed JWS
   license token.  
c. Remove or alter copyright notices or license attributions.  
d. Use the Software to build a competing product or service.  
e. Share a license key across installations beyond the purchased seat count.

---

## 6. Data and Privacy

- All vault data remains on Licensee's own infrastructure. Licensor does not
  have access to stored observations, skills, or scrolls.
- The heartbeat mechanism (Section 7) transmits only: `license_id`,
  `install_id`, and a timestamp. No source code or user content is transmitted.
- Team Profiles, skills, and scrolls are never transmitted to Licensor.
- Korva Hive (community cloud sync) is opt-in and applies a default-deny
  privacy filter before anything leaves the machine.

---

## 7. License Validation and Heartbeat

The Software validates the license offline using an embedded RS256 public key.
A background heartbeat runs every 24 hours to confirm the license is still
active. If the heartbeat cannot reach the licensing server:

- A **7-day grace period** applies before Teams features are suspended.
- After the grace period, the installation degrades to Community tier until
  connectivity is restored.
- No data is deleted during demotion; all observations remain accessible.

---

## 8. Termination

This license terminates immediately if:

a. Licensee fails to pay applicable fees within 30 days of the due date.  
b. Licensee materially breaches any term and fails to cure within 15 days of
   written notice.

Upon termination, Teams features are disabled. Community-tier features
and all stored data remain accessible indefinitely.

---

## 9. Warranty Disclaimer

THE SOFTWARE IS PROVIDED "AS IS" WITHOUT WARRANTY OF ANY KIND. LICENSOR
DISCLAIMS ALL WARRANTIES, EXPRESS OR IMPLIED, INCLUDING WARRANTIES OF
MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE.

---

## 10. Limitation of Liability

IN NO EVENT SHALL LICENSOR BE LIABLE FOR ANY INDIRECT, INCIDENTAL, SPECIAL,
OR CONSEQUENTIAL DAMAGES, OR LOSS OF PROFITS, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGES. LICENSOR'S TOTAL LIABILITY SHALL NOT EXCEED THE
FEES PAID BY LICENSEE IN THE 12 MONTHS PRECEDING THE CLAIM.

---

## 11. Governing Law

This agreement is governed by the laws of Chile. Any disputes shall be
resolved in the courts of Santiago, Chile.

---

## 12. Contact and Activation

To purchase a license, obtain a trial key, or request a custom arrangement:

- **Email:** felipe.alcantara.gar@gmail.com
- **Web:** https://korva.dev/pricing
- **CLI:** `korva license activate <your-key>`
- **Teams portal:** `korva connect https://portal.korva.dev <your-key>`

---

*Korva — AI memory infrastructure for engineering teams.*
