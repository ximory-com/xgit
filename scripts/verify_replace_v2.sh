#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

echo "== Repo =="
pwd
echo

# 准备样例
mkdir -p tests/data
cat > tests/data/r_v2_cn.txt <<'TXT'
标题：用　务包装　整个 应用 过程
下一行：保持不变
TXT

# 期望：用 contains_line + ignore_spaces 命中第一行，把“用……过程”替换为“已处理”
PATCH=$(cat <<'PATCH_EOF'
commitmsg: test(replace v2): contains_line + ignore_spaces demo (CN)
author: XGit Bot <bot@xgit.local>
repo: xgit

