#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd -P)"
BIN_DIR="$ROOT/bin"
OUT="$BIN_DIR/xgit_patchd"

mkdir -p "$BIN_DIR"
echo "Building -> $OUT"
cd "$ROOT"
GO111MODULE=off go build -o "$OUT" xgit_patchd.go
echo "OK: $OUT"
