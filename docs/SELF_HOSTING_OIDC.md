# Self-hosting Korva Vault with OIDC / SSO

Korva supports OpenID Connect (OIDC) for self-hosted Vault deployments
that need to plug into an enterprise identity provider — Okta, Azure
AD, Google Workspace, Authentik, Keycloak, or any other RFC-6749 +
RFC-7519 compliant IdP.

> **Status:** Phase 15.D. The OIDC routes are off by default. Setting
> the four required env vars enables them; missing any one var keeps
> them off so the default `docker compose up` stays minimal.

---

## TL;DR — five env vars, one IdP app registration

```bash
KORVA_OIDC_ISSUER_URL=https://idp.example.com
KORVA_OIDC_CLIENT_ID=korva-vault
KORVA_OIDC_CLIENT_SECRET=...redacted...
KORVA_OIDC_REDIRECT_URL=https://vault.example.com/auth/oidc/callback
KORVA_OIDC_ALLOWED_DOMAINS=acme.io,partner.dev   # optional
```

After `docker compose up -d vault`, point a browser at:

```
https://vault.example.com/auth/oidc/login
```

The Vault redirects you to the IdP, you authenticate, the IdP
redirects back to `/auth/oidc/callback`, and the Vault drops you on
`/app/overview` with a fresh team-member session.

---

## What problem does OIDC solve here?

Korva already supports two login flows for individual developers:

| Flow                       | Phase | When to use                                          |
| -------------------------- | ----- | ---------------------------------------------------- |
| Invite token + redeem      | 11    | First-time login, CLI / MCP usage                    |
| Email OTP re-login         | 12    | A previously-invited member re-authenticating        |
| **OIDC web flow**          | 15.D  | Enterprise SSO, browser-first ops, IdP-driven de-provisioning |

OIDC pairs naturally with the existing model: the team admin still
**pre-invites the user** (`POST /admin/teams/{team_id}/members`), and
the IdP only proves "this person currently controls that email
address." If a user is removed from the IdP, their next login fails;
existing sessions remain valid until they expire (30 days by default)
or the admin force-revokes them via `DELETE
/admin/teams/{team_id}/sessions/{session_id}`.

This split is deliberate. Implicit account provisioning ("first OIDC
login auto-creates a team member") would let any IdP user join any
team — exactly the multi-tenant boundary breach Korva's session model
is designed to prevent.

---

## End-to-end flow

```
┌─────────┐   1. GET /auth/oidc/login            ┌──────────┐
│ Browser ├────────────────────────────────────▶ │ Korva    │
│         │                                       │ Vault    │
│         │ ◀──────────────────────────────────── │          │
│         │   2. 302 to IdP authorize URL with    │          │
│         │      HMAC-signed state= param         └──────────┘
│         │      (no cookies set)
│         │
│         │   3. user authenticates at the IdP
│         │
│         │ ◀───── 4. IdP redirects to /auth/oidc/callback?code=...&state=...
│         │
│         │   5. GET /auth/oidc/callback           ┌──────────┐
│         ├─────────────────────────────────────▶  │ Vault    │
│         │                                        │          │
│         │                                        │ verify   │
│         │                                        │ state    │
│         │                                        │ HMAC +   │
│         │                                        │ id_token │
│         │                                        │ + lookup │
│         │                                        │ member   │
│         │                                        │ + mint   │
│         │                                        │ session  │
│         │ ◀───── 6. 302 /app/overview#session=<token> ─────┘
│         │
│         │   7. Beacon SPA reads window.location.hash,
│         │      stores token in sessionStorage, strips hash.
└─────────┘
```

Key invariants:
- **CSRF (Phase 17.A)**: the `state` URL param is a self-contained,
  HMAC-signed token: `issued_at(8B) || nonce(16B) || HMAC-SHA256(K, …)`.
  The signing key `K` is derived from `SHA256(admin.key bytes)` —
  unique per install, deterministic across vault restarts. The
  callback rejects tokens whose HMAC doesn't verify or whose
  `issued_at` is older than 10 minutes. **No cookies** are involved,
  so two browser tabs can start logins concurrently without one
  overwriting the other.
- **No token in URL log**: the session token rides in the URL
  fragment, which browsers never send to servers or to `Referer`
  headers. The SPA strips it via `history.replaceState` before mount.
- **No discovery at startup**: the Vault uses a lazy verifier — the
  IdP's `/.well-known/openid-configuration` is fetched on the first
  `/auth/oidc/*` request, not at boot. If the IdP is unreachable,
  `/healthz` still returns 200 and the Vault keeps serving the rest of
  its surface area.
- **Constant-time rejection (Phase 17.C)**: every 4xx response from
  `/auth/oidc/callback` is padded to a 100ms floor. The DB lookup
  for the email in `team_members` is naturally slower than checking
  a domain allowlist; without padding, a network observer could tell
  "this email is invited (just not approved)" from "this email isn't
  invited at all" by response latency. The padding flattens that
  curve. Successful redirects (302) are not padded.

---

## Required env vars (all four must be set)

| Variable                     | Example                                                       |
| ---------------------------- | ------------------------------------------------------------- |
| `KORVA_OIDC_ISSUER_URL`      | `https://accounts.google.com`                                 |
| `KORVA_OIDC_CLIENT_ID`       | `korva-vault-production`                                      |
| `KORVA_OIDC_CLIENT_SECRET`   | (whatever the IdP issued — keep out of source control)        |
| `KORVA_OIDC_REDIRECT_URL`    | `https://vault.example.com/auth/oidc/callback`                |

Conventions:
- `ISSUER_URL` should be the **issuer** value the IdP advertises, with
  no trailing slash. Korva trims trailing `/` defensively.
- `REDIRECT_URL` must match exactly what you registered with the IdP
  (path-sensitive, case-sensitive in some IdPs).

## Optional env vars

| Variable                     | Default                          | Notes                                  |
| ---------------------------- | -------------------------------- | -------------------------------------- |
| `KORVA_OIDC_ALLOWED_DOMAINS` | _(empty — allow all)_            | Comma-separated email-domain allowlist. |
| `KORVA_OIDC_SCOPES`          | `openid,email,profile`           | Override the scope list.               |

The allowlist matches the **email-domain suffix exactly** — sub-
domains are not implicit (`acme.io` does **not** match
`engineering.acme.io`). Case-insensitive on both sides. The leading
`@` is allowed and stripped (`@acme.io` ≡ `acme.io`).

---

## IdP-specific setup recipes

### Google Workspace

1. In Google Cloud Console → **APIs & Services** → **Credentials** →
   **Create credentials** → **OAuth client ID** → Application type
   **Web application**.
2. **Authorized redirect URIs**: add your full callback URL, e.g.
   `https://vault.example.com/auth/oidc/callback`.
3. Copy the **Client ID** + **Client secret**.
4. Env vars:
   ```bash
   KORVA_OIDC_ISSUER_URL=https://accounts.google.com
   KORVA_OIDC_CLIENT_ID=...apps.googleusercontent.com
   KORVA_OIDC_CLIENT_SECRET=...
   KORVA_OIDC_REDIRECT_URL=https://vault.example.com/auth/oidc/callback
   KORVA_OIDC_ALLOWED_DOMAINS=yourcompany.com
   ```

### Microsoft Entra ID (formerly Azure AD)

1. **App registrations** → **New registration**.
2. Supported account types: **Accounts in this organizational
   directory only** (single tenant).
3. Redirect URI: type **Web**, value
   `https://vault.example.com/auth/oidc/callback`.
4. **Certificates & secrets** → **New client secret**. Copy the
   *Value* (not the Secret ID).
5. **Token configuration** → add the **email** optional claim for
   id_tokens.
6. Env vars:
   ```bash
   KORVA_OIDC_ISSUER_URL=https://login.microsoftonline.com/<tenant-id>/v2.0
   KORVA_OIDC_CLIENT_ID=<application-(client)-id>
   KORVA_OIDC_CLIENT_SECRET=<secret-value>
   KORVA_OIDC_REDIRECT_URL=https://vault.example.com/auth/oidc/callback
   ```

### Okta

1. **Applications** → **Create App Integration** → **OIDC — OpenID
   Connect** → **Web Application**.
2. Sign-in redirect URI:
   `https://vault.example.com/auth/oidc/callback`.
3. Assignments: pick the groups that should be able to sign in
   (remember they still need a `team_members` row).
4. Env vars:
   ```bash
   KORVA_OIDC_ISSUER_URL=https://<your-org>.okta.com/oauth2/default
   KORVA_OIDC_CLIENT_ID=<client-id>
   KORVA_OIDC_CLIENT_SECRET=<client-secret>
   KORVA_OIDC_REDIRECT_URL=https://vault.example.com/auth/oidc/callback
   ```

### Keycloak / Authentik / Zitadel / self-hosted

1. Create a new **OIDC Confidential client**.
2. Set the **redirect URI** to your callback URL.
3. Use the discovery endpoint as `ISSUER_URL` (without `/.well-
   known/openid-configuration`).

---

## Pre-invite users before they can sign in

OIDC does NOT auto-provision team members. Run this once per user:

```bash
korva invite alice@acme.io --team eng --role member
```

Or, against a remote Vault:

```bash
curl -X POST https://vault.example.com/admin/teams/<team_id>/members \
  -H "X-Admin-Key: $KORVA_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@acme.io","role":"member"}'
```

Once that row exists, Alice can log in via
`https://vault.example.com/auth/oidc/login` and the callback will
mint her a session. If she hits OIDC before being invited, she gets
**403 Forbidden — no team membership found**. The fix is to invite
her, not to log in again.

---

## Reverse-proxy + TLS notes

The OIDC flow no longer sets cookies (Phase 17.A — the state lives
in a signed URL parameter instead), so there's no `Secure` flag for
Korva to decide. Transport security is fully delegated to the
reverse proxy:

- The vault should always be reached over HTTPS in production. The
  IdP requires the registered `KORVA_OIDC_REDIRECT_URL` to match
  exactly, including the scheme, so misconfiguring the proxy
  surfaces as an IdP-side rejection long before any request reaches
  the vault.
- The session token rides in the URL fragment of the final redirect.
  Fragments are never sent to servers or `Referer` headers, so the
  token never leaks through proxy access logs.
- No `X-Forwarded-*` headers are consulted by the OIDC handlers; you
  don't need to configure your proxy to forward them for OIDC
  specifically. (Other parts of Korva may still benefit — see
  DEPLOYMENT.md.)

---

## Troubleshooting

| Symptom                                                                | Likely cause                                                                                  |
| ---------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `503 — OIDC is not configured on this vault`                           | One of the four required env vars is empty. Check `docker compose config`.                    |
| `503 — OIDC discovery failed`                                          | Vault container can't reach the IdP (DNS, firewall, or wrong issuer URL).                     |
| `400 — invalid state — possible CSRF or expired token`                 | The signed state failed HMAC verification (tampered or signed by a different admin.key) OR is older than 10 minutes. Restart from `/auth/oidc/login`. |
| `403 — email is not verified at the IdP — contact your admin`          | The id_token has `email_verified: false`. Verify the email at the IdP side first.             |
| `403 — email domain is not in KORVA_OIDC_ALLOWED_DOMAINS`              | Adjust the env var or the user's email.                                                       |
| `403 — no team membership found for this email — ask your admin`       | Pre-invite the user with `korva invite` or via the admin REST API.                            |
| `401 — id_token verification failed: ...`                              | Clock skew, expired token, or signature mismatch. Check the IdP's JWKS endpoint is reachable. |

For deeper debugging, the Vault logs every OIDC error to stderr; tail
with `docker compose logs -f vault | grep -i oidc`.

---

## Disabling OIDC

Clear any one of the four required env vars and restart:

```bash
KORVA_OIDC_ISSUER_URL= docker compose up -d vault
```

The `/auth/oidc/*` routes will no longer be registered and return 404.
The invite-token + OTP flows are unaffected.
