#!/usr/bin/env sh
# Korva — instalador one-line para macOS y Linux
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.sh | sh
#
# Variables opcionales (env):
#   KORVA_VERSION   — version especifica a instalar (default: latest)
#   KORVA_PREFIX    — prefijo de instalacion (default: /usr/local)

set -e

REPO="AlcanDev/korva"
PREFIX="${KORVA_PREFIX:-/usr/local}"
BIN_DIR="${PREFIX}/bin"

# ─── Colores ───────────────────────────────────────────────────────────────────
info()    { printf '\033[1;34m[korva]\033[0m %s\n' "$*"; }
success() { printf '\033[1;32m[korva]\033[0m ✓ %s\n' "$*"; }
warn()    { printf '\033[1;33m[korva]\033[0m ! %s\n' "$*"; }
error()   { printf '\033[1;31m[korva]\033[0m ✗ %s\n' "$*" >&2; exit 1; }

# ─── Banner ────────────────────────────────────────────────────────────────────
printf '\n\033[36m  ██╗  ██╗ ██████╗ ██████╗ ██╗   ██╗ █████╗ \n'
printf '  ██║ ██╔╝██╔═══██╗██╔══██╗██║   ██║██╔══██╗\n'
printf '  █████╔╝ ██║   ██║██████╔╝██║   ██║███████║\n'
printf '  ██╔═██╗ ██║   ██║██╔══██╗╚██╗ ██╔╝██╔══██║\n'
printf '  ██║  ██╗╚██████╔╝██║  ██║ ╚████╔╝ ██║  ██║\n'
printf '  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝  ╚═══╝  ╚═╝  ╚═╝\033[0m\n\n'

# ─── Verificar dependencias ────────────────────────────────────────────────────
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 \
  || error "Se requiere curl o wget."
command -v tar >/dev/null 2>&1 || error "Se requiere tar."

# ─── Detectar plataforma ───────────────────────────────────────────────────────
info "Detectando plataforma..."

UNAME_S="$(uname -s)"
UNAME_M="$(uname -m)"

# GoReleaser capitaliza el OS: Darwin, Linux
case "$UNAME_S" in
  Darwin) OS_NAME="Darwin"  ;;
  Linux)  OS_NAME="Linux"   ;;
  *)      error "Sistema operativo no soportado: $UNAME_S. En Windows usa: scripts/install.ps1" ;;
esac

case "$UNAME_M" in
  x86_64)       ARCH_NAME="amd64" ;;
  arm64|aarch64) ARCH_NAME="arm64" ;;
  *)            error "Arquitectura no soportada: $UNAME_M" ;;
esac

success "Plataforma: $OS_NAME/$ARCH_NAME"

# ─── Resolver versión ─────────────────────────────────────────────────────────
if [ -z "${KORVA_VERSION:-}" ]; then
  info "Obteniendo version mas reciente..."
  if command -v curl >/dev/null 2>&1; then
    KORVA_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"tag_name": *"v?([^"]+)".*/\1/')
  else
    KORVA_VERSION=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"tag_name": *"v?([^"]+)".*/\1/')
  fi
fi

[ -z "${KORVA_VERSION:-}" ] && error "No se pudo determinar la version. Define KORVA_VERSION=X.Y.Z"
success "Version: v$KORVA_VERSION"

# ─── Descargar ────────────────────────────────────────────────────────────────
info "Descargando korva v$KORVA_VERSION para $OS_NAME/$ARCH_NAME..."

ARCHIVE="korva_${KORVA_VERSION}_${OS_NAME}_${ARCH_NAME}.tar.gz"
URL="https://github.com/$REPO/releases/download/v${KORVA_VERSION}/$ARCHIVE"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$TMP_DIR/$ARCHIVE" || error "Error descargando $URL"
else
  wget -qO "$TMP_DIR/$ARCHIVE" "$URL" || error "Error descargando $URL"
fi

tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"
success "Descarga y extraccion completadas"

# ─── Instalar binarios ────────────────────────────────────────────────────────
info "Instalando en $BIN_DIR..."

# Determinar si se necesita sudo
if [ -w "$BIN_DIR" ] || mkdir -p "$BIN_DIR" 2>/dev/null; then
  SUDO=""
else
  warn "Se requieren permisos de administrador para instalar en $BIN_DIR"
  SUDO="sudo"
  $SUDO mkdir -p "$BIN_DIR"
fi

for bin in korva korva-vault korva-sentinel; do
  src="$TMP_DIR/$bin"
  if [ -f "$src" ]; then
    $SUDO install -m 755 "$src" "$BIN_DIR/$bin"
    success "Instalado: $BIN_DIR/$bin"
  else
    warn "No encontrado en el archivo: $bin (puede ser normal en versiones futuras)"
  fi
done

# ─── Verificar instalacion ────────────────────────────────────────────────────
printf '\n'
if command -v korva >/dev/null 2>&1; then
  VER="$(korva version 2>/dev/null || echo "v$KORVA_VERSION")"
  success "Korva $VER instalado y listo"
else
  warn "korva no encontrado en PATH. Puede que necesites reabrir tu terminal."
  warn "O agrega $BIN_DIR a tu PATH:"
  printf '  \033[33mexport PATH="$PATH:%s"\033[0m\n' "$BIN_DIR"
  printf '  # Agregar permanentemente (zsh):\n'
  printf '  \033[33mecho '"'"'export PATH="$PATH:%s"'"'"' >> ~/.zshrc && source ~/.zshrc\033[0m\n' "$BIN_DIR"
fi

printf '\n\033[1;32m¡Instalacion completada!\033[0m\n'
printf '\nSiguientes pasos:\n'
printf '  \033[36m1.\033[0m korva init\n'
printf '  \033[36m2.\033[0m korva init --profile <url-profile-privado>  # para equipos\n'
printf '  \033[36m3.\033[0m cd ~/repos/mi-proyecto && korva sentinel install\n'
printf '\nDocumentacion: \033[37mhttps://github.com/AlcanDev/korva\033[0m\n\n'
