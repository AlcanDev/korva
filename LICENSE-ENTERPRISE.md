# Korva for Teams & Business — Commercial License

**Effective date:** 2025-01-01  
**Licensor:** Felipe Alcántara García, trading as Korva / AlcanDev  
**Contact:** felipe.alcantara.gar@gmail.com  
**Website:** https://korva.dev

---

## 1. License Scope

This Commercial License grants the purchaser (the "Licensee") the right to
use the Korva for Teams or Korva for Business software ("Software") for
internal business purposes under the terms below.

The core Korva vault, CLI, sentinel, and IDE integrations are separately
available under the MIT License at no cost. This Commercial License covers
only the enterprise features listed in Section 3.

---

## 2. Tiers and Pricing

### Launch Pricing (60 % discount — limited time)

| Tier | Regular | Launch Price | Billing |
|------|---------|-------------|---------|
| **Korva for Teams** | $29 / user / month | **$12 / user / month** | Annual, prepaid |
| **Korva for Business** | $79 / user / month | **$32 / user / month** | Annual, prepaid |
| **Korva Enterprise** | Custom | Custom | Custom contract |

Minimum seat count: **3 users** for Teams; **10 users** for Business.

Launch pricing is locked in for 12 months from the date of first activation.
Renewal is at the then-current published price with 30-day advance notice.

---

## 3. Feature Set by Tier

### Community (MIT — free forever)

- Vault MCP server (local SQLite, 23 MCP tools core)
- `vault_save`, `vault_search`, `vault_context` (base), `vault_timeline`,
  `vault_get`, `vault_hint`, `vault_compress`, `vault_summary`
- Sentinel pre-commit hooks
- IDE integrations (Claude Code, Cursor, Windsurf, Copilot, Codex, Gemini,
  OpenCode, VS Code)
- Lore scrolls (public)
- Community support (GitHub Issues)

### Teams ($12 / user / month at launch)

Everything in Community, plus:

- **Skills Hub** — team skill editor in Beacon dashboard (`vault_skill_match`)
- **Smart Skill Auto-Loader** — auto-injects team skills into `vault_context`
  based on project + prompt + file patterns (no manual `/skill` commands)
- **Team Profiles** — custom agent behavior per team
- **Private Scrolls** — team-managed knowledge base (`vault_export_lore`)
- **Audit Log** — full admin change trail
- **Member Invites** — email-based team onboarding
- Seat enforcement + license heartbeat
- Email support (48 h SLA)

### Business ($32 / user / month at launch)

Everything in Teams, plus:

- **`vault_code_health`** — composite project quality score (A-F grade),
  trend detection, breakdown by type
- **`vault_pattern_mine`** — surface emerging implicit patterns from recent
  observations across the team
- **Multiple Team Profiles** — run multiple active behavior profiles
  simultaneously per project
- **Private Cloud Sync** — encrypted team sync (Hive private mode)
- **Forge SDD Workflow** — 5-phase Software Design Document tooling
- **Beacon full dashboard** — all analytics, code health panel, pattern panel
- Priority email support (24 h SLA)

### Enterprise (custom pricing)

Everything in Business, plus:

- SSO / SAML integration
- Custom Team Profiles with private config (never touches public repo)
- Dedicated vault instance (on-premises or private cloud)
- Custom data retention policies
- SOC 2-aligned audit exports
- Quarterly architecture review
- Dedicated Slack channel
- Custom SLA (99.9 % uptime)

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

a. Resell, sublicense, or redistribute the Software or its enterprise features
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

---

## 7. License Validation and Heartbeat

The Software validates the license offline using an embedded RS256 public key.
A background heartbeat runs every 24 hours to confirm the license is still
active. If the heartbeat cannot reach the licensing server:

- A **7-day grace period** applies before enterprise features are suspended.
- After the grace period, the installation degrades to Community tier until
  connectivity is restored.
- No data is deleted during demotion; all observations remain accessible.

---

## 8. Termination

This license terminates immediately if:

a. Licensee fails to pay applicable fees within 30 days of the due date.  
b. Licensee materially breaches any term and fails to cure within 15 days of
   written notice.

Upon termination, enterprise features are disabled. Community-tier features
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

To purchase a license, obtain a trial key, or request a custom Enterprise
quote, contact:

- **Email:** felipe.alcantara.gar@gmail.com
- **Web:** https://korva.dev/pricing
- **CLI:** `korva license activate <your-key>`

---

*Korva — AI memory infrastructure for engineering teams.*
