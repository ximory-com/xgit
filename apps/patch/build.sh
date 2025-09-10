#!/bin/bash
set -e

# 仓库根目录
REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd -P)"
# 仓库外的 patch 目录
PATCH_DIR="$(cd "$REPO_DIR/../patch" && pwd -P)"

echo "Building -> $PATCH_DIR/xgit_patchd"
go build -o "$PATCH_DIR/xgit_patchd" "$REPO_DIR/apps/patch/xgit_patchd.go"

