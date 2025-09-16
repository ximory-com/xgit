# XGit Line 指令集 v0.1（草案）

## 目标
在不依赖 `git diff` 的前提下，完成**行级别**的插入 / 替换 / 删除等操作；锚点稳定、顺序可控、幂等、安全可回滚。

---

## 通用规则
- **语法外壳**：沿用补丁包裹协议：块头 `=== line.xxx: "<path>" ===`，随后参数区（KV 与 `k<>k` 多行值），空行后为正文，收尾 `=== end ===`，全文件以 `=== PATCH EOF ===` 结束。
- **定位**：二选一  
  1) `lineno>0` → 直接定位；  
  2) `lineno=0`（默认）→ 使用 `keys`（宽松 AND 匹配）唯一定位到**一行**。
- **匹配（keys）**：忽略大小写(`icase=1`)、忽略行首缩进（`TrimLeft` 空白后比较）。当 `keys` 多值时，要求**同一行同时包含全部关键字**（AND）。
  - `keys` 多值写法：  
    - 单行：`keys=a|b|c` 或 `keys=a,b,c`  
    - 多行块：  
      ```
      keys<
       a
       b
       c
      >keys
      ```
- **顺序**：按补丁中出现的顺序依次执行（一个操作写盘后，下一个读取最新文件）。
- **冲突控制**：默认**不做冲突检测**（“顺序即真理”）。如需严格：`strict=1` → 同一文件同一目标行/重叠操作报错。
- **换行**：`ensure_nl=1`（默认）→ 写回时强制末尾换行。
- **幂等**：  
  - `insert_*`：插入内容已存在于目标位置 → `ℹ️ 已存在，跳过`；  
  - `replace_*`：新旧一致 → 当 `allow_noop=1` 视为成功，否则记录 `ℹ️ 无变化`；  
  - `delete_*`：目标不存在 → 默认报错；`allow_noop=1` 时跳过。
- **日志**：控制台 = `patch.log` 镜像；每条操作打印：`操作名 文件:行 影响行数 (+/-N)`。

---

## 指令清单

### 1) `line.insert_before: "<path>"`
在目标行**之前**插入正文。
- **定位**：`lineno` 或 `keys`
- **正文**：插入的多行文本
- **示例**
```
=== line.insert_before: "apps/web/web.css" ===
keys<
 /* XGit overrides */
>keys
icase=1
ensure_nl=1

:root{--brand:#2563eb}
=== end ===
```

### 2) `line.insert_after: "<path>"`
在目标行**之后**插入正文。
- 用法同上。

### 3) `line.replace_line: "<path>"`
将**整行**替换成正文（允许单行或多行替换）。
- 示例
```
=== line.replace_line: "apps/web/index.html" ===
keys<
 XGit
 Web App
>keys
icase=1

<title>XGit</title>
=== end ===
```

### 4) `line.delete_line: "<path>"`
删除目标行（整行移除）。
- `allow_noop=1` 可在未命中时静默跳过。

> 以上四条覆盖 80% 修改类需求。下面是“区间类”可选项（后续扩展）。

### 5) `line.insert_block_before: "<path>"`
在目标行之前插入一个多行区块。

### 6) `line.insert_block_after: "<path>"`
在目标行之后插入一个多行区块。

### 7) `line.replace_block: "<path>"`
以多行正文替换目标行。

### 8) `line.delete_block: "<path>"`
删除一段多行内容。

---

## 示例补丁

```
repo: xgit
commitmsg: feat(line): demo line.* usage
author: XGit Bot <bot@xgit.local>

=== line.insert_after: "apps/web/index.html" ===
keys<
 <body>
>keys

<div class="banner">Hello LineOps!</div>
=== end ===

=== PATCH EOF ===
```
