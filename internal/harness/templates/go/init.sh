#!/usr/bin/env bash
# init.sh — environment sanity + test gate (Go stack).
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
echo "── 2. Go toolchain ────────────────────────────────────"

if ! command -v go >/dev/null 2>&1; then
  fail "go not on PATH"
  EXIT_CODE=1
else
  GOVER=$(go version | awk '{print $3}')
  ok "go available ($GOVER)"
fi

echo ""
echo "── 3. feature_list.json validation ────────────────────"

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
echo "── 4. Build ───────────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ]; then
  if go build ./... 2>&1; then
    ok "go build ./... clean"
  else
    fail "go build ./... failed"
    EXIT_CODE=1
  fi
fi

echo ""
echo "── 5. Test suite ──────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ]; then
  # -count=1 disables the test cache so a green run is a real run.
  if go test ./... -count=1 2>&1; then
    ok "go test ./... passed"
  else
    fail "go test ./... failed"
    EXIT_CODE=1
  fi
fi

echo ""
echo "── 6. Summary ─────────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ]; then
  ok "Harness ready. Proceed."
else
  fail "Harness not ready. Resolve the [FAIL] entries above."
fi

exit $EXIT_CODE
