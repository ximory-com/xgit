@@
-for (( i=0; i<\${#blocks_path[@]:-0}; i++ )); do
-  p="\${blocks_path[\$i]-}"; a="\${blocks_anchor[\$i]-}"; m="\${blocks_mode[\$i]-}"
-  tmp="\${blocks_tmp[\$i]-}"; idx="\${blocks_index[\$i]-1}"
-  [[ -z "\${p:-}" || -z "\${a:-}" || -z "\${tmp:-}" || -z "\${idx:-}" ]] && continue
-  ensure_canonical_in_repo "\$p"; mkdir -p "\$(dirname "\$p")"
-  if apply_block "\$REPO/\$p" "\$a" "\$m" "\$tmp" "\$idx"; then
-    git add "\$p"; log "✅ 区块：\$p #\$a (\$m @index=\$idx)"
-  else
-    log "❌ 区块失败：\$p #\$a (\$m @index=\$idx)"; exit 1
-  fi
-done
+for (( i=0; i<\${#blocks_path[@]:-0}; i++ )); do
+  p="\${blocks_path[\$i]-}"; a="\${blocks_anchor[\$i]-}"; m="\${blocks_mode[\$i]-}"
+  tmp="\${blocks_tmp[\$i]-}"; idx="\${blocks_index[\$i]-1}"
+  [[ -z "\${p:-}" || -z "\${a:-}" || -z "\${tmp:-}" || -z "\${idx:-}" ]] && continue
+  ensure_canonical_in_repo "\$p"; mkdir -p "\$(dirname "\$p")"
+  # 传相对路径，避免绝对/相对混用引起的歧义
+  if apply_block "\$p" "\$a" "\$m" "\$tmp" "\$idx"; then
+    git add "\$p"; log "✅ 区块：\$p #\$a (\$m @index=\$idx)"
+  else
+    log "❌ 区块失败：\$p #\$a (\$m @index=\$idx)"; exit 1
+  fi
+done
@@
-apply_block(){
-  # $1=file, $2=anchor, $3=mode, $4=content_file, $5=index (1-based)
-  local file="$1" anchor="$2" mode="$3" content_file="$4" index="${5:-1}"
-
-  # 文件不存在则创建
-  if [[ ! -e "$file" ]]; then
-    mkdir -p "$(dirname "$file")"
-    :> "$file"
-  fi
-
-  # 尝试定位第 index 个锚区；若不存在则引导空锚点并重试一次
-  locate_and_apply(){
-    local file="$1" anchor="$2" mode="$3" content_file="$4" index="$5"
-
-    # 栈匹配所有 (begin,end) 对
-    read -r start end count < <(python3 - "$file" "$anchor" "$index" <<'PY'
+apply_block(){
+  # $1=file(relative to REPO), $2=anchor, $3=mode, $4=content_file, $5=index (1-based)
+  local rel="$1" anchor="$2" mode="$3" content_file="$4" index="${5:-1}"
+  local file="$REPO/$rel"
+
+  # 文件不存在则创建
+  if [[ ! -e "$file" ]]; then
+    mkdir -p "$(dirname "$file")"
+    :> "$file"
+  fi
+
+  locate_and_apply(){
+    local file="$1" anchor="$2" mode="$3" content_file="$4" index="$5"
+    # 栈匹配所有 (begin,end) 对；返回 start end total
+    read -r start end total < <(python3 - "$file" "$anchor" "$index" <<'PY'
 import sys,re
 path,anchor,idx=sys.argv[1],sys.argv[2],int(sys.argv[3])
 rx_b=re.compile(r'XGIT:\s*BEGIN\s+'+re.escape(anchor)+r'\b',re.I)
 rx_e=re.compile(r'XGIT:\s*END\s+'  +re.escape(anchor)+r'\b',re.I)
 lines=open(path,'rb').read().decode('utf-8','replace').splitlines()
 pairs=[]; stack=[]
 for i,l in enumerate(lines,1):
     if rx_b.search(l): stack.append(i)
     if rx_e.search(l) and stack:
         s=stack.pop(); pairs.append((s,i))
 pairs.sort(key=lambda p:p[0])
-if 1<=idx<=len(pairs):
-    print(pairs[idx-1][0], pairs[idx-1][1], len(pairs))
-else:
-    print('', '', len(pairs))
+total=len(pairs)
+if 1<=idx<=total:
+    print(pairs[idx-1][0], pairs[idx-1][1], total)
+else:
+    print('', '', total)
 PY
 )
-    if [[ -z "${start:-}" || -z "${end:-}" ]]; then
-      return 1
-    fi
+    if [[ -z "${start:-}" || -z "${end:-}" ]]; then
+      return 1
+    fi
@@
-    return 0
+    log "🧩 命中锚区：$rel #$anchor (start=$start,end=$end,total=$total, mode=$mode)"
+    return 0
   }
 
-  # 第一次尝试定位/应用
-  if locate_and_apply "$file" "$anchor" "$mode" "$content_file" "$index"; then
-    return 0
-  fi
+  # 第一次尝试
+  if locate_and_apply "$file" "$anchor" "$mode" "$content_file" "$index"; then return 0; fi
 
-  # 若未找到：自动引导空锚点到文件末尾（仅一次），再尝试一次
+  # 若未找到：自动引导空锚点到文件末尾（仅一次）
   {
     printf '\n<!-- XGIT:BEGIN %s -->\n' "$anchor"
     printf '<!-- XGIT:END %s -->\n' "$anchor"
   } >>"$file"
-  log "ℹ️ 自动引导空锚点：$file #$anchor（@index=$index）"
+  log "ℹ️ 自动引导空锚点：$rel #$anchor (@index=$index)"
 
-  if locate_and_apply "$file" "$anchor" "$mode" "$content_file" "$index"; then
-    return 0
-  fi
+  # 重新扫描，优先命中“最后一个”锚区（避免历史未配对影响）
+  read -r _s _e total <<<'$(python3 - "$file" "$anchor" 1 <<'PY'
+import sys,re
+path,anchor=sys.argv[1],sys.argv[2]
+import re
+rb=re.compile(r'XGIT:\s*BEGIN\s+'+re.escape(anchor)+r'\b',re.I)
+re_=re.compile(r'XGIT:\s*END\s+'  +re.escape(anchor)+r'\b',re.I)
+lines=open(path,'rb').read().decode('utf-8','replace').splitlines()
+pairs=[]; stack=[]
+for i,l in enumerate(lines,1):
+    if rb.search(l): stack.append(i)
+    if re_.search(l) and stack:
+        s=stack.pop(); pairs.append((s,i))
+print(0,0,len(pairs))
+PY
+)'
+  # 选 total（最后一个）
+  if [[ "${total:-0}" -ge 1 ]]; then
+    if locate_and_apply "$file" "$anchor" "$mode" "$content_file" "$total"; then return 0; fi
+  fi
 
-  log "❌ 未找到锚区或 index 超界：$file #$anchor @index=$index"
+  log "❌ 未找到锚区或 index 超界：$rel #$anchor @index=$index"
   return 1
 }

