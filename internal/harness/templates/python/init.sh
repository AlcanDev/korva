#!/usr/bin/env bash
# init.sh — environment sanity + test gate (Python stack).
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
echo "── 2. Python toolchain ────────────────────────────────"

PYTHON_BIN=""
for candidate in python3 python; do
  if command -v "$candidate" >/dev/null 2>&1; then
    PYTHON_BIN="$candidate"
    break
  fi
done

if [ -z "$PYTHON_BIN" ]; then
  fail "python not on PATH"
  EXIT_CODE=1
else
  PYVER=$("$PYTHON_BIN" --version 2>&1)
  ok "python available ($PYVER)"
fi

echo ""
echo "── 3. Virtualenv ──────────────────────────────────────"

if [ -d .venv ] || [ -d venv ]; then
  ok "virtualenv directory present"
else
  warn "no .venv / venv directory — relying on global interpreter"
fi

echo ""
echo "── 4. feature_list.json validation ────────────────────"

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
echo "── 5. Harness invariants (schema + SDD spec coverage) ─"

if command -v korva >/dev/null 2>&1; then
  if korva harness check; then
    ok "korva harness check passed"
  else
    fail "korva harness check reported issues"
    EXIT_CODE=1
  fi
else
  warn "korva CLI not on PATH — skipping invariant check"
fi

echo ""
echo "── 6. Test suite ──────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ] && [ -n "$PYTHON_BIN" ]; then
  if "$PYTHON_BIN" -m pytest --version >/dev/null 2>&1; then
    if "$PYTHON_BIN" -m pytest -q 2>&1; then
      ok "pytest passed"
    else
      fail "pytest failed"
      EXIT_CODE=1
    fi
  else
    warn "pytest not installed — install it or replace this block with"
    warn "your project's test command (unittest, nose, hypothesis, etc.)"
  fi
fi

echo ""
echo "── 7. Summary ─────────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ]; then
  ok "Harness ready. Proceed."
else
  fail "Harness not ready. Resolve the [FAIL] entries above."
fi

exit $EXIT_CODE
