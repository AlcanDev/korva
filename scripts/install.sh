#!/usr/bin/env bash
# Korva — one-line installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/alcandev/korva/main/scripts/install.sh | bash
#
# Options (env vars):
#   KORVA_VERSION   — specific version to install (default: latest)
#   KORVA_PREFIX    — install prefix (default: /usr/local)
#   KORVA_NO_VAULT  — set to 1 to skip installing korva-vault

set -euo pipefail

REPO="alcandev/korva"
PREFIX="${KORVA_PREFIX:-/usr/local}"
BIN_DIR="${PREFIX}/bin"

# --- helpers ---

info()    { printf '\033[1;34m[korva]\033[0m %s\n' "$*"; }
success() { printf '\033[1;32m[korva]\033[0m %s\n' "$*"; }
error()   { printf '\033[1;31m[korva]\033[0m %s\n' "$*" >&2; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || error "Required: $1 is not installed."
}

# --- detect platform ---

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) error "Unsupported OS: $OS. On Windows use: winget install AlcanDev.Korva" ;;
esac

# --- resolve version ---

require curl
require tar

if [ -z "${KORVA_VERSION:-}" ]; then
  info "Resolving latest release…"
  KORVA_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"v?([^"]+)".*/\1/')
fi

if [ -z "$KORVA_VERSION" ]; then
  error "Could not determine latest version. Set KORVA_VERSION explicitly."
fi

info "Installing Korva v${KORVA_VERSION} (${OS}/${ARCH})…"

# --- download ---

ARCHIVE="korva_${KORVA_VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${KORVA_VERSION}/${ARCHIVE}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

info "Downloading ${ARCHIVE}…"
curl -fsSL "$URL" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

# --- install binaries ---

install_bin() {
  local name="$1"
  local src="${TMP}/${name}"
  if [ ! -f "$src" ]; then
    info "  (${name} not found in archive, skipping)"
    return
  fi
  install -d "$BIN_DIR"
  install -m 755 "$src" "${BIN_DIR}/${name}"
  info "  ✓ ${BIN_DIR}/${name}"
}

install_bin "korva"
install_bin "korva-sentinel"

if [ "${KORVA_NO_VAULT:-0}" != "1" ]; then
  install_bin "korva-vault"
fi

# --- verify ---

if ! command -v korva >/dev/null 2>&1; then
  info ""
  info "NOTE: ${BIN_DIR} is not in your PATH."
  info "Add this to your shell profile:"
  info "  export PATH=\"${BIN_DIR}:\$PATH\""
fi

success "Korva v${KORVA_VERSION} installed."
success "Run: korva init"
