#!/usr/bin/env bash
# Build xgit_patchd into the external patch directory (Git repo sibling)
set -euo pipefail

# 进入 apps/patch（本脚本所在目录）
cd "$(dirname "$0")"

# 目标输出：仓库同级 patch 目录（Git 外）
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd -P)"
PATCH_DIR="$(cd "$REPO_ROOT/.." && pwd -P)/patch"
OUT="${PATCH_DIR}/xgit_patchd"

mkdir -p "$PATCH_DIR"

echo "Building -> ${OUT}"

# 确保模块开启；本项目仅使用标准库，无额外依赖
GO111MODULE=on go mod tidy >/dev/null 2>&1 || true
GO111MODULE=on go build -trimpath -o "$OUT" .

chmod +x "$OUT"
echo "Build OK: $OUT"
