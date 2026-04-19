#!/usr/bin/env bash
# Korva installer — https://korva.dev/install
#
# Usage:
#   curl -fsSL https://korva.dev/install | bash
#
# Options (env vars):
#   KORVA_VERSION    Pin a specific version, e.g. v0.3.0  (default: latest)
#   KORVA_INSTALL_DIR  Override install directory           (default: /usr/local/bin or ~/.local/bin)
#   KORVA_NO_VAULT   Skip installing korva-vault            (default: install it)
#
set -euo pipefail

REPO="alcandev/korva"
VERSION="${KORVA_VERSION:-latest}"
INSTALL_VAULT="${KORVA_NO_VAULT:-yes}"

# ─── Detect OS ────────────────────────────────────────────────────────────────

OS="$(uname -s)"
case "$OS" in
  Linux)   OS="linux"   ;;
  Darwin)  OS="darwin"  ;;
  *)
    echo "❌ Unsupported operating system: $OS"
    echo "   Windows users: download from https://github.com/${REPO}/releases"
    exit 1
    ;;
esac

# ─── Detect architecture ─────────────────────────────────────────────────────

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# ─── Resolve version ─────────────────────────────────────────────────────────

if [ "$VERSION" = "latest" ]; then
  echo "  → Fetching latest release…"
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
  if [ -z "$VERSION" ]; then
    echo "❌ Could not determine latest version. Set KORVA_VERSION explicitly."
    exit 1
  fi
fi

VERSION_CLEAN="${VERSION#v}"
ARCHIVE="korva_${VERSION_CLEAN}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

# ─── Resolve install directory ───────────────────────────────────────────────

if [ -n "${KORVA_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$KORVA_INSTALL_DIR"
elif [ -w "/usr/local/bin" ] || [ "$(id -u)" = "0" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# ─── Download & extract ──────────────────────────────────────────────────────

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "  → Downloading Korva ${VERSION} (${OS}/${ARCH})…"
curl -fsSL --progress-bar "$URL" | tar xz -C "$TMP"

# ─── Install binaries ────────────────────────────────────────────────────────

INSTALLED=()

for bin in korva korva-sentinel; do
  if [ -f "$TMP/$bin" ]; then
    install -m 755 "$TMP/$bin" "$INSTALL_DIR/$bin"
    INSTALLED+=("$bin")
  fi
done

if [ "$INSTALL_VAULT" != "no" ] && [ -f "$TMP/korva-vault" ]; then
  install -m 755 "$TMP/korva-vault" "$INSTALL_DIR/korva-vault"
  INSTALLED+=("korva-vault")
fi

# ─── Path check ──────────────────────────────────────────────────────────────

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "  ⚠  $INSTALL_DIR is not in your PATH."
    echo "     Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo "       export PATH=\"\$PATH:$INSTALL_DIR\""
    echo ""
    ;;
esac

# ─── Done ────────────────────────────────────────────────────────────────────

echo ""
echo "  ✓ Korva ${VERSION} installed to ${INSTALL_DIR}"
echo "    Binaries: ${INSTALLED[*]}"
echo ""
echo "  Quick start:"
echo "    korva init               # initialise project"
echo "    korva vault start        # start the local vault server"
echo "    korva doctor             # verify your setup"
echo ""
echo "  Docs: https://korva.dev/docs"
