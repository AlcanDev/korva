#!/usr/bin/env bash
# init.sh — environment sanity + test gate (TypeScript stack).
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
echo "── 2. Node toolchain ──────────────────────────────────"

if ! command -v node >/dev/null 2>&1; then
  fail "node not on PATH"
  EXIT_CODE=1
else
  NODEVER=$(node --version)
  ok "node available ($NODEVER)"
fi

if ! command -v npm >/dev/null 2>&1; then
  fail "npm not on PATH"
  EXIT_CODE=1
else
  ok "npm available"
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
echo "── 4. Harness invariants (schema + SDD spec coverage) ─"

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
echo "── 5. Dependencies installed ──────────────────────────"

if [ -f package.json ]; then
  if [ ! -d node_modules ]; then
    warn "node_modules missing — running npm ci"
    if npm ci 2>&1 | tail -5; then
      ok "npm ci complete"
    else
      fail "npm ci failed"
      EXIT_CODE=1
    fi
  else
    ok "node_modules present"
  fi
else
  warn "no package.json — skipping install step"
fi

echo ""
echo "── 6. Type-check + build ──────────────────────────────"

if [ $EXIT_CODE -eq 0 ] && [ -f package.json ]; then
  # Many projects do tsc -b + vite/esbuild via `npm run build`. If a
  # `typecheck` script exists, prefer it (cheaper).
  if npm run --silent typecheck >/dev/null 2>&1; then
    if npm run typecheck 2>&1; then
      ok "typecheck passed"
    else
      fail "typecheck failed"
      EXIT_CODE=1
    fi
  elif npm run --silent build >/dev/null 2>&1; then
    if npm run build 2>&1 | tail -10; then
      ok "build passed"
    else
      fail "build failed"
      EXIT_CODE=1
    fi
  else
    warn "no typecheck or build script defined — skipping"
  fi
fi

echo ""
echo "── 7. Test suite ──────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ] && [ -f package.json ]; then
  if npm run --silent test >/dev/null 2>&1; then
    if npm test 2>&1; then
      ok "npm test passed"
    else
      fail "npm test failed"
      EXIT_CODE=1
    fi
  else
    warn "no test script defined — add one to package.json"
  fi
fi

echo ""
echo "── 8. Summary ─────────────────────────────────────────"

if [ $EXIT_CODE -eq 0 ]; then
  ok "Harness ready. Proceed."
else
  fail "Harness not ready. Resolve the [FAIL] entries above."
fi

exit $EXIT_CODE
