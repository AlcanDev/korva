# Korva Licensing — Operator & User Guide

This document covers the full licensing lifecycle: how to set up and operate the licensing server (for Korva operators), and how end-users activate a Korva for Teams license.

---

## Table of Contents

1. [Architecture](#1-architecture)
2. [Operator: Generate the RSA Key Pair](#2-operator-generate-the-rsa-key-pair)
3. [Operator: Deploy the Licensing Server](#3-operator-deploy-the-licensing-server)
4. [Operator: Issue a License](#4-operator-issue-a-license)
5. [User: Activate a License](#5-user-activate-a-license)
6. [User: Check License Status](#6-user-check-license-status)
7. [User: Deactivate a Seat](#7-user-deactivate-a-seat)
8. [Offline Verification & Grace Period](#8-offline-verification--grace-period)
9. [Key Rotation](#9-key-rotation)
10. [Security Considerations](#10-security-considerations)

---

## 1. Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Licensing Server (korva-licensing — port 7440)             │
│                                                              │
│  POST /v1/issue      ← operator only (admin bearer secret)  │
│  POST /v1/activate   ← user (license_key + install_id)      │
│  POST /v1/heartbeat  ← vault (every 24h, license_id)        │
│  POST /v1/deactivate ← user (license_id + install_id)       │
│  GET  /v1/health     ← monitoring                           │
└────────────────────────────────┬─────────────────────────────┘
                                 │  Issues JWS (RS256)
                                 ▼
              ┌──────────────────────────────┐
              │  ~/.korva/license.key        │
              │  JWS RS256 signed by         │
              │  priv.pem (server only)      │
              └──────────┬───────────────────┘
                         │  Verified offline by
                         ▼
              ┌──────────────────────────────┐
              │  internal/license/keys/      │
              │  pubkey.pem (embedded in     │
              │  korva-vault binary)         │
              └──────────────────────────────┘
```

**Key properties:**
- The private key (`priv.pem`) lives **only** on the licensing server. Never distribute it.
- The public key (`pubkey.pem`) is embedded into `korva-vault` at build time via `//go:embed`.
- License verification is **100% offline** — no call to the server on vault start.
- The licensing server is only contacted at: activate, heartbeat (24h), deactivate.

---

## 2. Operator: Generate the RSA Key Pair

Run this **once** before your first production deployment:

```bash
make keygen
# → writes priv.pem (RSA-4096, 0600) and pubkey.pem (0644) to the current directory
# → prints deployment instructions
```

Then:

1. **Copy `pubkey.pem` into the binary:**

   ```bash
   cp pubkey.pem internal/license/keys/pubkey.pem
   # Rebuild korva-vault — the new key is embedded via //go:embed
   go build github.com/alcandev/korva/vault/cmd/korva-vault
   ```

2. **Store `priv.pem` securely:**
   - Upload as a secret to your deployment platform
   - Never commit it to Git
   - Never copy it to developer machines

> **CRITICAL:** If `priv.pem` leaks, an attacker can issue unlimited valid licenses. Treat it like a payment signing key.

---

## 3. Operator: Deploy the Licensing Server

### Option A — Docker (recommended)

```bash
# Build the image (from repo root — go.work spans all modules)
docker build \
  -f forge/licensing-server/Dockerfile \
  -t ghcr.io/alcandev/korva-licensing:latest \
  .

# Run (supply secrets via env vars — never bake them into the image)
docker run -d \
  --name korva-licensing \
  -p 7440:7440 \
  -v korva-licensing-data:/data \
  -e KORVA_LICENSING_ADMIN_SECRET="$(openssl rand -hex 32)" \
  -e KORVA_LICENSING_PRIVATE_KEY_PEM="$(cat priv.pem)" \
  ghcr.io/alcandev/korva-licensing:latest
```

### Option B — Docker Compose (vault + licensing)

```bash
# Set required env vars in .env (never commit this file)
cat > .env <<EOF
KORVA_LICENSING_SECRET=$(openssl rand -hex 32)
KORVA_LICENSING_KEY_PEM=$(cat priv.pem | tr '\n' '~' | sed 's/~/\\n/g')
EOF

docker compose --profile teams up -d
```

### Option C — Binary directly

```bash
KORVA_LICENSING_PORT=7440 \
KORVA_LICENSING_ADMIN_SECRET="your-secret" \
KORVA_LICENSING_PRIVATE_KEY_FILE=/path/to/priv.pem \
  ./korva-licensing
```

### Health check

```bash
curl https://licensing.your-domain.com/v1/health
# → {"ok":true,"service":"korva-licensing"}
```

---

## 4. Operator: Issue a License

Use the `/v1/issue` endpoint (requires the admin bearer secret set at deploy time):

```bash
curl -X POST https://licensing.your-domain.com/v1/issue \
  -H "Authorization: Bearer YOUR_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "customer_email": "alice@corp.com",
    "seats": 5,
    "expire_days": 365,
    "tier": "teams",
    "features": ["private_scrolls", "custom_whitelist", "audit_log", "admin_skills", "multi_profile"]
  }'
```

Response:

```json
{
  "license_id": "lic_01hx...",
  "license_key": "KORVA-A1B2-C3D4-E5F6-G7H8",
  "customer_email": "alice@corp.com",
  "seats": 5,
  "tier": "teams",
  "features": ["private_scrolls", "custom_whitelist", "audit_log", "admin_skills", "multi_profile"],
  "expires_at": "2027-04-18T00:00:00Z"
}
```

Send the `license_key` to the customer. The `license_id` is internal.

**Defaults when fields are omitted:**
- `seats` — 5
- `expire_days` — 365
- `tier` — `"teams"`
- `features` — all Teams features

---

## 5. User: Activate a License

```bash
# The license key you received looks like: KORVA-XXXX-XXXX-XXXX-XXXX
korva license activate KORVA-A1B2-C3D4-E5F6-G7H8
```

What this does:
1. Reads `~/.korva/install.id` (generated by `korva init`)
2. Sends `{license_key, install_id}` to the activation endpoint
3. Receives a JWS token (RS256, signed by the server's private key)
4. Stores the JWS at `~/.korva/license.key`
5. The vault verifies the JWS locally on every start — no further network call needed

> **Seat limit:** Each unique `install_id` consumes one seat. If you reinstall on the same machine, the same `install_id` is reused (free renewal). Activating on a new machine when all seats are occupied returns HTTP 402.

---

## 6. User: Check License Status

```bash
korva license status
```

Example output:

```
License:  lic_01hx...
Tier:     teams
Seats:    3 / 5 active
Features: private_scrolls, custom_whitelist, audit_log, admin_skills, multi_profile
Expires:  2027-04-18 (364 days remaining)
Heartbeat: 2026-04-18 09:14:32 UTC (18 hours ago)
Grace:    7 days remaining before offline degradation
```

---

## 7. User: Deactivate a Seat

Free up a seat (e.g., when decommissioning a machine):

```bash
korva license deactivate
```

This contacts the licensing server, removes the activation record, and deletes `~/.korva/license.key`. The seat is immediately available for reuse on another machine.

---

## 8. Offline Verification & Grace Period

The vault verifies the license **without any network call** on startup:

1. Reads `~/.korva/license.key` (the JWS)
2. Verifies the RS256 signature against the public key embedded in the binary
3. Checks the expiry field in the JWS payload
4. Reads `~/.korva/license.state.json` to check when the last heartbeat occurred

**Grace period:** If the last heartbeat was more than `grace_days` ago (default: 7 days), `HasFeature()` returns `false` and the vault degrades to Community tier with a banner in Beacon. Normal operation resumes automatically when the heartbeat succeeds again.

**Air-gapped environments:** Contact support for extended grace periods or static license files.

---

## 9. Key Rotation

### Rotating the private key (server side)

1. Generate a new key pair: `make keygen`
2. Update `internal/license/keys/pubkey.pem` with the new public key
3. The `verify.go` init block supports multiple `kid` values — add the new kid alongside the old one so existing licenses remain valid during transition
4. Deploy the new `korva-vault` binary
5. Update the licensing server with the new private key
6. All new activations will use the new key; existing JWS tokens remain valid until they expire

### Current supported key IDs (`kid`)

| kid | Status | Notes |
|-----|--------|-------|
| `korva-license-dev` | Dev only | Embedded in `forge/licensing-mock` — never used in production |
| `korva-license-v1` | Production | Replace `internal/license/keys/pubkey.pem` with your actual public key before shipping |

---

## 10. Security Considerations

| Risk | Mitigation |
|------|-----------|
| `priv.pem` leak | Deploy as a secret (env var or secret manager), never commit; rotate immediately if leaked |
| Admin secret leak | Use `openssl rand -hex 32`; rotate via env var update + container restart |
| License key share | Seat limit enforces per-machine activation; sharing a key across > N machines returns 402 |
| JWS replay | Expiry embedded in payload; revocation via `deactivate` removes the DB row |
| Key pinning bypass | `pubkey.pem` is compiled into the binary via `//go:embed` — tampering requires rebuilding |

*Last updated: 2026-04-30*
