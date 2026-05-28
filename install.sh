#!/bin/sh
# ackoctl installer — macOS and Linux, single one-liner.
#
#   curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
#
# Options (set in the environment before the curl pipe, or pass the version
# as the first positional argument when running the script directly):
#
#   ACKOCTL_VERSION   pin a specific release, e.g. v0.1.0 (default: latest)
#   BIN_DIR           target directory (default: /usr/local/bin; falls back to
#                     $HOME/.local/bin when not writable)
#
# Detected matrix:        darwin/linux × amd64/arm64

set -eu

REPO="aerospike-ce-ecosystem/ackoctl"
BIN_NAME="ackoctl"
DEFAULT_BIN_DIR="/usr/local/bin"

VERSION="${ACKOCTL_VERSION:-${1:-}}"
BIN_DIR="${BIN_DIR:-$DEFAULT_BIN_DIR}"

log()  { printf '\033[1;34m▸\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m✓\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }

# $HOME is referenced for the $HOME/.local/bin fallback install dir. Guard
# against an unset HOME up-front so the error message is clear instead of an
# opaque "/.local/bin: Permission denied" later.
[ -n "${HOME:-}" ] || err "HOME is not set; cannot determine fallback install directory"

require() {
  for cmd in "$@"; do
    command -v "$cmd" >/dev/null 2>&1 || err "missing required command: $cmd"
  done
}

require curl tar uname mktemp

# --- OS / Arch detection -----------------------------------------------------

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux"  ;;
  *) err "unsupported OS: $(uname -s) (need Darwin or Linux)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64)   ARCH="amd64" ;;
  arm64|aarch64)  ARCH="arm64" ;;
  *) err "unsupported arch: $(uname -m) (need amd64 or arm64)" ;;
esac

# --- Version resolution ------------------------------------------------------

if [ -z "$VERSION" ]; then
  log "resolving latest release"
  REDIRECT=$(curl -fsSLI --retry 3 --retry-delay 2 --retry-connrefused \
    -o /dev/null -w '%{url_effective}' \
    "https://github.com/$REPO/releases/latest") \
    || err "could not reach https://github.com/$REPO/releases/latest"
  TAG="${REDIRECT##*/}"
  case "$TAG" in
    v*) VERSION="$TAG" ;;
    *) err "could not parse latest tag from '$REDIRECT'" ;;
  esac
fi

case "$VERSION" in
  v*) ;;
  *)  VERSION="v$VERSION" ;;
esac
VERSION_NOV="${VERSION#v}"

log "ackoctl $VERSION ($OS/$ARCH) → $BIN_DIR/$BIN_NAME"

# --- Download ----------------------------------------------------------------

ARCHIVE="${BIN_NAME}_${VERSION_NOV}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"
CHECKSUM_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t ackoctl)
trap 'rm -rf "$TMP"' EXIT

log "downloading $URL"
curl -fsSL --retry 3 --retry-delay 2 --retry-connrefused \
  -o "$TMP/$ARCHIVE" "$URL" \
  || err "download failed; check that release $VERSION has the asset $ARCHIVE"

# --- Checksum verification ---------------------------------------------------
#
# Every failure path here is fatal. This is a curl-piped installer, so the
# threat model is exactly a tampered download — falling back to an unverified
# binary would defeat the guarantee the checksum exists to provide. goreleaser
# publishes checksums.txt for every release, so a missing file/entry means a
# network problem, a MITM, or a malformed release — never a normal install.

curl -fsSL --retry 3 --retry-delay 2 --retry-connrefused \
  -o "$TMP/checksums.txt" "$CHECKSUM_URL" \
  || err "could not download $CHECKSUM_URL — refusing to install an unverified binary"

log "verifying checksum"
# Match the entry by exact field equality on column 2 (the filename), not by
# regex on a single-space prefix. checksums.txt is whitespace-separated and
# the historical "grep ' $ARCHIVE$'" form was brittle to format drift (tabs,
# double spaces) and treated $ARCHIVE as a regex — a future archive name
# containing a regex metacharacter could match the wrong line. awk avoids
# both problems and exits on the first hit.
EXPECTED=$(awk -v f="$ARCHIVE" '$2==f {print $1; exit}' "$TMP/checksums.txt")
[ -n "$EXPECTED" ] || err "no checksum entry for $ARCHIVE in checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMP/$ARCHIVE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "$TMP/$ARCHIVE" | awk '{print $1}')
else
  err "neither sha256sum nor shasum is available — cannot verify the download"
fi

[ "$ACTUAL" = "$EXPECTED" ] || err "checksum mismatch ($ACTUAL != $EXPECTED)"
ok "checksum verified"

# --- Extract & install -------------------------------------------------------

tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
[ -f "$TMP/$BIN_NAME" ] || err "archive did not contain $BIN_NAME"
chmod +x "$TMP/$BIN_NAME"

install_binary() {
  install -m 0755 "$TMP/$BIN_NAME" "$1/$BIN_NAME"
}

if [ -w "$BIN_DIR" ] 2>/dev/null || mkdir -p "$BIN_DIR" 2>/dev/null && [ -w "$BIN_DIR" ]; then
  install_binary "$BIN_DIR"
elif command -v sudo >/dev/null 2>&1 && [ "$BIN_DIR" = "$DEFAULT_BIN_DIR" ]; then
  log "sudo required to write to $BIN_DIR"
  sudo install -m 0755 "$TMP/$BIN_NAME" "$BIN_DIR/$BIN_NAME"
else
  FALLBACK="$HOME/.local/bin"
  warn "$BIN_DIR not writable; installing to $FALLBACK instead"
  mkdir -p "$FALLBACK"
  install_binary "$FALLBACK"
  BIN_DIR="$FALLBACK"
fi

# --- Done --------------------------------------------------------------------

ok "installed $BIN_DIR/$BIN_NAME"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) warn "$BIN_DIR is not in your PATH — add it to your shell profile" ;;
esac

"$BIN_DIR/$BIN_NAME" version --short 2>/dev/null \
  && ok "run 'ackoctl --help' to get started" \
  || warn "binary installed but 'ackoctl version' failed — check $BIN_DIR/$BIN_NAME"
