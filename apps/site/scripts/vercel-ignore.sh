#!/usr/bin/env bash
set -euo pipefail

echo "[vercel-ignore] project=apps/site"

# 处理首次构建或浅克隆：没有 HEAD~1 时，直接允许构建
if ! git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
  echo "[vercel-ignore] no previous commit -> build"
  exit 1
fi

# 计算这次提交相对上一提交的变更文件（相对仓库根路径）
CHANGED="$(git diff --name-only HEAD~1 HEAD || true)"
echo "[vercel-ignore] changed files:"
echo "$CHANGED"

# apps/site 子目录有变更 -> 构建
if echo "$CHANGED" | grep -E '^apps/site/' >/dev/null 2>&1; then
  echo "[vercel-ignore] changes in apps/site -> build"
  exit 1
fi

echo "[vercel-ignore] no changes in apps/site -> skip build"
exit 0

