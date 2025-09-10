#!/usr/bin/env bash
set -euo pipefail

# XGIT:BEGIN BUILD_CONF
# 说明：将构建产物输出到仓库同级的 patch 目录（Git 仓库之外）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
PATCH_DIR="$(cd "$REPO_ROOT/.." && pwd -P)/patch"
OUT="$PATCH_DIR/xgit_patchd"
# XGIT:END BUILD_CONF

mkdir -p "$PATCH_DIR"
echo "Building -> $OUT"
cd "$REPO_ROOT/apps/patch"
go build -o "$OUT" ./...
echo "Done."
