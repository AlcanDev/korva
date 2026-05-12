#!/usr/bin/env bash
# Smoke test for the Observatory backend.
#
# Verifies the new endpoints return reasonable data against a running
# `korva-vault` instance. Run it after a fresh build to confirm the
# whole flow works end-to-end without opening Beacon.
#
# Usage:
#   scripts/smoke-observatory.sh                # uses ~/.korva/admin.key
#   KORVA_ADMIN_KEY=<hex> scripts/smoke-observatory.sh
#   PORT=7437 HOST=127.0.0.1 scripts/smoke-observatory.sh

set -euo pipefail

HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-7437}"
BASE="http://${HOST}:${PORT}"

if [[ -z "${KORVA_ADMIN_KEY:-}" ]]; then
    if [[ -f "${HOME}/.korva/admin.key" ]]; then
        KORVA_ADMIN_KEY="$(jq -r '.key' "${HOME}/.korva/admin.key" 2>/dev/null || true)"
    fi
fi

if [[ -z "${KORVA_ADMIN_KEY:-}" ]]; then
    echo "❌ Could not resolve admin key. Set KORVA_ADMIN_KEY or run \`korva init --admin\`."
    exit 1
fi

ok() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1" && exit 1; }

echo "→ Vault healthcheck"
curl -fsS "${BASE}/healthz" >/dev/null && ok "GET /healthz" || fail "vault not responding on ${BASE}"

# ── 1. Public ingest ────────────────────────────────────────────────────────
echo "→ POST /api/v1/interactions"
INGEST_BODY='{
  "project": "smoke",
  "agent": "claude",
  "model": "claude-opus-4-7",
  "prompt": "smoke test prompt — observatory check",
  "response": "ok",
  "usage": {
    "input_tokens": 1000,
    "output_tokens": 200,
    "cache_read_input_tokens": 500,
    "cache_creation_input_tokens": 100
  },
  "duration_ms": 1234
}'
RESP="$(curl -fsS -X POST -H "Content-Type: application/json" -d "${INGEST_BODY}" "${BASE}/api/v1/interactions")"
INTERACTION_ID="$(echo "${RESP}" | jq -r '.id')"
[[ -n "${INTERACTION_ID}" ]] || fail "no id returned"
ok "interaction id ${INTERACTION_ID}"

# Estimated fallback when usage is omitted.
RESP="$(curl -fsS -X POST -H "Content-Type: application/json" \
    -d '{"project":"smoke","agent":"claude","prompt":"no usage reported"}' \
    "${BASE}/api/v1/interactions")"
[[ "$(echo "${RESP}" | jq -r '.estimated')" == "true" ]] && ok "fallback marks estimated=true" || fail "estimated flag missing"

# ── 2. System status ────────────────────────────────────────────────────────
echo "→ GET /admin/system-status"
RESP="$(curl -fsS -H "X-Admin-Key: ${KORVA_ADMIN_KEY}" "${BASE}/admin/system-status")"
echo "${RESP}" | jq -e '.vault.running == true' >/dev/null && ok "vault.running=true"
echo "${RESP}" | jq -e '.ide | type == "array"' >/dev/null && ok "ide is array"
echo "${RESP}" | jq -e '.observations.total | type == "number"' >/dev/null && ok "observations counted"

# ── 3. Token analytics ──────────────────────────────────────────────────────
echo "→ GET /admin/tokens/stats"
RESP="$(curl -fsS -H "X-Admin-Key: ${KORVA_ADMIN_KEY}" "${BASE}/admin/tokens/stats")"
echo "${RESP}" | jq -e '.totals.input_tokens >= 1000' >/dev/null && ok "input_tokens recorded"
echo "${RESP}" | jq -e '.cache_hit_pct > 0' >/dev/null && ok "cache_hit_pct > 0"
echo "${RESP}" | jq -e '.baseline_naive_tokens > 0' >/dev/null && ok "baseline computed"

# ── 4. Activity timeline ────────────────────────────────────────────────────
echo "→ GET /admin/activity"
RESP="$(curl -fsS -H "X-Admin-Key: ${KORVA_ADMIN_KEY}" "${BASE}/admin/activity?project=smoke&limit=5")"
echo "${RESP}" | jq -e '.interactions | length > 0' >/dev/null && ok "smoke project visible"

echo "→ GET /admin/activity/{id}"
curl -fsS -H "X-Admin-Key: ${KORVA_ADMIN_KEY}" "${BASE}/admin/activity/${INTERACTION_ID}" | jq -e '.id' >/dev/null && ok "single interaction fetched"

# ── 5. Config editor ────────────────────────────────────────────────────────
echo "→ GET /admin/config"
RESP="$(curl -fsS -H "X-Admin-Key: ${KORVA_ADMIN_KEY}" "${BASE}/admin/config?scope=local")"
HASH="$(echo "${RESP}" | jq -r '.hash')"
[[ -n "${HASH}" ]] && ok "config hash present" || fail "no config hash"

# ── 6. Sentinel rules editor ────────────────────────────────────────────────
echo "→ GET /admin/sentinel/rules"
RESP="$(curl -fsS -H "X-Admin-Key: ${KORVA_ADMIN_KEY}" "${BASE}/admin/sentinel/rules")"
echo "${RESP}" | jq -e '.builtin | length == 10' >/dev/null && ok "10 built-in rules"

echo "→ POST /admin/sentinel/test"
TEST_BODY='{
  "rule": {
    "id": "SMOKE-001",
    "pattern": "console\\.log",
    "severity": "error",
    "paths_include": ["src/**/*.ts"],
    "message": "no console.log"
  },
  "code": "const x = 1;\nconsole.log(\"hi\")\nconst y = 2;",
  "file_path": "src/app.ts"
}'
RESP="$(curl -fsS -X POST -H "Content-Type: application/json" -H "X-Admin-Key: ${KORVA_ADMIN_KEY}" -d "${TEST_BODY}" "${BASE}/admin/sentinel/test")"
echo "${RESP}" | jq -e '.matches | length == 1' >/dev/null && ok "test playground matches once"

echo
echo "✅ Observatory smoke test passed."
