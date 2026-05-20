# Quickstart — Korva Cloud (team memory)

A new developer goes from "never used Korva" to "writing observations to a
team-shared vault from their editor" in under five minutes. Cloud
deployment is at `app.korva.dev` / `api.korva.dev` / `mcp.korva.dev`;
self-hosters substitute their own three hosts.

> **Operators** — read [`DEPLOY_COOLIFY.md`](DEPLOY_COOLIFY.md) first.
> This guide assumes the cloud is already up and you can already issue
> admin commands against `api.korva.dev`.

> **Prerequisite — Teams license on the cloud.** Team creation, member
> management, and invites are Teams-tier features. The operator must
> activate a Korva for Teams license on the cloud vault before step 3
> works. Without it, `POST /admin/teams` returns
> `{"error":"feature 'admin_skills' requires Korva for Teams license"}`.
> See [Appendix B](#appendix-b--activate-the-teams-license) for the
> activation flow.

---

## 1. Install the CLI

```bash
# macOS / Linux — auto-detects OS and arch, installs into /usr/local/bin
curl -fsSL https://korva.dev/install | bash

# or with Homebrew on macOS
brew install alcandev/tap/korva

# or with the Go toolchain
go install github.com/alcandev/korva/cli/cmd/korva@latest
```

Verify:

```bash
korva --version
# → korva v1.36.5+ (any 1.36 or newer)
```

---

## 2. Point the CLI at the cloud

```bash
korva config set vault.endpoint https://api.korva.dev
```

The CLI now talks to the cloud for every command. Equivalent: export
`KORVA_VAULT_ENDPOINT=https://api.korva.dev` in your shell rc.

Verify:

```bash
korva auth status
# → ✗ no active session — run 'korva auth redeem <invite-token>'
```

The "no active session" error is expected on a fresh install — proves the
CLI is reaching the cloud and that the cloud is enforcing auth.

---

## 3. Get an invite from your team admin

Each team has one admin who mints invite tokens. If you ARE the admin
bootstrapping a new team, skip to [Appendix A](#appendix-a--bootstrap-the-first-team).
If you're joining an existing team, ask the admin to run:

```bash
korva teams invite you@yourteam.com --team <team-id>
```

They paste you the token (a long hex string). It's good for 7 days and
can only be redeemed once.

---

## 4. Redeem the invite

```bash
korva auth redeem <invite-token>
# → ✓ Welcome, you@yourteam.com (team: yourteam)
#     Session token saved to ~/.korva/session.token
#     Expires in 30 days
```

Status now shows your identity:

```bash
korva auth status
# → ✓ you@yourteam.com  ·  team: yourteam  ·  role: member
#     Expires: 2026-06-19 (30 days)
```

---

## 5. Configure your editor for the remote MCP

Get the bearer your editor will send on every MCP call:

```bash
korva auth token
# → eyJhbGc...                       ← long opaque string; copy it
```

### Claude Code

Add to `~/.claude/mcp.json` (create if missing):

```jsonc
{
  "mcpServers": {
    "korva": {
      "url": "https://mcp.korva.dev/mcp",
      "headers": { "Authorization": "Bearer YOUR_SESSION_TOKEN" }
    }
  }
}
```

Restart Claude Code; the vault tools (`vault_save`, `vault_search`,
`vault_context` …) appear in the tool palette.

### Cursor

`~/.cursor/mcp.json` — same shape.

### Editors that only speak stdio

If your editor doesn't speak remote MCP yet (Aider, older Continue
builds), you can still target the cloud by spawning `korva-vault` locally
in proxy mode:

```jsonc
{
  "mcpServers": {
    "korva": {
      "command": "korva-vault",
      "args": ["--mode", "mcp"],
      "env": {
        "KORVA_VAULT_ENDPOINT": "https://api.korva.dev",
        "KORVA_SESSION_TOKEN": "YOUR_SESSION_TOKEN"
      }
    }
  }
}
```

The local `korva-vault` becomes a stdio → HTTPS transcoder; every tool
call lands at the cloud vault. Same team-memory benefit, works in any
MCP-capable editor.

---

## 6. Round-trip from the terminal (sanity check)

Skip your editor — prove the wire works first:

```bash
TOKEN=$(korva auth token)

# Save an observation
curl -sS -X POST https://mcp.korva.dev/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0", "id": 1, "method": "tools/call",
    "params": {
      "name": "vault_save",
      "arguments": {
        "project": "smoke",
        "type": "context",
        "title": "First write to cloud vault",
        "content": "If you can read this in the next call, Korva Cloud works."
      }
    }
  }' | jq .

# Read it back
curl -sS -X POST https://mcp.korva.dev/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0", "id": 2, "method": "tools/call",
    "params": { "name": "vault_search",
                "arguments": { "query": "First write", "project": "smoke" } }
  }' | jq '.result.content[0].text | fromjson | .results[0].title'
# → "First write to cloud vault"
```

If you see your title come back, the entire chain works — DNS → Cloudflare
→ Traefik → korva-vault → SQLite → Hive (if enabled) → response.

---

## 7. View it in Beacon

Open <https://app.korva.dev/> and log in with your team session. The
observation from step 6 appears in the Vault Explorer.

---

## Troubleshooting

### `korva auth redeem` → "401 unauthorized"

The token is single-use. If you already redeemed it, ask the admin to
mint a fresh one.

### `korva auth token` works but editor still 401s

Check the bearer in the editor config matches `korva auth token` output
character-for-character (no leading/trailing whitespace, no quotes). The
session token rotates when you re-redeem; old tokens are revoked
server-side.

### MCP endpoint returns `-32001 unauthorized` for valid token

Three things to check:

1. `korva auth status` — if expired, redeem again (or `korva auth login`).
2. The cloud's `member_sessions` table — admin can run
   `sqlite3 /data/vault.db 'SELECT email, expires_at FROM member_sessions ORDER BY id DESC LIMIT 5'`
   inside the container.
3. The cloud has `KORVA_MCP_ALLOW_ANONYMOUS=false` (correct) — anonymous
   requests get -32001 even when otherwise valid; verify the editor IS
   sending `Authorization: Bearer …`.

### `korva auth login --email …` says "email delivery not configured"

Either `KORVA_EMAIL_API_KEY` is unset on the cloud (operator needs to
set it), or your email isn't on the team. Fall back to the invite-token
flow from step 3.

---

## Appendix A — Bootstrap the first team

If you operate the cloud and need to create the very first team + admin,
you do it once via direct admin API calls (no human bootstrap path
exists in the CLI — by design, to prevent accidental admin creation
from a leaked invite token).

You'll need:
- The cloud's `KORVA_ADMIN_KEY` (set in Coolify env vars by the operator)
- A team name and your email

```bash
ADMIN_KEY='<the value from KORVA_ADMIN_KEY>'
BASE=https://api.korva.dev

# 1. Create the team
TEAM=$(curl -sS -X POST "$BASE/admin/teams" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme","owner":"you@acme.com"}' | jq -r .id)
echo "Team ID: $TEAM"

# 2. Add yourself as admin
curl -sS -X POST "$BASE/admin/teams/$TEAM/members" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@acme.com","role":"admin"}' | jq .

# 3. Mint your invite
INVITE=$(curl -sS -X POST "$BASE/admin/teams/$TEAM/invites" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@acme.com"}' | jq -r .token)
echo "Invite token: $INVITE"
```

Now jump back to step 4 with the printed token. Once you're in, future
members go through the normal `korva teams invite` flow.

> ⚠ Keep `KORVA_ADMIN_KEY` secret — it bypasses all team auth. Treat it
> like the root password it effectively is.

---

## Appendix B — Activate the Teams license

Korva is **OSS Community by default**: single-user, fully local, free
forever. The cloud features that make team memory work (team creation,
member management, invite minting, OIDC, etc.) live behind a **Korva
for Teams license** by design — that's the line between the OSS product
and the paid tier.

To activate the cloud:

```bash
# 1. Operator runs the licensing server (forge/licensing-server) and
#    generates an RSA-4096 keypair via `make keygen`.
# 2. Operator issues a license for this install:
curl -sS -X POST https://licensing.korva.dev/v1/issue \
  -H "Authorization: Bearer $KORVA_LICENSING_ADMIN_SECRET" \
  -H 'Content-Type: application/json' \
  -d '{"install_id":"<your-install-id>","tier":"teams","features":["admin_skills","audit_log",...]}'
# → returns a JWS key

# 3. Activate on the cloud vault (one-shot, from inside the container or
#    from your machine pointing at the cloud):
korva license activate <key>
# → ✓ Korva for Teams activated, features: admin_skills, audit_log, ...
#     Expires: 2027-05-20
```

Once activated, restart the cloud vault (Coolify "Redeploy"). The
`POST /admin/teams` endpoint stops 402-ing and Appendix A's bootstrap
flow proceeds.

> The licensing-server in this repo is currently **incomplete** — the
> `keygen/main.go` and key-issue handler haven't been ported yet. See
> tracker for "Complete licensing-server" task. Until that's done, the
> cloud runs Community-tier and the team flow above is documented but
> not executable end-to-end. The transport + auth + MCP layers are
> already live and verifiable via the smoke checks in
> [`DEPLOY_COOLIFY.md`](DEPLOY_COOLIFY.md#paso-7--smoke-test-terminal-local).
