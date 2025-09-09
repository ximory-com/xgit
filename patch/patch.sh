#!/usr/bin/env bash
# XGit patch.sh (STRICT v1.6.4)
# - 事务：失败即整单回滚（trap ERR）+ EXIT 兜底
# - 干净策略 REQUIRE_CLEAN：1(默认)=拒绝脏区 | auto=reset --hard + clean -fd | 0=忽略
# - 僵尸锁自愈：.patch.lock/pid 不存活则自动清锁
# - block：@index, append_once, 嵌套识别, 缺失自动引导(可回滚)
# - diff：统一补丁 git apply（缺省 opts 安全；Bash 3.2 兼容）
# - 严格 EOF；默认推送开启
set -Eeuo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
PATCH_FILE_DEFAULT="${SCRIPT_DIR}/文本.txt"
PATCH_FILE="${PATCH_FILE:-$PATCH_FILE_DEFAULT}"
LOG_FILE="${SCRIPT_DIR}/patch.log"
LOCK_DIR="${SCRIPT_DIR}/.patch.lock"
LOCK_PID="${LOCK_DIR}/pid"
EOF_MARK="${EOF_MARK:-=== PATCH EOF ===}"

ts(){ date '+%F %T'; }
log(){ echo "$(ts) $*" | tee -a "$LOG_FILE"; }

# ---------- 单实例锁（含僵尸锁自愈） ----------
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  # 已存在；判断是否僵尸
  if [[ -f "$LOCK_PID" ]]; then
    old_pid="$(cat "$LOCK_PID" 2>/dev/null || true)"
    if [[ -n "${old_pid:-}" ]] && ! ps -p "$old_pid" >/dev/null 2>&1; then
      log "⚠️ 检测到僵尸锁(pid=$old_pid)，自动清理…"
      rm -rf "$LOCK_DIR" || true
      mkdir "$LOCK_DIR" || { log "❌ 无法重建锁目录"; exit 1; }
    else
      log "❌ 已有 patch 实例在运行，退出。"; exit 1
    fi
  else
    log "⚠️ 锁目录存在但无 pid，尝试自愈…"
    rm -rf "$LOCK_DIR" || true
    mkdir "$LOCK_DIR" || { log "❌ 无法重建锁目录"; exit 1; }
  fi
fi
echo "$$" > "$LOCK_PID"
cleanup_lock(){ rm -rf "$LOCK_DIR" 2>/dev/null || true; }
trap 'cleanup_lock' INT TERM

# ---------- 补丁存在性 ----------
if [[ ! -f "$PATCH_FILE" ]]; then
  log "ℹ️ 未找到补丁文件：$PATCH_FILE"
  log "================ patch.sh end ================"; cleanup_lock; exit 0
fi

# ---------- 通用工具 ----------
trim(){ local s="${1%$'\r'}"; s="${s#"${s%%[![:space:]]*}"}"; s="${s%"${s##*[![:space:]]}"}"; printf '%s' "$s"; }
lower(){ printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }
upper(){ printf '%s' "$1" | tr '[:lower:]' '[:upper:]'; }

norm_path(){ # 标准化路径 + 名字大小写规范 + 去首尾空白
  local p="$(trim "$1")"
  p="$(printf '%s' "$p" | sed 's#//*#/#g; s#^\./##')"
  local dir base name ext ext_l
  dir="$(dirname "$p")"; base="$(basename "$p")"
  if [[ "$base" == *.* ]]; then name="${base%.*}"; ext="${base##*.}"; else name="$base"; ext=""; fi
  ext_l="$(lower "$ext")"
  if [[ -z "$ext" || "$ext_l" == "md" ]]; then name="$(upper "$name")"; else name="$(lower "$name")"; fi
  local base2="$name"; [[ -n "$ext_l" ]] && base2="${name}.${ext_l}"
  [[ "$dir" == "." ]] && printf '%s\n' "$base2" || printf '%s/%s\n' "$dir" "$base2"
}

ensure_canonical_in_repo(){ # 修正大小写差异
  local want="$1"; local abs="$REPO/$want"
  mkdir -p "$(dirname "$abs")"
  if [[ -e "$abs" ]]; then return 0; fi
  local parent leaf hit
  parent="$(dirname "$want")"; leaf="$(basename "$want")"
  hit="$( (cd "$REPO/$parent" 2>/dev/null && find . -maxdepth 1 -iname "$leaf" -print | sed 's#^\./##') || true )"
  if [[ -n "$hit" && "$hit" != "$leaf" && -e "$REPO/$parent/$hit" ]]; then
    ( cd "$REPO" && git mv -f "$parent/$hit" "$want" ) || true
  fi
}

first_field(){ sed -n "s/^$1:[[:space:]]*//p; q" "$PATCH_FILE" | head -n1; }
find_git_root(){ local s="$1"; while [[ -n "$s" && "$s" != "/" ]]; do [[ -d "$s/.git" ]] && { echo "$s"; return 0; }; s="$(dirname "$s")"; done; return 1; }
is_repo(){ git -C "$1" rev-parse --is-inside-work-tree >/dev/null 2>&1; }

# ---------- 仓库定位（映射 + 缓存 + 兄弟目录 + 向上查找） ----------
PATCH_DIR="$(cd "$(dirname "$PATCH_FILE")" && pwd -P)"
MAP_PATCH="$PATCH_DIR/.repos"; MAP_SCRIPT="$SCRIPT_DIR/.repos"; MAP_GLOBAL="$HOME/.config/xgit/repos"
CACHE_PATCH="$PATCH_DIR/.repo";  CACHE_GLOBAL="$HOME/.config/xgit/repo"

repo_from_maps(){ local name="$1"; local f line k v
  for f in "$MAP_PATCH" "$MAP_SCRIPT" "$MAP_GLOBAL"; do
    [[ -f "$f" ]] || continue
    while IFS= read -r line || [[ -n "$line" ]]; do
      line="${line%%#*}"; line="${line%%;*}"; line="$(trim "$line")"; [[ -z "$line" ]] && continue
      if echo "$line" | grep -qE '^[[:space:]]*default[[:space:]]*='; then
        v="$(echo "$line" | sed -E 's/^[[:space:]]*default[[:space:]]*=[[:space:]]*(\S+).*/\1/')"; [[ "$name" == "default" ]] && { printf '%s\n' "$v"; return 0; }
      elif echo "$line" | grep -qE '^[^=[:space:]]+[[:space:]]*='; then
        k="$(echo "$line" | sed -E 's/^([^=[:space:]]+).*/\1/')"; v="$(echo "$line" | sed -E 's/^[^=[:space:]]+[[:space:]]*=[[:space:]]*(.+)$/\1/')"; [[ "$k" == "$name" ]] && { printf '%s\n' "$v"; return 0; }
      elif echo "$line" | grep -qE '^[^[:space:]]+[[:space:]]+/'; then
        k="$(echo "$line" | awk '{print $1}')"; v="$(echo "$line" | sed -E 's/^[^[:space:]]+[[:space:]]+(.+)$/\1/')"; [[ "$k" == "$name" ]] && { printf '%s\n' "$v"; return 0; }
      fi
    done < "$f"
  done
  return 1
}

scan_sibling_repo(){ local base="$1"; local parent="$(dirname "$base")"; local hits=0 last=""
  while IFS= read -r d; do [[ -d "$d/.git" ]] || continue; hits=$((hits+1)); last="$d"; done < <(find "$parent" -mindepth 1 -maxdepth 1 -type d 2>/dev/null)
  [[ $hits -eq 1 ]] && printf '%s\n' "$last" || printf '%s\n' ""
}

candidates=()
[[ -n "${REPO:-}" ]] && candidates+=("$(trim "$REPO")")
REPO_HDR="$(trim "$(first_field repo || true)")"
if [[ -n "$REPO_HDR" ]]; then
  if [[ "$REPO_HDR" = /* ]]; then candidates+=("$REPO_HDR")
  else
    if MAP_PATH="$(repo_from_maps "$REPO_HDR" 2>/dev/null || true)"; then candidates+=("$MAP_PATH")
    elif MAP_DEF_NAME="$(repo_from_maps default 2>/dev/null || true)"; then
      if MAP_DEF_PATH="$(repo_from_maps "$MAP_DEF_NAME" 2>/dev/null || true)"; then candidates+=("$MAP_DEF_PATH"); fi
    fi
  fi
fi
[[ -f "$CACHE_PATCH"  ]] && candidates+=("$(trim "$(cat "$CACHE_PATCH")")")
[[ -f "$CACHE_GLOBAL" ]] && candidates+=("$(trim "$(cat "$CACHE_GLOBAL")")")
sib1="$(scan_sibling_repo "$SCRIPT_DIR")"; [[ -n "$sib1" ]] && candidates+=("$sib1")
sib2="$(scan_sibling_repo "$PATCH_DIR")";  [[ -n "$sib2" ]] && candidates+=("$sib2")
root1="$(find_git_root "$SCRIPT_DIR" || true)"; [[ -n "$root1" ]] && candidates+=("$root1")
root2="$(find_git_root "$PATCH_DIR"  || true)"; [[ -n "$root2" ]] && candidates+=("$root2")

uniq_candidates=()
for c in "${candidates[@]:-}"; do
  [[ -z "$c" ]] && continue
  c="$(python3 - <<'PY' "$c"
import os,sys; print(os.path.realpath(sys.argv[1]))
PY
)"; skip=0
  for u in "${uniq_candidates[@]:-}"; do [[ "$u" == "$c" ]] && { skip=1; break; }; done
  [[ $skip -eq 1 ]] && continue
  uniq_candidates+=("$c")
done

REPO=""
for c in "${uniq_candidates[@]:-}"; do if is_repo "$c"; then REPO="$c"; break; fi; done
if [[ -z "$REPO" ]]; then
  log "❌ 未能自动定位 Git 仓库根目录。可在补丁头写 repo: /abs/path 或 repo: <name>（配合 .repos），或导出 REPO=/abs/path。"
  false
fi
mkdir -p "$(dirname "$CACHE_GLOBAL")"
printf '%s\n' "$REPO" >"$CACHE_PATCH"
printf '%s\n' "$REPO" >"$CACHE_GLOBAL"

log "================ patch.sh begin ================"
log "📂 仓库根目录：$REPO"
log "📄 补丁文件：$PATCH_FILE"

# ---------- 提交信息 ----------
COMMIT_MSG="$(trim "$(first_field commitmsg || true)")"
AUTHOR_LINE="$(trim "$(first_field author || true)")"
log "📝 提交说明：${COMMIT_MSG:-(空)}"
log "👤 提交作者：${AUTHOR_LINE:-(空)}"

# ---------- 严格 EOF ----------
LAST_MEANINGFUL_LINE="$(awk 'NF{last=$0} END{print last}' "$PATCH_FILE")"
if [ "${LAST_MEANINGFUL_LINE:-}" != "${EOF_MARK:-}" ]; then
  log "❌ 严格 EOF 校验失败：期望『${EOF_MARK:-}』，实得『${LAST_MEANINGFUL_LINE:-}』"
  false
fi

# ---------- 解析补丁 ----------
files_todo=(); files_tmp=()
deletes_todo=()
moves_from=(); moves_to=()
blocks_path=(); blocks_anchor=(); blocks_mode=(); blocks_tmp=(); blocks_index=()
diffs_tmp=(); diffs_opts=()

in_block=0; cur_path=""; cur_tmp=""; cur_anchor=""; cur_mode=""; cur_index=""; cur_diff_opts=""

while IFS= read -r raw || [[ -n "$raw" ]]; do
  line="${raw%$'\r'}"

  # file
  if [[ $in_block -eq 0 ]] && echo "$line" | grep -qE '^=== file:'; then
    cur_path="$(norm_path "$(trim "$(echo "$line" | sed -E 's/^=== file:[[:space:]]*(.+)[[:space:]]*===$/\1/')")")"
    cur_tmp="$(mktemp)"; in_block=1; continue
  fi
  # delete
  if [[ $in_block -eq 0 ]] && echo "$line" | grep -qE '^=== delete:'; then
    deletes_todo+=( "$(norm_path "$(trim "$(echo "$line" | sed -E 's/^=== delete:[[:space:]]*(.+)[[:space:]]*===$/\1/')")")" ); continue
  fi
  # mv
  if [[ $in_block -eq 0 ]] && echo "$line" | grep -qE '^=== mv:'; then
    from="$(trim "$(echo "$line" | sed -E 's/^=== mv:[[:space:]]*(.+)[[:space:]]*=>[[:space:]]*(.+)[[:space:]]*===$/\1/')")"
    to="$(trim   "$(echo "$line" | sed -E 's/^=== mv:[[:space:]]*(.+)[[:space:]]*=>[[:space:]]*(.+)[[:space:]]*===$/\2/')")"
    moves_from+=( "$(norm_path "$from")" ); moves_to+=( "$(norm_path "$to")" ); continue
  fi
  # block
  if [[ $in_block -eq 0 ]] && echo "$line" | grep -qE '^=== block:'; then
    cur_path="$(norm_path "$(trim "$(echo "$line" | sed -E 's/^=== block:[[:space:]]*([^#[:space:]]+)\#.*$/\1/')")")"
    cur_anchor="$(trim "$(echo "$line" | sed -E 's/^=== block:[[:space:]]*[^#]+#([A-Za-z0-9_-]+).*$/\1/')")"
    cur_mode="$(trim "$(echo "$line" | sed -E 's/^.*mode=(replace|append|prepend|append_once).*$/\1/')" )"; [[ -z "$cur_mode" ]] && cur_mode="replace"
    cur_index="$(echo "$line" | sed -nE 's/^=== block:[[:space:]]*[^#]+#[^[:space:]@]+@index=([0-9]+).*$/\1/p')"; [[ -z "$cur_index" ]] && cur_index="1"
    cur_tmp="$(mktemp)"; in_block=2; continue
  fi
  # diff
  if [[ $in_block -eq 0 ]] && echo "$line" | grep -qE '^=== diff:'; then
    cur_diff_opts="$(trim "$(echo "$line" | sed -E 's/^=== diff:[[:space:]]*(.+)[[:space:]]*===$/\1/')" )"
    cur_tmp="$(mktemp)"; in_block=3; continue
  fi

  # end
  if [[ $in_block -eq 1 && "$line" == "=== end ===" ]]; then
    files_todo+=( "$cur_path" ); files_tmp+=( "$cur_tmp" ); in_block=0; cur_path=""; cur_tmp=""; continue
  fi
  if [[ $in_block -eq 2 && "$line" == "=== end ===" ]]; then
    blocks_path+=( "$cur_path" ); blocks_anchor+=( "$cur_anchor" ); blocks_mode+=( "$cur_mode" ); blocks_tmp+=( "$cur_tmp" ); blocks_index+=( "$cur_index" )
    in_block=0; cur_path=""; cur_tmp=""; cur_anchor=""; cur_mode=""; cur_index=""; continue
  fi
  if [[ $in_block -eq 3 && "$line" == "=== end ===" ]]; then
    diffs_tmp+=( "$cur_tmp" ); diffs_opts+=( "$cur_diff_opts" ); in_block=0; cur_tmp=""; cur_diff_opts=""; continue
  fi

  # EOF
  if [[ "$line" == "${EOF_MARK:-}" ]]; then break; fi

  # 内容收集
  if [[ $in_block -eq 1 || $in_block -eq 2 || $in_block -eq 3 ]]; then printf '%s\n' "$line" >>"$cur_tmp"; fi
done < "$PATCH_FILE"

if [[ $in_block -ne 0 ]]; then log "❌ 补丁块未正常结束。"; false; fi

log "📦 统计：file=${#files_todo[@]} delete=${#deletes_todo[@]} mv=${#moves_from[@]} block=${#blocks_path[@]} diff=${#diffs_tmp[@]}"

# ---------- 执行（进入仓库 + 干净策略 + 开启事务） ----------
cd "$REPO"

# 干净策略
case "${REQUIRE_CLEAN:-1}" in
  auto)
    log "ℹ️ 自动清理：git reset --hard && git clean -fd"
    git reset --hard >/dev/null
    git clean -fd   >/dev/null
    ;;
  1|true|yes)
    if { ! git diff --quiet || ! git diff --cached --quiet; }; then
      log "❌ 工作区不干净；为保证事务性已中止（REQUIRE_CLEAN=auto 可自动清理，=0 忽略）"
      false
    fi
    ;;
  0|false|no) : ;;
  *)
    log "⚠️ 未知 REQUIRE_CLEAN='${REQUIRE_CLEAN:-}'，按默认 1 处理"
    if { ! git diff --quiet || ! git diff --cached --quiet; }; then
      log "❌ 工作区不干净；为保证事务性已中止"; false
    fi
    ;;
esac

# 事务起点 + 回滚器（ERR & EXIT 兜底）
START_HEAD="$(git rev-parse --verify HEAD 2>/dev/null || true)"
TX_DONE=0
rollback(){
  git reset --hard "${START_HEAD:-HEAD}" >/dev/null 2>&1 || true
  git clean -fd >/dev/null 2>&1 || true
  log "↩️ 已回滚到 ${START_HEAD:-HEAD}"
}
trap 'log "❌ 出错，回滚中…"; rollback' ERR
trap 'rc=$?; if [[ $rc -ne 0 && ${TX_DONE:-0} -eq 0 ]]; then log "⚠️ 非零退出($rc)，执行兜底回滚…"; rollback; fi; cleanup_lock' EXIT

# ---------- mv / delete ----------
for (( i=0; i<${#moves_from[@]:-0}; i++ )); do
  from="${moves_from[$i]-}"; to="${moves_to[$i]-}"
  [[ -z "${from:-}" || -z "${to:-}" ]] && continue
  ensure_canonical_in_repo "$from"; ensure_canonical_in_repo "$to"
  if [[ -e "$from" ]]; then mkdir -p "$(dirname "$to")"; git mv -f "$from" "$to" || true; log "🔁 改名：$from => $to"
  else log "ℹ️ 跳过改名（不存在）：$from"; fi
done

for d in "${deletes_todo[@]:-}"; do
  [[ -z "${d:-}" ]] && continue
  ensure_canonical_in_repo "$d"
  if [[ -e "$d" ]]; then git rm -f "$d" || true; log "🗑️ 删除：$d"
  else log "ℹ️ 跳过删除（不存在）：$d"; fi
done

# ---------- file 写入 ----------
for (( i=0; i<${#files_todo[@]:-0}; i++ )); do
  p="${files_todo[$i]-}"; tmp="${files_tmp[$i]-}"
  [[ -z "${p:-}" || -z "${tmp:-}" ]] && continue
  ensure_canonical_in_repo "$p"; mkdir -p "$(dirname "$p")"
  LC_ALL=C sed -e 's/\r$//' <"$tmp" >"$p"
  if [ -s "$p" ] && [ "$(tail -c1 "$p" 2>/dev/null | wc -c)" -ne 0 ]; then printf '\n' >>"$p"; fi
  git add "$p"; log "✅ 写入文件：$p"
done
for t in "${files_tmp[@]:-}"; do rm -f "$t" 2>/dev/null || true; done

# ---------- block 应用（嵌套 + 引导 + append_once） ----------
apply_block(){
  local rel="$1" anchor="$2" mode="$3" content_file="$4" index="${5:-1}"
  local file="$REPO/$rel"
  [[ -e "$file" ]] || { mkdir -p "$(dirname "$file")"; :> "$file"; }

  locate_and_apply(){
    local file="$1" anchor="$2" mode="$3" content_file="$4" index="$5"
    read -r start end total < <(python3 - "$file" "$anchor" "$index" <<'PY'
import sys,re
path,anchor,idx=sys.argv[1],sys.argv[2],int(sys.argv[3])
rb=re.compile(r'XGIT:\s*BEGIN\s+'+re.escape(anchor)+r'\b',re.I)
re_=re.compile(r'XGIT:\s*END\s+'  +re.escape(anchor)+r'\b',re.I)
lines=open(path,'rb').read().decode('utf-8','replace').splitlines()
pairs=[]; stack=[]
for i,l in enumerate(lines,1):
    if rb.search(l): stack.append(i)
    if re_.search(l) and stack:
        s=stack.pop(); pairs.append((s,i))
pairs.sort(key=lambda p:p[0])
total=len(pairs)
if 1<=idx<=total: print(pairs[idx-1][0], pairs[idx-1][1], total)
else:             print('', '', total)
PY
)
    [[ -z "${start:-}" || -z "${end:-}" ]] && return 1

    local head_file body_file tail_file norm_content out begin_line end_line
    head_file="$(mktemp)"; body_file="$(mktemp)"; tail_file="$(mktemp)"; norm_content="$(mktemp)"; out="$(mktemp)"
    awk -v s="$start"        'NR < s' "$file" >"$head_file"
    awk -v s="$start" -v e="$end" 'NR> s && NR< e' "$file" >"$body_file"
    awk -v e="$end"          'NR > e' "$file" >"$tail_file"
    LC_ALL=C sed -e 's/\r$//' <"$content_file" >"$norm_content"
    begin_line="$(sed -n "${start}p" "$file")"; end_line="$(sed -n   "${end}p"   "$file")"

    if [[ "$mode" == "append_once" ]]; then
      if python3 - "$body_file" "$norm_content" <<'PY'
import sys
b=open(sys.argv[1],'rb').read().replace(b'\r',b'')
c=open(sys.argv[2],'rb').read().replace(b'\r',b'')
def norm(x): return b'\n'.join(ln.rstrip() for ln in x.splitlines())
sys.exit(0 if norm(c) in norm(b) else 1)
PY
      then
        out="$(mktemp)"; sed -n "1,${start}p" "$file" >"$out"; cat "$body_file" >>"$out"; sed -n "${end},\$p" "$file" >>"$out"
        mv -f "$out" "$file"; rm -f "$head_file" "$body_file" "$tail_file" "$norm_content"
        log "ℹ️ append_once：内容已存在，跳过（$rel #$anchor @index=$index）"; return 0
      fi
    fi

    sed -n "1,${start}p" "$file" >"$out"
    printf '%s\n' "$begin_line" >>"$out"
    case "$mode" in
      replace)     cat "$norm_content" >>"$out" ;;
      append)      cat "$body_file" >>"$out"; cat "$norm_content" >>"$out" ;;
      append_once) cat "$body_file" >>"$out"; cat "$norm_content" >>"$out" ;;
      prepend)     cat "$norm_content" >>"$out"; cat "$body_file" >>"$out" ;;
      *)           cat "$norm_content" >>"$out" ;;
    esac
    printf '%s\n' "$end_line" >>"$out"
    sed -n "${end},\$p" "$file" >>"$out"

    mv -f "$out" "$file"
    rm -f "$head_file" "$body_file" "$tail_file" "$norm_content"
    log "🧩 命中锚区：$rel #$anchor (mode=$mode)"
    return 0
  }

  # 首次尝试
  if locate_and_apply "$file" "$anchor" "$mode" "$content_file" "$index"; then return 0; fi

  # 自动引导（失败可回滚）
  local booted=0
  {
    printf '\n<!-- XGIT:BEGIN %s -->\n' "$anchor"
    printf '<!-- XGIT:END %s -->\n' "$anchor"
  } >>"$file" && booted=1
  log "ℹ️ 自动引导空锚点：$rel #$anchor (@index=$index)"

  # 引导后优先命中最后一个
  read -r _s _e total < <(python3 - "$file" "$anchor" 1 <<'PY'
import sys,re
path,anchor=sys.argv[1],sys.argv[2]
rb=re.compile(r'XGIT:\s*BEGIN\s+'+re.escape(anchor)+r'\b',re.I)
re_=re.compile(r'XGIT:\s*END\s+'  +re.escape(anchor)+r'\b',re.I)
lines=open(path,'rb').read().decode('utf-8','replace').splitlines()
pairs=[]; stack=[]
for i,l in enumerate(lines,1):
    if rb.search(l): stack.append(i)
    if re_.search(l) and stack:
        s=stack.pop(); pairs.append((s,i))
print(0,0,len(pairs))
PY
)
  if [[ "${total:-0}" -ge 1 ]]; then
    if locate_and_apply "$file" "$anchor" "$mode" "$content_file" "$total"; then return 0; fi
  fi

  # 回滚刚引导的锚点（避免半成品）
  if [[ "$booted" -eq 1 ]]; then
    local l1="<!-- XGIT:BEGIN ${anchor} -->"
    local l2="<!-- XGIT:END ${anchor} -->"
    local tail2; tail2="$(tail -n 2 "$file" 2>/dev/null || true)"
    if printf '%s\n' "$tail2" | grep -Fqx "$l1"$'\n'"$l2"; then
      local n; n="$(wc -l < "$file" | tr -d ' ')"
      if [[ "${n:-0}" -ge 2 ]]; then head -n $((n-2)) "$file" >"$file.tmp" && mv -f "$file.tmp" "$file"; log "↩️ 回滚引导锚点：$rel #$anchor"; fi
    fi
  fi

  log "❌ 未找到锚区或 index 超界：$rel #$anchor @index=$index"; return 1
}

for (( i=0; i<${#blocks_path[@]:-0}; i++ )); do
  p="${blocks_path[$i]-}"; a="${blocks_anchor[$i]-}"; m="${blocks_mode[$i]-}"
  tmp="${blocks_tmp[$i]-}"; idx="${blocks_index[$i]-}"   # @index 1-based
  [[ -z "${p:-}" || -z "${a:-}" || -z "${tmp:-}" || -z "${idx:-}" ]] && continue
  ensure_canonical_in_repo "$p"; mkdir -p "$(dirname "$p")"
  if apply_block "$p" "$a" "$m" "$tmp" "$idx"; then git add "$p"; log "✅ 区块：$p #$a ($m @index=$idx)"
  else log "❌ 区块失败：$p #$a ($m @index=$idx)"; false; fi
done
for t in "${blocks_tmp[@]:-}"; do rm -f "$t" 2>/dev/null || true; done

# ---------- diff（统一补丁；opts 缺省安全） ----------
apply_diff(){
  local tmp="${1-}"; local opts="${2-}"
  local mode="apply" strip="1" whitespace="nowarn" threeway="0" reverse="0" subpath=""
  for tok in ${opts:-}; do
    case "$tok" in
      mode=*) mode="${tok#mode=}" ;;
      strip=*) strip="${tok#strip=}" ;;
      whitespace=*) whitespace="${tok#whitespace=}" ;;
      threeway=1|threeway=true) threeway="1" ;;
      reverse=1|reverse=true) reverse="1" ;;
      path=*) subpath="$(trim "${tok#path=}")" ;;
    esac
  done
  local args=(--index "-p${strip}" "--whitespace=${whitespace}")
  [[ "${threeway:-0}" == "1" ]] && args+=("-3")
  if [[ "$reverse" == "1" || "$mode" == "reverse" ]]; then args+=(--reverse); fi
  local workdir="$REPO"; [[ -n "$subpath" ]] && workdir="$REPO/$subpath"
  if ! git -C "$workdir" apply --check "${args[@]}" "$tmp" >/dev/null 2>&1; then
    log "❌ git apply --check 失败：${opts:-<none>}"
    git -C "$workdir" apply --check "${args[@]}" "$tmp" || true
    return 1
  fi
  git -C "$workdir" apply "${args[@]}" "$tmp"
  log "✅ 已应用 diff（${opts:-<none>}）"
  return 0
}
for (( i=0; i<${#diffs_tmp[@]:-0}; i++ )); do
  dt="${diffs_tmp[$i]-}"; dopts="${diffs_opts[$i]-}"
  [[ -z "${dt:-}" ]] && continue
  if ! apply_diff "$dt" "${dopts-}"; then log "❌ diff 应用失败：${dopts-}"; false; fi
done
for t in "${diffs_tmp[@]:-}"; do rm -f "$t" 2>/dev/null || true; done

# ---------- 提交 & 推送（事务收尾） ----------
if git diff --cached --quiet; then
  log "ℹ️ 无改动需要提交。"
  TX_DONE=1
  log "================ patch.sh end ================"
  exit 0
fi

if [[ -n "${AUTHOR_LINE:-}" ]]; then git commit --author "$AUTHOR_LINE" -m "${COMMIT_MSG:-chore: apply patch}" >/dev/null
else git commit -m "${COMMIT_MSG:-chore: apply patch}" >/dev/null; fi
log "✅ 已提交：${COMMIT_MSG:-chore: apply patch}"

if [[ "${PUSH:-1}" == "1" ]]; then
  log "🚀 正在推送…"
  if git push origin HEAD >/dev/null; then
    log "🚀 推送完成"
    TX_DONE=1
  else
    log "❌ 推送失败；开始回滚"
    rollback; TX_DONE=1; exit 1
  fi
else
  log "ℹ️ 已禁用推送（PUSH=0）"
  TX_DONE=1
fi

log "================ patch.sh end ================"

