# XGit Line 指令集 v0.1（草案）

## 目标

在不依赖 `git diff` 的前提下，完成**行级别**的插入 / 替换 /
删除等操作；锚点稳定、顺序可控、幂等、安全可回滚。

------------------------------------------------------------------------

## 通用规则

-   **语法外壳**：使用你已定的补丁包裹规范（头为
    `=== line.xxx: "<path>" ===`，参数在 body 的 KV
    区，正文为负载文本，`=== end ===` 结束，末尾 `=== PATCH EOF ===`）。
-   **定位**：二选一
    1)  `lineno>0` → 直接定位；\
    2)  `lineno=0`（默认）→ 使用 `keys`（宽松 AND
        匹配）唯一定位到**一行**。
-   **匹配（keys）**：忽略大小写(`icase=1`)、忽略行首缩进（自动
    `TrimLeft` 空白后比较）。当 `keys`
    多值时，要求**同一行同时包含全部关键字**（AND）。
    -   `keys` 多值可用：
        -   单行：`keys=a|b|c`\
        -   重复键：多行 `keys=...`\
        -   多行块：`keys< ... >keys`（每行一个，首行加空格）
-   **顺序**：按补丁中出现的顺序依次执行（一个操作写盘后，下一个读取最新文件）。
-   **冲突**：默认**不做冲突检测**（顺序即真理）。如需严格：`strict=1` →
    同一文件同一目标行/重叠操作报错。
-   **换行**：`ensure_nl=1`（默认）→ 写回时强制末尾换行。
-   **幂等**：
    -   `insert_*` 如插入内容已存在于目标位置 → 记 `ℹ️ 已存在，跳过`。\
    -   `replace_*` 新旧一致 → `allow_noop=1` 视为成功，否则报
        `ℹ️ 无变化` 并跳过/成功（实现层可选其一）。\
    -   `delete_*` 目标不存在 → 默认报错；`allow_noop=1` 时跳过。
-   **日志**：控制台 = patch.log
    镜像；每条操作打印：`操作名 文件:行 影响行数 (+/-N)`。

------------------------------------------------------------------------

## 指令清单

### 1) `line.insert_before: "<path>"`

在目标行**之前**插入正文。 - **定位**：`lineno` 或 `keys` -
**正文**：插入的多行文本 - **示例** \`\`\` === line.insert_before:
"apps/web/web.css" === keys\< /\* XGit overrides \*/ \>keys icase=1
ensure_nl=1

:root{--brand:#2563eb} === end === \`\`\`

### 2) `line.insert_after: "<path>"`

在目标行**之后**插入正文。 - 用法同上。

### 3) `line.replace_line: "<path>"`

将**整行**替换成正文。 - 正文：替换后的"单行或多行"（通常 1 行） - 示例
\`\`\` === line.replace_line: "apps/web/index.html" === keys\< XGit Web
App \>keys icase=1

```{=html}
<title>
```
XGit
```{=html}
</title>
```
=== end === \`\`\`

### 4) `line.delete_line: "<path>"`

删除目标行（整行移除）。 - `allow_noop=1` 可在未命中时静默跳过。

> 以上四个覆盖 80% 修改类需求。下面是"区间类"可选项（可后续实现）。

### 5) `line.replace_range: "<path>"`（可选）

将 `[start..end]` 区间替换为正文。 -
参数：`start`、`end`（二选一：显式行号；或 `start_keys`/`end_keys`
锚点唯一命中） - 使用场景：小函数/小段模板整体替换。

### 6) `line.delete_range: "<path>"`（可选）

删除 `[start..end]` 区间。 - 参数同上，支持 `allow_noop=1`。

------------------------------------------------------------------------

## 参数一览（执行层读取自 `Args`）

-   `lineno`（默认 0）\
-   `keys`（多值；AND；见"匹配"）\
-   `icase`（默认 1）\
-   `ensure_nl`（默认 1）\
-   `allow_noop`（默认 0）\
-   `strict`（默认 0）\
-   （可选）`preview`（1：仅预览，不落盘；将会打印定位结果与 unified
    diff）

------------------------------------------------------------------------

## 匹配细节

-   预处理：取目标文件每一行的**文本副本**做匹配：`strings.TrimLeft(line, "    ")`，再做大小写统一（当
    `icase=1`）。\
-   命中规则：
    -   单值：该行包含子串 `keys`。\
    -   多值：同一行包含所有子串（AND）。\
-   计数：
    -   **0 命中**：报错并打印采样（前后 ±2 行）。\
    -   **\>1 命中**：报错并打印前 5 个命中行号，建议加关键字或改用
        `lineno`。

------------------------------------------------------------------------

## 执行顺序与偏移

-   每条操作**立即写回**，下一条基于最新文件。\
-   如需"同文件内一次性多操作并避免偏移"，可在实现层提供
    `mode=batch`：同文件计划合并后**按行号降序**执行（可留作扩展）。

------------------------------------------------------------------------

## 日志规范（建议）

-   定位阶段：
    -   `🔎 锚点 apps/web/index.html keys="</header>" -> L=37 (icase=1)`\
    -   `⚠️ 多命中 3 处：L=37, 89, 120`
-   执行阶段：
    -   `➕ insert_after apps/web/index.html:L37 (+3)`\
    -   `✏️ replace_line apps/web/index.html:L10 (1→1)`\
    -   `🗑️ delete_line apps/web/index.html:L58 (-1)`
-   事务：
    -   `✅ 预检通过` / `❌ 失败并回滚`

------------------------------------------------------------------------

## 示例补丁（混合操作）

    repo: xgit
    commitmsg: feat(web): header tidy, add site nav, cap hero logo
    author: XGit Bot <bot@xgit.local>

    === line.replace_line: "apps/web/index.html" ===
    keys<
     XGit
     Web App
    >keys
    icase=1

    <title>XGit</title>
    === end ===

    === line.insert_after: "apps/web/index.html" ===
    keys=</header>
    icase=1

    <nav class="nav">
      <a class="link-site" href="https://xgit.ximory.com">返回官网</a>
    </nav>
    === end ===

    === line.insert_before: "apps/web/web.css" ===
    lineno=1

    /* XGit overrides */
    :root{--brand:#2563eb}
    .nav{display:flex;justify-content:flex-end;margin:8px 0 16px}
    .link-site{color:var(--brand);text-decoration:none}
    .link-site:hover{text-decoration:underline}
    === end ===

    === PATCH EOF ===

------------------------------------------------------------------------

## 测试建议

1)  单命中 / 多命中 / 未命中\
2)  大小写差异 / 缩进差异\
3)  插入前后幂等（重复执行不脏数据）\
4)  `allow_noop` 分支\
5)  `strict=1` 冲突检测\
6)  回滚：中途一条失败，前面改动被撤回
