#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$root"



echo "== Repo =="; pwd; echo
echo "== ls tests/data =="; ls -l tests/data | sed -n '1,200p'; echo

# 1) p2_all_count0: 5 个独立 X
