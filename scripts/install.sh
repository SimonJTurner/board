#!/usr/bin/env sh
# One-line install: curl -fsSL https://raw.githubusercontent.com/SimonJTurner/board/main/scripts/install.sh | sh
# Downloads the latest board binary for your OS/arch and installs to a dir in PATH.

set -e

REPO="${BOARD_INSTALL_REPO:-SimonJTurner/board}"
BASE_URL="https://github.com/${REPO}/releases/latest/download"

# Detect OS (GOOS)
case "$(uname -s)" in
  Darwin)  GOOS=darwin ;;
  Linux)   GOOS=linux ;;
  MINGW*|MSYS*|CYGWIN*) GOOS=windows ;;
  *)
    echo "Unsupported OS: $(uname -s). Download a release from https://github.com/${REPO}/releases" >&2
    exit 1
    ;;
esac

# Detect arch (GOARCH)
case "$(uname -m)" in
  x86_64|amd64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *)
    echo "Unsupported arch: $(uname -m). Download a release from https://github.com/${REPO}/releases" >&2
    exit 1
    ;;
esac

FILENAME="board-${GOOS}-${GOARCH}"
[ "$GOOS" = "windows" ] && FILENAME="${FILENAME}.exe"

# Install dir: prefer BOARD_BIN, then /usr/local/bin, then ~/.local/bin, then ~/bin
if [ -n "${BOARD_BIN:-}" ]; then
  BIN_DIR="$BOARD_BIN"
else
  if [ -w /usr/local/bin ] 2>/dev/null; then
    BIN_DIR=/usr/local/bin
  elif [ -w "$HOME/.local/bin" ] 2>/dev/null || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    BIN_DIR="$HOME/.local/bin"
  else
    BIN_DIR="$HOME/bin"
    mkdir -p "$BIN_DIR" 2>/dev/null || true
  fi
fi

if [ -n "${BOARD_INSTALL_DRY_RUN:-}" ]; then
  echo "DRY RUN: would download ${BASE_URL}/${FILENAME}"
  echo "DRY RUN: would install to ${BIN_DIR}/board$([ "$GOOS" = "windows" ] && echo .exe)"
  exit 0
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading board ${GOOS}/${GOARCH} to ${BIN_DIR}..."
if ! curl -fsSL "${BASE_URL}/${FILENAME}" -o "${TMP}/board"; then
  echo "Download failed. Check https://github.com/${REPO}/releases for available builds." >&2
  exit 1
fi

chmod +x "${TMP}/board"
if [ "$GOOS" = "windows" ]; then
  mv "${TMP}/board" "${BIN_DIR}/board.exe"
  echo "Installed to ${BIN_DIR}/board.exe"
else
  mv "${TMP}/board" "${BIN_DIR}/board"
  echo "Installed to ${BIN_DIR}/board"
fi

if ! command -v board >/dev/null 2>&1; then
  echo "Ensure ${BIN_DIR} is in your PATH."
fi
