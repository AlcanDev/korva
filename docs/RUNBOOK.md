# Runbook — Korva Vault Operations

> Production scenarios with copy-paste commands. Update this file
> when you encounter a new failure mode in production — your future
> self (and on-call rotation) will thank you.

*Last updated: 2026-05-14 — covers Korva ≥ 1.32*

---

## Quick reference

| Scenario | Section |
|---|---|
| `/readyz` returns 503 | [DB locked or unreachable](#db-locked-or-unreachable) |
| Admin lost their `admin.key` | [Admin key rotation](#admin-key-rotation) |
| Sessions all expired at once | [Session TTL + bulk re-issue](#session-ttl--bulk-re-issue) |
| OIDC login fails with "state mismatch" | [OIDC failure modes](#oidc-failure-modes) |
| Hive worker isn't syncing | [Hive sync stalled](#hive-sync-stalled) |
| Vault crashed and won't start | [Vault crash recovery](#vault-crash-recovery) |
| Need to apply a schema change | [Schema migration policy](#schema-migration-policy) |
| Editor adoption widget shows zeros | [Editor telemetry not arriving](#editor-telemetry-not-arriving) |

---

## DB locked or unreachable

**Symptom:** `/readyz` returns 503 with `checks.db != "ok"`. Beacon shows "Vault offline." All `/admin/*` requests fail.

**Diagnosis:**

```bash
# 1. Probe both endpoints — liveness first.
curl -s http://localhost:7437/healthz  | jq .   # should always return ok
curl -sf http://localhost:7437/readyz   | jq .  # 503 = problem
```

If `/healthz` is 200 and `/readyz` is 503, the process is up but the DB ping failed. Likely causes (in order of frequency):

1. **A long-running write transaction is holding a write lock.** SQLite serializes writes on a single connection (`SetMaxOpenConns(1)` per `internal/db/sqlite.go:62`). A stuck Hive worker or admin export can block readers.
2. **Disk full.** `df -h /data` (Docker) or wherever `KORVA_VAULT_DB` points.
3. **WAL file ballooned.** `~/.korva/vault.db-wal` > 100 MB suggests a checkpoint never ran.

**Fixes:**

```bash
# 1. Check who's holding the lock (Linux only).
lsof | grep vault.db

# 2. Force a WAL checkpoint to release pent-up writes.
sqlite3 ~/.korva/vault.db 'PRAGMA wal_checkpoint(TRUNCATE);'

# 3. Restart the vault — drops all in-flight transactions.
docker compose restart vault
# OR for systemd:
sudo systemctl restart korva-vault
```

If the DB file is genuinely corrupt:

```bash
# Verify integrity — output "ok" means the file is fine.
sqlite3 ~/.korva/vault.db 'PRAGMA integrity_check;'

# If corrupt, restore from backup (see UPGRADE.md for backup policy).
mv ~/.korva/vault.db ~/.korva/vault.db.broken-$(date +%s)
cp /path/to/backup/vault.db ~/.korva/vault.db
```

---

## Admin key rotation

**Scenario:** The current `admin.key` is suspected compromised, or the operator who held it left.

**Procedure:**

```bash
# 1. Create a new admin key file. The vault accepts any non-empty string.
openssl rand -hex 32 > ~/.korva/admin.key.new
chmod 600 ~/.korva/admin.key.new

# 2. Rotate atomically.
mv ~/.korva/admin.key      ~/.korva/admin.key.old
mv ~/.korva/admin.key.new  ~/.korva/admin.key

# 3. Restart so OIDC's signed-state derivation picks up the new key.
docker compose restart vault

# 4. Update every Beacon admin user's stored key. The Beacon login
#    page accepts the new value; existing sessions (X-Session-Token)
#    are unaffected by admin-key rotation.

# 5. Securely destroy the old key.
shred -u ~/.korva/admin.key.old
```

**Important — Phase 17.A side effect:** the OIDC state-token signing key is derived from `SHA256(admin.key)`. After rotation, every in-flight OIDC login (the user has clicked "login" but not yet returned from the IdP) will fail with `invalid state — possible CSRF or expired token`. The 10-minute TTL means this clears within minutes; users just retry. No persistent damage.

---

## Session TTL + bulk re-issue

**Scenario:** Sessions are valid for 30 days (`vault/internal/api/auth_session.go:18`). All sessions issued near the same date will expire near the same date. Symptom: many users report "session expired" the same morning.

**Mitigation (proactive):**

```bash
# Query upcoming expiries.
sqlite3 ~/.korva/vault.db <<'EOF'
SELECT email, team_id, expires_at
  FROM member_sessions
 WHERE expires_at < datetime('now', '+3 days')
 ORDER BY expires_at;
EOF

# Re-issue invites in advance for users about to expire.
for email in alice@acme.io bob@acme.io; do
  curl -X POST http://localhost:7437/admin/teams/$TEAM_ID/invites \
    -H "X-Admin-Key: $(cat ~/.korva/admin.key)" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\"}"
done
```

**Reactive (after expiry):** users hit `/auth/otp/request` (Phase 12) which mints a one-time code by email. No admin intervention required if the mailer is configured.

---

## OIDC failure modes

| Error response | Likely cause | Fix |
|---|---|---|
| `503 OIDC is not configured on this vault` | One of `KORVA_OIDC_ISSUER_URL` / `CLIENT_ID` / `CLIENT_SECRET` / `REDIRECT_URL` is empty | `docker compose config | grep KORVA_OIDC` |
| `503 OIDC discovery failed` | Vault container can't reach the IdP | `docker exec korva-vault wget -qO- $KORVA_OIDC_ISSUER_URL/.well-known/openid-configuration` |
| `400 invalid state — possible CSRF or expired token` | Token > 10 min old, OR admin.key rotated mid-flow (see [Admin key rotation](#admin-key-rotation)), OR genuine CSRF attempt | Restart from `/auth/oidc/login`. If frequent, check clock drift (≥ 10 min skew breaks state validation). |
| `403 email is not verified at the IdP` | The IdP issued an `id_token` with `email_verified: false` | Verify the email at the IdP side (Google: re-confirm in Workspace admin; Azure AD: enforce email verification policy) |
| `403 email domain is not in KORVA_OIDC_ALLOWED_DOMAINS` | Domain allowlist mismatch | Update `KORVA_OIDC_ALLOWED_DOMAINS` env var, restart vault |
| `403 no team membership found for this email` | OIDC works but the user isn't in `team_members` yet | `korva invite <email> --team <id>` (admin must pre-invite — see SELF_HOSTING_OIDC.md) |

---

## Hive sync stalled

**Symptom:** Beacon's "Hive sync" widget shows `last_sync_at` minutes/hours stale. `/api/v1/hive/status` reports `pending_count` growing without bound.

**Diagnosis:**

```bash
# 1. Read the worker status.
curl -s http://localhost:7437/api/v1/hive/status \
  -H "X-Admin-Key: $(cat ~/.korva/admin.key)" | jq .

# 2. Check the outbox for stuck rows.
sqlite3 ~/.korva/vault.db <<'EOF'
SELECT status, COUNT(*) FROM cloud_outbox GROUP BY status;
EOF

# 3. Tail the vault log for hive errors.
docker compose logs -f vault | grep -i hive
```

Common causes:

- **Network change:** Hive endpoint moved (default `https://hive.korva.dev`). Verify with `curl $KORVA_HIVE_ENDPOINT/v1/health`.
- **Killed by privacy filter:** rows with `status=rejected_privacy` are correctly skipped — not stuck.
- **License expired:** Teams-only features may degrade. Check `/admin/license/status`.

**Recovery:**

```bash
# Pause sync for one project (keeps queueing, stops sending).
curl -X POST http://localhost:7437/admin/hive/projects/<project>/pause \
  -H "X-Admin-Key: $(cat ~/.korva/admin.key)"

# Resume after fixing the upstream issue.
curl -X POST http://localhost:7437/admin/hive/projects/<project>/resume \
  -H "X-Admin-Key: $(cat ~/.korva/admin.key)"

# Disable Hive entirely (kill switch — survives restarts).
echo 'KORVA_HIVE_DISABLE=1' >> .env
docker compose restart vault
```

---

## Vault crash recovery

**Symptom:** `docker compose ps` shows the vault container as `restarting` or `exited`. `docker logs korva-vault` ends with a panic, OOM, or "database is locked".

**Sequence:**

```bash
# 1. Capture the crash log BEFORE restarting.
docker compose logs --no-color vault > /tmp/vault-crash-$(date +%s).log

# 2. Verify the DB is intact (don't restart yet).
sqlite3 ~/.korva/vault.db 'PRAGMA integrity_check;'

# 3. If integrity is ok, restart.
docker compose restart vault

# 4. Watch /readyz come back.
until curl -sf http://localhost:7437/readyz | grep -q '"ready"'; do
  echo waiting; sleep 2
done
```

If the integrity check fails, see [DB locked or unreachable](#db-locked-or-unreachable) — restore from backup.

**Auto-restart policy:** `docker-compose.yml` sets `restart: unless-stopped`. K8s deployments should set the same. Liveness probes use `/healthz` (process check), not `/readyz` (which would cause restart loops on transient DB issues — see Phase 20.B).

---

## Schema migration policy

Korva's migrations are **append-only and idempotent**. See `internal/db/migrations.go`:

- New tables use `CREATE TABLE IF NOT EXISTS`.
- New columns use `ALTER TABLE ... ADD COLUMN`. The `Migrate` function ignores `duplicate column name` errors so re-running on an already-migrated DB is safe.
- We **never** drop or rename columns. Breaking changes require a new column + dual-write window + flip + drop in three separate releases.

**What this means for you:**

- Upgrading Korva is safe to do without manual SQL — the binary applies the diff on startup.
- Downgrading Korva is also safe — older code ignores newer columns. You CAN'T downgrade through a release that added a constraint, but the project guarantees not to.
- Always backup before upgrading anyway (see UPGRADE.md). The guarantee is best-effort, not contractual.

---

## Editor telemetry not arriving

**Symptom:** Beacon's "Editor adoption" widget on Overview shows total=0 or only "anonymous" rows after a long-running deployment.

**Diagnosis:**

```bash
# 1. Confirm interactions are arriving at all.
curl -s "http://localhost:7437/admin/interactions" \
  -H "X-Admin-Key: $(cat ~/.korva/admin.key)" | jq '.calls | length'

# 2. Confirm the editor column has values.
sqlite3 ~/.korva/vault.db <<'EOF'
SELECT editor, COUNT(*) FROM interactions GROUP BY editor;
SELECT editor, COUNT(*) FROM mcp_calls    GROUP BY editor;
EOF
```

If both tables show only `""` (empty editor), no client is sending the signal. Two channels:

- **HTTP**: client must send `X-Korva-Editor: <id>` on `POST /api/v1/interactions`. See HARNESS_EDITORS.md for the per-editor recipe.
- **MCP**: client must send `clientInfo.name` in the `initialize` message (most modern MCP clients do — Phase 19.A). If your client doesn't, it'll show as anonymous.

To **disable** the telemetry entirely:

```bash
echo 'KORVA_EDITOR_TELEMETRY_DISABLE=1' >> .env
docker compose restart vault
```

---

## Don't see your scenario here?

Add it. Open a PR with the symptom + diagnosis + fix in the same shape as the existing entries. The runbook is only useful if it stays current.
