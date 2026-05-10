#!/bin/sh
set -eu

BIN_NAME="apple-mail-mcp"
INSTALL_DIR="$HOME/.local/bin"
TARGET="$INSTALL_DIR/$BIN_NAME"
CLAUDE_CONFIG="$HOME/Library/Application Support/Claude/claude_desktop_config.json"

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
  [ "$os" = "Darwin" ] || fail "This uninstaller only supports macOS."
}

ensure_jq() {
  if command -v jq >/dev/null 2>&1; then
    return 0
  fi
  if ! confirm_default_yes "jq is required to edit Claude Desktop config. Install jq with Homebrew now? [Y/n]: "; then
    fail "jq is required but not installed"
  fi
  command -v brew >/dev/null 2>&1 || fail "Homebrew not found; install jq manually and retry"
  brew install jq
  command -v jq >/dev/null 2>&1 || fail "jq installation did not succeed"
}

remove_desktop_entry_fallback() {
  [ -f "$CLAUDE_CONFIG" ] || return 0
  ensure_jq

  ts=$(date +%Y%m%d-%H%M%S)
  cp "$CLAUDE_CONFIG" "$CLAUDE_CONFIG.bak.$ts"

  tmp=$(mktemp)
  jq 'if .mcpServers then .mcpServers |= del(."apple-mail") else . end' "$CLAUDE_CONFIG" > "$tmp"
  mv "$tmp" "$CLAUDE_CONFIG"
}

remove_claude_code_entry_fallback() {
  if command -v claude >/dev/null 2>&1; then
    claude mcp remove apple-mail || true
  fi
}

main() {
  ensure_macos

  if [ -x "$TARGET" ]; then
    if [ -t 0 ] || [ -r /dev/tty ]; then
      "$TARGET" uninstall
    else
      "$TARGET" uninstall --yes
    fi
  elif command -v "$BIN_NAME" >/dev/null 2>&1; then
    if [ -t 0 ] || [ -r /dev/tty ]; then
      "$BIN_NAME" uninstall
    else
      "$BIN_NAME" uninstall --yes
    fi
  else
    log "Installed binary not found; using fallback config cleanup."
    remove_desktop_entry_fallback
    remove_claude_code_entry_fallback
  fi

  rm -f "$TARGET"
  log "Removed $TARGET"
  log "Uninstall complete."
}

main "$@"
