#!/bin/sh
set -eu

REPO="maximbilan/apple-mail-mcp-go"
BIN_NAME="apple-mail-mcp"
INSTALL_DIR="$HOME/.local/bin"
TARGET="$INSTALL_DIR/$BIN_NAME"

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

confirm_default_yes() {
  prompt="$1"
  if [ -r /dev/tty ]; then
    printf '%s' "$prompt" > /dev/tty
    IFS= read -r input < /dev/tty || true
  elif [ -t 0 ]; then
    printf '%s' "$prompt"
    IFS= read -r input || true
  else
    return 0
  fi
  case "$(printf '%s' "$input" | tr '[:upper:]' '[:lower:]')" in
    ""|y|yes) return 0 ;;
    *) return 1 ;;
  esac
}

ensure_macos() {
  os=$(uname -s)
  [ "$os" = "Darwin" ] || fail "This installer only supports macOS."
}

build_with_go() {
  tmpdir="$1"
  if [ -f "./go.mod" ] && [ -f "./cmd/apple-mail-mcp/main.go" ]; then
    log "Building from local source with go build..."
    go build -o "$tmpdir/$BIN_NAME" ./cmd/apple-mail-mcp
  else
    log "Installing latest release via go install..."
    GOBIN="$tmpdir" go install "github.com/$REPO/cmd/apple-mail-mcp@latest"
  fi
}

download_release_binary() {
  tmpdir="$1"
  arch=$(uname -m)
  case "$arch" in
    arm64|aarch64) asset="$BIN_NAME-darwin-arm64" ;;
    x86_64|amd64) asset="$BIN_NAME-darwin-amd64" ;;
    *) fail "Unsupported architecture: $arch" ;;
  esac

  base="https://github.com/$REPO/releases/latest/download"
  log "Downloading prebuilt binary: $asset"
  curl -fsSL "$base/$asset" -o "$tmpdir/$asset" || fail "Failed to download $asset"
  curl -fsSL "$base/SHA256SUMS" -o "$tmpdir/SHA256SUMS" || fail "Failed to download SHA256SUMS"

  expected=$(grep "  $asset$" "$tmpdir/SHA256SUMS" | awk '{print $1}')
  [ -n "$expected" ] || fail "Checksum entry for $asset not found"
  actual=$(shasum -a 256 "$tmpdir/$asset" | awk '{print $1}')
  [ "$expected" = "$actual" ] || fail "Checksum mismatch for $asset"

  mv "$tmpdir/$asset" "$tmpdir/$BIN_NAME"
  chmod +x "$tmpdir/$BIN_NAME"
}

install_binary() {
  tmpdir="$1"
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$tmpdir/$BIN_NAME" "$TARGET"
  log "Installed $TARGET"
}

run_registration() {
  if [ -t 0 ] || [ -r /dev/tty ]; then
    "$TARGET" install
  else
    "$TARGET" install --yes
  fi
}

main() {
  ensure_macos
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT INT TERM

  if command -v go >/dev/null 2>&1; then
    build_with_go "$tmpdir"
  else
    download_release_binary "$tmpdir"
  fi

  install_binary "$tmpdir"

  if confirm_default_yes "Register apple-mail MCP server with detected Claude clients? [Y/n]: "; then
    run_registration
  else
    log "Skipping automatic registration."
  fi

  log "Install complete."
}

main "$@"
