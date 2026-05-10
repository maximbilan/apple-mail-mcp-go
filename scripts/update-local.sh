#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BIN_NAME="apple-mail-mcp"
DEST_DIR="${APPLE_MAIL_MCP_LOCAL_BIN_DIR:-$HOME/.local/bin}"
DEST_PATH="$DEST_DIR/$BIN_NAME"
CACHE_BASE="${APPLE_MAIL_MCP_CACHE_DIR:-$ROOT_DIR/.tmp-go-cache}"
GOCACHE_DIR="$CACHE_BASE/gocache"
GOMODCACHE_DIR="$CACHE_BASE/gomodcache"
GOPATH_DIR="$CACHE_BASE/gopath"

cd "$ROOT_DIR"
mkdir -p "$GOCACHE_DIR" "$GOMODCACHE_DIR" "$GOPATH_DIR"
GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOPATH="$GOPATH_DIR" go build -o "$BIN_NAME" ./cmd/apple-mail-mcp
mkdir -p "$DEST_DIR"
cp "$BIN_NAME" "$DEST_PATH"
chmod +x "$DEST_PATH"

echo "Updated $DEST_PATH"
