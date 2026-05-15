# Troubleshooting — Korva Vault

> Top errors with copy-paste solutions. For workflow-level scenarios
> (admin key rotation, OIDC failures by class, etc.) see
> [RUNBOOK.md](RUNBOOK.md). For upgrade-specific issues see
> [UPGRADE.md](UPGRADE.md).

*Last updated: 2026-05-14*

---

## HTTP errors from the vault

### `503 — OIDC is not configured on this vault`

**Cause:** One of the four required OIDC env vars is empty.

**Fix:**
```bash
docker compose config | grep KORVA_OIDC
# Confirm all four are set:
#   KORVA_OIDC_ISSUER_URL
#   KORVA_OIDC_CLIENT_ID
#   KORVA_OIDC_CLIENT_SECRET
#   KORVA_OIDC_REDIRECT_URL
```
Restart vault after adding the missing var. Full setup → [SELF_HOSTING_OIDC.md](SELF_HOSTING_OIDC.md).

### `503 — OIDC discovery failed`

**Cause:** Vault container can't reach the IdP's `/.well-known/openid-configuration`.

**Fix:**
```bash
# From inside the container:
docker exec korva-vault wget -qO- $KORVA_OIDC_ISSUER_URL/.well-known/openid-configuration | head -10
# If it hangs/fails, fix DNS or the egress firewall — IdP discovery is mandatory.
```

### `400 — invalid state — possible CSRF or expired token, restart the login`

**Cause:** OIDC state token failed HMAC verification OR is older than 10 minutes.

**Likely scenarios:**
- User opened the IdP redirect, walked away for an hour, then returned.
- `admin.key` was rotated mid-flow (Phase 17.A signs state with `SHA256(admin.key)` — see [RUNBOOK Admin key rotation](RUNBOOK.md#admin-key-rotation)).
- Vault's clock drifted ≥ 10 min from the IdP's clock.

**Fix:** Restart from `/auth/oidc/login`. If frequent, check NTP sync.

### `403 — no team membership found for this email — ask your admin to invite you first`

**Cause:** OIDC authenticated the user successfully but they aren't in the `team_members` table.

**Fix:**
```bash
# Admin invites the user first.
curl -X POST http://localhost:7437/admin/teams/$TEAM_ID/members \
  -H "X-Admin-Key: $(cat ~/.korva/admin.key)" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@acme.io","role":"member"}'
# Then the user retries OIDC login — succeeds.
```

### `401 — X-Session-Token header required`

**Cause:** Caller hit a session-protected endpoint (e.g. `/team/skills`) without a session token.

**Fix:** Run `korva auth <invite-token>` to get a session, OR set `X-Session-Token: <token>` from the dashboard's auth state.

### `429 Too Many Requests`

**Cause:** Per-IP rate limit (120 req/min, see `vault/internal/api/router.go:23` `cleanupInterval`).

**Fix:** Wait. If legitimate traffic, raise the cap by editing `NewRateLimiter(120, time.Minute)` and rebuilding (no env knob today — file an issue if you need this configurable).

### `500 — DB lock timeout` / `database is locked`

See [RUNBOOK DB locked or unreachable](RUNBOOK.md#db-locked-or-unreachable).

---

## Docker / docker-compose issues

### `docker compose up` fails immediately with "no such image"

**Cause:** The image `ghcr.io/alcandev/korva-vault:latest` couldn't be pulled — either ghcr.io is unreachable, or the tag was retracted.

**Fix:**
```bash
docker pull ghcr.io/alcandev/korva-vault:latest
# If it fails:
docker login ghcr.io   # ghcr.io may require auth depending on your setup
```

### `wget: server returned error: HTTP/1.1 503` in the docker healthcheck

**Cause:** The healthcheck (`Dockerfile:57`) greps for `"status":"ok"`. After Phase 20.B the response shape stays compatible — but if you see this on a fresh deployment, the vault hasn't finished migrations.

**Fix:** Wait 30s. If persistent, look at `docker compose logs vault` for migration errors. Healthcheck should NOT use `/readyz` — it would restart-loop on transient DB locks.

### Volume permission errors after upgrade

**Cause:** New image uses uid/gid that doesn't match the host bind mount.

**Fix:** The image runs as `korva` user (uid auto-assigned by Alpine). Use named volumes (`korva-data`) instead of bind mounts in production. If you must bind-mount:
```bash
sudo chown -R 1000:1000 /path/to/host/data   # or whatever the container uid is
```

---

## SQLite / DB issues

### `database is locked` (intermittent)

**Cause:** Long-running write transaction. SQLite serializes writes (`SetMaxOpenConns(1)`).

**Fix:** Most often self-heals when the offending request completes. If persistent:
```bash
sqlite3 ~/.korva/vault.db 'PRAGMA wal_checkpoint(TRUNCATE);'
```
If still stuck, restart the vault.

### `vault.db-wal` is huge (> 100 MB)

**Cause:** WAL checkpointing isn't keeping up with write volume. Usually means a long reader holding open snapshots.

**Fix:** Restart vault to drain. WAL recreates on first write. If recurring, consider whether the read patterns can be batched.

### "too many open files"

**Cause:** Process file descriptor limit too low for the request volume.

**Fix:**
```bash
# Check current limit (inside container):
docker exec korva-vault sh -c 'ulimit -n'
# Raise in docker-compose.yml:
#   ulimits:
#     nofile:
#       soft: 65536
#       hard: 65536
```

---

## Beacon dashboard issues

### "Vault offline" banner stays even after vault is up

**Cause:** Beacon's `useVaultHealth` hook caches errors for 30s. Stale state persists in React Query.

**Fix:** Hard reload the browser tab. If you see the banner persistently with `/healthz` responding 200 from `curl`, check the browser console for CORS errors — likely `KORVA_CORS_ORIGIN` doesn't match the Beacon's URL.

### Editor adoption widget shows zeros after a long-running deployment

See [RUNBOOK Editor telemetry not arriving](RUNBOOK.md#editor-telemetry-not-arriving).

### "session expired" loop after OIDC login

**Cause:** The SPA's session-token consumer expects `#session=<token>` in the URL fragment after redirect. If the IdP redirected to a non-SPA URL (e.g. directly to `/api/...`), the token never lands.

**Fix:** Ensure `KORVA_OIDC_REDIRECT_URL` points at the vault's `/auth/oidc/callback`, NOT at the SPA. The vault then redirects to `/app/overview#session=<token>`, which the SPA's `consumeOIDCSessionFromURL` (Phase 15.D) picks up.

---

## CLI issues

### `korva harness review` exits non-zero on a passing spec

**Cause:** Without `--record`, the CLI returns non-zero whenever there are issues — that's the contract for CI gates. Even info-level findings can trip it depending on your `harness check` config.

**Fix:** Use `--record` if you're a reviewer taking responsibility for the verdict; the exit becomes 0 (Phase 18.A). Or pipe through a wrapper that reads the JSON and decides:
```bash
korva harness review 1 --json | jq -e '.ok'
```

### `korva harness check` fails after upgrading the vault

**Cause:** The CLI's view of valid statuses moved with a vault upgrade. Pre-1.30 CLIs don't know about `spec_ready`.

**Fix:** Upgrade the CLI to match the vault. The CLI is a separate binary; install it via your usual package channel.

### `korva auth <token>` returns "invalid or expired invite token"

**Causes (in order):**
1. The token already redeemed (one-time use).
2. The token expired (default 7 days).
3. Wrong vault — the CLI is talking to a different vault than the one that issued the invite.

**Fix:**
```bash
# Confirm which vault the CLI is using:
korva status | grep -i endpoint
# If wrong, set KORVA_VAULT_URL or fix ~/.korva/config.json.
# If expired, ask admin for a fresh invite.
```

---

## When all else fails

```bash
# Capture diagnostics for an issue report.
mkdir -p /tmp/korva-diag
docker compose logs --no-color vault > /tmp/korva-diag/vault.log
curl -s http://localhost:7437/healthz > /tmp/korva-diag/healthz.json
curl -sf http://localhost:7437/readyz   > /tmp/korva-diag/readyz.json   || true
curl -s -H "X-Admin-Key: $(cat ~/.korva/admin.key)" \
  http://localhost:7437/admin/system-status > /tmp/korva-diag/system-status.json
sqlite3 ~/.korva/vault.db 'PRAGMA integrity_check;'  > /tmp/korva-diag/integrity.txt
sqlite3 ~/.korva/vault.db .schema                    > /tmp/korva-diag/schema.sql
tar czf /tmp/korva-diag.tar.gz /tmp/korva-diag

# Strip secrets before sharing externally — admin.key, session
# tokens, and email addresses may appear in the logs.
```

Open an issue at https://github.com/AlcanDev/korva/issues with the `.tar.gz` attached.
