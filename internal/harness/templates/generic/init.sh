#!/usr/bin/env bash
# init.sh — environment sanity + test gate.
# Run this:
#   - At the START of every agent session
#   - Before declaring any feature `done`
# If this script fails, do NOT proceed.

set -u
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

ok()    { printf "${GREEN}[OK]${NC}    %s\n" "$1"; }
warn()  { printf "${YELLOW}[WARN]${NC}  %s\n" "$1"; }
fail()  { printf "${RED}[FAIL]${NC}  %s\n" "$1"; }

EXIT_CODE=0

echo "── 1. Harness files ───────────────────────────────────"

for f in AGENTS.md feature_list.json progress/current.md docs/architecture.md docs/conventions.md docs/verification.md CHECKPOINTS.md; do
  if [ ! -f "$f" ]; then
    fail "missing: $f"
    EXIT_CODE=1
  else
    ok "$f"
  fi
done

echo ""
echo "── 2. feature_list.json validation ────────────────────"

# Reuse the korva binary when available — it knows the schema cold.
if command -v korva >/dev/null 2>&1; then
  if korva harness status >/dev/null 2>&1; then
    ok "feature_list.json valid (via korva harness status)"
  else
    fail "korva harness status reported errors"
    EXIT_CODE=1
  fi
else
  warn "korva CLI not on PATH — skipping deep validation"
fi

echo ""
echo "── 3. Test suite ──────────────────────────────────────"

# The generic preset doesn't pre-pick a runner. Replace this block with
# your project's test command (make test, pytest, npm test, etc.) and
# return non-zero on failure.
warn "No test runner configured. Replace the block in init.sh with your"
warn "project's test command, e.g. 'make test' or 'pytest -q'."

echo ""
echo "── 4. Summary ─────────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ]; then
  ok "Harness ready. Proceed."
else
  fail "Harness not ready. Resolve the [FAIL] entries above."
fi

exit $EXIT_CODE
