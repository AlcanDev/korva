#!/bin/sh
# Korva Sentinel — hook installer
# Usage: ./sentinel/hooks/install.sh [git-repo-path]

REPO=${1:-.}
GIT_DIR="$REPO/.git"

if [ ! -d "$GIT_DIR" ]; then
    echo "Error: $REPO is not a git repository"
    exit 1
fi

HOOKS_DIR="$GIT_DIR/hooks"
SCRIPT_DIR="$(dirname "$0")"
mkdir -p "$HOOKS_DIR"

# Install pre-commit hook (architecture validation)
cp "$SCRIPT_DIR/pre-commit" "$HOOKS_DIR/pre-commit"
chmod +x "$HOOKS_DIR/pre-commit"
echo "  ✓ pre-commit  — architecture validation (Sentinel)"

# Install post-commit hook (vault auto-sync)
cp "$SCRIPT_DIR/post-commit" "$HOOKS_DIR/post-commit"
chmod +x "$HOOKS_DIR/post-commit"
echo "  ✓ post-commit — vault auto-sync"

echo ""
echo "Korva hooks installed in $HOOKS_DIR"
echo "Run 'korva sentinel check' to validate without committing"
echo "Run 'korva sync --vault' to sync manually"
