#!/usr/bin/env bash
# Install brand-image-generator from GitHub Releases (no Go required).
set -euo pipefail

REPO="${BIG_REPO:-GabrielRezendi/brand-image-generator}"
BINARY="brand-image-generator"
INSTALL_DIR="${BIG_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${BIG_VERSION:-latest}"

info() { printf '==> %s\n' "$*"; }
err() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "preciso de '$1' no PATH"
}

detect_os() {
  case "$(uname -s)" in
    Linux*) echo linux ;;
    Darwin*) echo darwin ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) err "OS não suportado: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) err "arch não suportada: $(uname -m)" ;;
  esac
}

download() {
  local url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
  else
    err "preciso de curl ou wget"
  fi
}

json_get_tag() {
  # Prefer jq; fall back to sed/grep for "tag_name".
  local body="$1"
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$body" | jq -r '.tag_name'
    return
  fi
  printf '%s' "$body" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
EXT="tar.gz"
[[ "$OS" == "windows" ]] && EXT="zip"

need tar
[[ "$EXT" == "zip" ]] && need unzip

API="https://api.github.com/repos/${REPO}/releases/latest"
if [[ "$VERSION" != "latest" ]]; then
  API="https://api.github.com/repos/${REPO}/releases/tags/${VERSION}"
fi

info "a obter release (${VERSION}) de ${REPO}…"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

download "$API" "$TMP/release.json"
TAG="$(json_get_tag "$(cat "$TMP/release.json")")"
[[ -n "$TAG" && "$TAG" != "null" ]] || err "não foi possível ler tag da release (existe alguma release pública?)"

ASSET="${BINARY}_${TAG}_${OS}_${ARCH}.${EXT}"
# GoReleaser uses Version without leading v sometimes — try both.
ASSET_ALT="${BINARY}_${TAG#v}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
URL_ALT="https://github.com/${REPO}/releases/download/${TAG}/${ASSET_ALT}"

info "a descarregar ${ASSET}…"
if ! download "$URL" "$TMP/pkg.${EXT}" 2>/dev/null; then
  info "tentativa alternativa ${ASSET_ALT}…"
  download "$URL_ALT" "$TMP/pkg.${EXT}" || err "asset não encontrado: ${ASSET} / ${ASSET_ALT}"
fi

mkdir -p "$TMP/extract"
if [[ "$EXT" == "zip" ]]; then
  unzip -q "$TMP/pkg.${EXT}" -d "$TMP/extract"
else
  tar -xzf "$TMP/pkg.${EXT}" -C "$TMP/extract"
fi

BIN_PATH="$(find "$TMP/extract" -type f -name "$BINARY" -o -name "${BINARY}.exe" | head -1)"
[[ -n "$BIN_PATH" ]] || err "binário não encontrado no arquivo"

mkdir -p "$INSTALL_DIR"
DEST="${INSTALL_DIR}/${BINARY}"
[[ "$OS" == "windows" ]] && DEST="${DEST}.exe"
install -m 755 "$BIN_PATH" "$DEST"

info "instalado em ${DEST} (${TAG})"

case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    printf '\nAviso: %s não está no PATH. Adiciona por exemplo:\n' "$INSTALL_DIR"
    printf '  export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
    ;;
esac

if ! command -v rsvg-convert >/dev/null 2>&1; then
  printf 'Aviso: rsvg-convert não encontrado.\n'
  printf '  Debian/Ubuntu: sudo apt install librsvg2-bin\n'
  printf '  macOS:         brew install librsvg\n'
fi

printf '\nCorre: %s\n' "$BINARY"
