#!/usr/bin/env bash
# 兼容 macOS 的 BSD grep：不用 \b 词边界，统一用 -w
set -uo pipefail

root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$root"

pass() { printf "✔ %s\n" "$*"; }
fail() { printf "✘ %s\n" "$*" >&2; exit 1; }

# 容错计数：即使 0 次匹配也不会因 pipefail 退出
safe_count() { local word="$1" file="$2"; grep -Eow "$word" "$file" 2>/dev/null | wc -l | tr -d ' ' || true; }
# 读取某行并去掉末尾的 \r（用于 CRLF 文件内容断言）
line_n() { local n="$1" file="$2"; sed -n "${n}p" "$file" | sed 's/\r$//'; }

echo "== Repo =="; pwd; echo
echo "== ls tests/data =="; ls -l tests/data | sed -n '1,200p'; echo

# 1) p2_all_count0: 5 个独立 X
n=$(safe_count 'X' tests/data/p2_all_count0.txt)
[[ "$n" == "5" ]] && pass "p2_all_count0: X count == 5" || fail "p2_all_count0: expected 5 X, got $n"

# 2) p2_single_line: 第 2 行 'L2: HIT'
l2="$(line_n 2 tests/data/p2_single_line.txt)"
[[ "$l2" =~ ^L2:\ *HIT$ ]] && pass "p2_single_line: L2 is HIT" || fail "p2_single_line: got '$l2'"

# 3) p2_inverted_range: alpha/beta/gamma
grep -Eq '^alpha$' tests/data/p2_inverted_range.txt || fail "p2_inverted_range: missing alpha"
grep -Eq '^beta$'  tests/data/p2_inverted_range.txt || fail "p2_inverted_range: missing beta"
grep -Eq '^gamma$' tests/data/p2_inverted_range.txt || fail "p2_inverted_range: missing gamma"
pass "p2_inverted_range: alpha/beta/gamma present"

# 4) p2_unicode: 不应再有 foo/FOO；BAR >= 3
! grep -Eqw 'foo' tests/data/p2_unicode.txt || fail "p2_unicode: still has foo"
! grep -Eqw 'FOO' tests/data/p2_unicode.txt || fail "p2_unicode: still has FOO"
nbar=$(safe_count 'BAR' tests/data/p2_unicode.txt)
[[ ${nbar:-0} -ge 3 ]] && pass "p2_unicode: BAR count >= 3 (=$nbar)" || fail "p2_unicode: BAR too few ($nbar)"

# 5) p2_capture_move: key: value
grep -Eq '^first_name:\s*River$' tests/data/p2_capture_move.txt || fail "capture: first_name not normalized"
grep -Eq '^last_name:\s*Lee$'    tests/data/p2_capture_move.txt || fail "capture: last_name not normalized"
grep -Eq '^company:\s*XGit$'     tests/data/p2_capture_move.txt || fail "capture: company not normalized"
pass "p2_capture_move: normalized"

# 6) p2_anchor_range: L1 NEEDLE
l1="$(line_n 1 tests/data/p2_anchor_range.txt)"
[[ "$l1" =~ ^NEEDLE$ ]] && pass "p2_anchor_range: line1 NEEDLE" || fail "p2_anchor_range: line1 != NEEDLE"

# 7) p2_crlf_keep2: 内容 & CRLF
line_n 1 tests/data/p2_crlf_keep2.txt | grep -Eq '^one$'   || fail "crlf: missing 'one'"
line_n 2 tests/data/p2_crlf_keep2.txt | grep -Eq '^TWO$'   || fail "crlf: missing 'TWO'"
line_n 3 tests/data/p2_crlf_keep2.txt | grep -Eq '^three$' || fail "crlf: missing 'three'"
pass "crlf: content ok (one/TWO/three)"

#    再校验确为 CRLF（双保险）
if command -v file >/dev/null 2>&1; then
  if file tests/data/p2_crlf_keep2.txt | grep -qi 'CRLF'; then
    pass "crlf: CRLF detected (file)"
  else
    if od -An -t x1 tests/data/p2_crlf_keep2.txt | head -n1 | grep -q '0d 0a'; then
      pass "crlf: CRLF detected (od)"
    else
      fail "crlf: not CRLF"
    fi
  fi
else
  if od -An -t x1 tests/data/p2_crlf_keep2.txt | head -n1 | grep -q '0d 0a'; then
    pass "crlf: CRLF detected (od)"
  else
    fail "crlf: not CRLF"
  fi
fi

# 8) p2_oob_start: unchanged
[[ "$(cat tests/data/p2_oob_start.txt)" == "only this line" ]] && pass "oob: unchanged" || fail "oob: content changed"

# 9) p2_delete_by_empty_repl: no 'remove me'; KEEP >= 2
! grep -Eq '^remove me$' tests/data/p2_delete_by_empty_repl.txt || fail "delete: still has 'remove me'"
kcnt=$(grep -Eo 'KEEP' tests/data/p2_delete_by_empty_repl.txt | wc -l | tr -d ' ' || true)
[[ ${kcnt:-0} -ge 2 ]] && pass "delete: KEEP >= 2 (=$kcnt)" || fail "delete: KEEP too few ($kcnt)"

echo
echo "✅ All checks passed."
