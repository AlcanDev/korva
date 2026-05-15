# Upgrade Guide — Korva Vault

> Practical, copy-paste safe upgrade procedure for production Korva
> deployments. Read this BEFORE bumping the image tag.

*Last updated: 2026-05-14 — applies to ≥ 1.30 → next.*

---

## TL;DR — the 30-second upgrade

```bash
# 1. Backup. Always backup. Do not skip.
docker exec korva-vault sqlite3 /data/vault.db ".backup /data/backup-$(date +%Y%m%d-%H%M%S).db"
cp ~/.korva/admin.key ~/.korva/admin.key.backup-$(date +%Y%m%d)

# 2. Pull the new image + restart.
docker compose pull vault
docker compose up -d vault

# 3. Wait for /readyz to come back.
until curl -sf http://localhost:7437/readyz | grep -q '"ready"'; do
  echo waiting; sleep 2
done

# 4. Verify the version.
curl -s http://localhost:7437/healthz | jq .version
```

If anything goes wrong, see [Rollback procedure](#rollback-procedure).

---

## Pre-upgrade checklist

| Step | Why |
|---|---|
| 🔴 **Backup `vault.db`** | The only durable state. Migrations are append-only but disk failure during upgrade isn't the only risk. |
| 🔴 **Backup `admin.key`** | Lose it and you lose admin access; you also lose the OIDC state-signing key (Phase 17.A). Recovery requires re-issuing all in-flight OIDC sessions. |
| 🟡 **Note the current version** | `curl -s http://localhost:7437/healthz | jq .version` — needed for rollback. |
| 🟡 **Read the CHANGELOG entry** | Major version bumps may carry breaking changes; minor + patch are safe. |
| 🟢 **Drain traffic if you have multiple instances** | `/readyz` returning 503 keeps load balancers honest. |

---

## What's safe between versions

Korva's compatibility guarantees — these hold for every release ≥ 1.30:

- **Schema is append-only.** No column ever gets dropped or renamed in a single release. Adding a new column always uses `ALTER TABLE ADD COLUMN` with a `DEFAULT`, so old binaries that don't know about the column keep working. See `internal/db/migrations.go` for the policy.
- **API responses gain fields, never lose them.** The contract is "all documented fields stay; new fields may appear." Forward-compat clients ignore unknown fields.
- **HTTP routes are additive.** A removed route gets one minor-version deprecation notice in the CHANGELOG before deletion.
- **CLI flags follow the same.** New flags are added; deprecated ones print a warning for at least one minor version before removal.

What we don't guarantee:

- **Behavior under undocumented fields.** If you've been depending on a field that's not in the public API surface, it may move.
- **Beacon URL paths or class names.** Beacon is a UI; integrate via the vault's HTTP API instead.

---

## Compatibility matrix

| From → To | Notes |
|---|---|
| 1.30 → 1.31 | Phase 17 — OIDC state moved from cookie to signed token. **Existing users see no change.** OIDC operators: 17.A's signed state is derived from `admin.key` SHA256, so cross-restart is fine; cross-rotation invalidates in-flight logins (see [RUNBOOK Admin key rotation](RUNBOOK.md#admin-key-rotation)). |
| 1.31 → 1.32 | Phase 18 — `Feature.Review` added, `interactions.editor` column. Both additive. |
| 1.32 → 1.33 | Phase 19 — `mcp_calls.editor` column, optional `Rules.RequireApprovedReview` (off by default). |
| 1.33 → 1.34 | Phase 20 — `/readyz` endpoint added (additive); two store bugs fixed (`replay.go` interactions timestamps + `code_health.go` deadlock). No schema or API breaks. |

Read the CHANGELOG between any two versions before skipping more than one minor.

---

## Standard upgrade (single-instance)

```bash
# Step 1 — backup
TS=$(date +%Y%m%d-%H%M%S)
docker exec korva-vault sqlite3 /data/vault.db ".backup /data/backup-${TS}.db"
docker cp korva-vault:/data/backup-${TS}.db /tmp/vault-backup-${TS}.db
cp ~/.korva/admin.key ~/.korva/admin.key.backup-${TS}

# Step 2 — pull the new image (verify it exists first).
docker pull ghcr.io/alcandev/korva-vault:latest

# Step 3 — restart with the new image.
docker compose up -d vault

# Step 4 — wait for readiness, NOT just liveness.
# /healthz returns 200 the moment the process responds; /readyz
# waits for the DB to be reachable AND migrations to finish.
until curl -sf http://localhost:7437/readyz | jq -e '.status == "ready"' >/dev/null; do
  echo "  waiting for readyz..."
  sleep 2
done

# Step 5 — sanity check.
curl -s http://localhost:7437/healthz | jq .
curl -s http://localhost:7437/readyz  | jq .
```

If `/readyz` doesn't come back within 30s, see [Rollback procedure](#rollback-procedure).

---

## Standard upgrade (multi-instance behind a load balancer)

```bash
# Per instance, in sequence:

# 1. Drain — stop sending traffic. Most LBs use /readyz; force a
#    503 response by stopping the vault gracefully.
docker compose stop vault   # SIGTERM, then SIGKILL after 10s

# 2. Backup the local DB (each instance has its own SQLite file
#    by default — Korva is single-writer, so multi-instance ==
#    multi-vault, each owning its own data).
TS=$(date +%Y%m%d-%H%M%S)
docker run --rm -v korva-data:/data alpine \
  sqlite3 /data/vault.db ".backup /data/backup-${TS}.db"

# 3. Pull + restart.
docker compose pull vault
docker compose up -d vault

# 4. Wait for /readyz, then move to the next instance.
```

---

## Rollback procedure

```bash
# 1. Stop the new image.
docker compose stop vault

# 2. Restore the backup DB.
TS=<timestamp from your backup>
docker cp /tmp/vault-backup-${TS}.db korva-vault:/data/vault.db
cp ~/.korva/admin.key.backup-${TS} ~/.korva/admin.key

# 3. Pin the previous image tag in docker-compose.yml.
#    Replace `:latest` with the version you noted before upgrade.
sed -i.bak 's|korva-vault:latest|korva-vault:1.32.1|' docker-compose.yml

# 4. Start the old image with the restored data.
docker compose up -d vault
```

Because the schema is append-only, a downgrade reads the new
columns as if they were `DEFAULT ''` (or the column-specific
default). No data is lost.

---

## Special cases

### Upgrading across an OIDC state-format change

This **can't happen accidentally** — OIDC state changes carry a major-version bump and an explicit CHANGELOG note. As of Phase 17.A the format is `issued_at(8B) || nonce(16B) || HMAC-SHA256`. If a future release changes it, the upgrade notes will spell out the in-flight-login impact.

### Upgrading with a non-default `KORVA_VAULT_DB` path

Adjust the backup path:

```bash
docker exec korva-vault sqlite3 $KORVA_VAULT_DB ".backup ${KORVA_VAULT_DB%.db}-backup.db"
```

### Upgrading the licensing server

Different cycle from the Vault. See `docker-compose.yml` for the `licensing` service profile (`docker compose --profile teams ...`). Backup `licensing.db` separately.

---

## After every upgrade

- Verify a few smoke flows in Beacon (Vault dashboard, Sessions, Harness if you use it).
- Check `/admin/system-status` for unexpected warnings.
- If you use OIDC: do a fresh login to confirm the IdP roundtrip works.
- Tail `docker compose logs -f vault` for ~5 minutes — most regressions surface immediately.

If you spotted something the docs didn't predict, open a PR adding it to RUNBOOK.md or this file.
