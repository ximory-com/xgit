#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd -P)"
OUT="$ROOT/bin/xgit_patchd"

GO_BIN="${GO_BIN:-$(command -v go || true)}"
if [[ -z "${GO_BIN:-}" ]]; then
  for p in /usr/local/go/bin/go /opt/homebrew/opt/go/libexec/bin/go /opt/homebrew/bin/go; do
    [[ -x "$p" ]] && GO_BIN="$p" && break
  done
fi

if [[ -z "${GO_BIN:-}" ]]; then
  echo "❌ 未找到 go，请安装 Go 或设置 GO_BIN 指向 go 可执行文件。"
  echo "   例如：export GO_BIN=/usr/local/go/bin/go"
  exit 1
fi

echo "Building -> $OUT"
mkdir -p "$(dirname "$OUT")"
"$GO_BIN" build -o "$OUT" "$ROOT/xgit_patchd.go"
echo "✅ OK: $OUT"
