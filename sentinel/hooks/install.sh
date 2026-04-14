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
mkdir -p "$HOOKS_DIR"

# Install pre-commit hook
cp "$(dirname "$0")/pre-commit" "$HOOKS_DIR/pre-commit"
chmod +x "$HOOKS_DIR/pre-commit"

echo "Korva Sentinel hooks installed in $HOOKS_DIR"
echo "Run 'korva sentinel check' to validate without committing"
