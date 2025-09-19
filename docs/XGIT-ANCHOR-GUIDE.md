# XGit 补丁锚点写法指南 v1.0

> 面向 **line.*** / **block.*** 指令的定位规则与写法速查。目标：**少参、稳命中、错必可解释**。

---

## 0) 统一约定

- 行与块的定位优先级：`lineno`（相对行号） > `keys`（关键词组合）。
- **keys 匹配（宽松 AND）**：同一行只要**同时包含**所有关键字即命中（忽略大小写、忽略行首缩进）。
- **多处命中**：报错；可用 `nthl`（line）或 `nthb`（block）指定第 N 个命中（1-based）。
- **范围模式（可选）**：
  - `start_keys` 必须命中（唯一或用 `nthb` 指定）；`end_keys` 可省略（省略=直到 EOF）。
  - 所有 line 操作若提供范围参数，**只能在范围内部**执行（含 `lineno` / `offset`）。
- **offset**：行定位后支持 `offset`（可为负），在命中行的前/后相对移动再操作。
- **正文**：多行正文原样写入；系统统一 LF 并保证末尾换行。
- **幂等**：无变化不会报错，但会在日志标注为 noop（由实现决定）。
- **无空行规则**：参数区与正文区**之间不留空行**。

---

## 1) line.* 指令

支持：`line.insert`（在命中行**前**插）、`line.append`（在命中行**后**补）、`line.replace`（整行替换，可多行）、`line.delete`（删一行）  
定位方式三选一/组合：`lineno` | `keys` (+ `nthl`) | `keys` + `offset`

### 1.1 基础示例

**在 `<head>` 后追加 `<link>`（keys 命中 + append）**
```
=== line.append: "apps/web/index.html" ===
keys<
 <head>
>keys
    <link rel="stylesheet" href="./web.css">
=== end ===
```

**把 `<title>…</title>` 那行替换为新标题（keys 命中 + replace）**
```
=== line.replace: "apps/web/index.html" ===
keys<
 <title>
>keys
    <title>XGit Web App</title>
=== end ===
```

**删除精确匹配的行（lineno）**
```
=== line.delete: "docs/README.md" ===
lineno=3
=== end ===
```

**对命中行的上一行插入（offset=-1）**
```
=== line.insert: "apps/web/index.html" ===
keys<
 <body>
>keys
offset=-1
<!— before <body> —>
=== end ===
```

### 1.2 范围 + 行定位

> 仅在范围内找行，避免全局歧义；`lineno` 在范围内为**相对行号**。

**在函数块内第 3 行后补一行**
```
=== line.append: "pkg/api/handler.go" ===
start_keys<
 func Hello(
>start_keys
end_keys<
 }
>end_keys
lineno=3
    log.Println("in hello")
=== end ===
```

**范围 + keys + nthl：在 `<ul>` 容器内命中第 2 个 `<li>` 并替换**
```
=== line.replace: "apps/site/index.html" ===
start_keys<
 <ul class="menu">
>start_keys
end_keys<
 </ul>
>end_keys
keys<
 <li>
>keys
nthl=2
    <li><a href="/docs">Docs</a></li>
=== end ===
```

---

## 2) block.* 指令

支持：`block.delete`、`block.replace`（正文为替换后的整块）。  
**必须提供范围**：`start_keys`（必需，唯一或用 `nthb` 指定）+ 可选 `end_keys`（省略=到 EOF）。

**删除一个函数块**
```
=== block.delete: "pkg/svc/user.go" ===
start_keys<
 func Unused(
>start_keys
end_keys<
 }
>end_keys
=== end ===
```

**用一段新实现替换整块**
```
=== block.replace: "pkg/svc/user.go" ===
start_keys<
 func Hello(
>start_keys
end_keys<
 }
>end_keys
func Hello(w http.ResponseWriter, r *http.Request) {
    _, _ = w.Write([]byte("hi"))
}
=== end ===
```

---

## 3) 进阶要点

- **nthl / nthb**：当 `keys`（行）或 `start_keys`（块）命中多处时，用来选第 N 个命中；缺省为 1（必须唯一或报错）。
- **keys 写法**：
  - 单行或多行均可，多行视为多关键词 AND。
  - 也支持 `a|b|c` 写在一行（等价多关键词）。
- **范围与 offset**：在范围模式下允许 `offset`，但**不能越界**；越界直接报错。
- **失败日志**：命中 0 行 → 打印样本头尾片段；命中多行 → 打印前若干命中行号提示扩充 keys 或使用 nth。

---

## 4) 常见配方

**在 `</head>` 前插入 `meta viewport`**
```
=== line.insert: "apps/web/index.html" ===
keys<
 </head>
>keys
    <meta name="viewport" content="width=device-width,initial-scale=1">
=== end ===
```

**在 `<script type="module">` 行后追加一行 import**
```
=== line.append: "apps/web/index.html" ===
keys<
 <script type="module">
>keys
    import '/main.js'
=== end ===
```

**替换 `.btn` 样式整行（多行替换）**
```
=== line.replace: "apps/web/web.css" ===
keys<
 .btn{
>keys
.btn{padding:10px 14px;border-radius:8px}
.btn--primary{background:#1677ff;color:#fff}
=== end ===
```

---

## 5) 故障排查

- **未命中**：确认关键词是否在**同一行**内；必要时增加更独特的关键字或切换到范围模式。
- **多处命中**：增加关键字、或在 line 用 `nthl`、在 block 用 `nthb`。
- **越界/偏移失败**：检查 `offset` 是否超出范围；或改用 `lineno` 相对行号。
- **混淆大小写/缩进**：匹配默认不区分大小写并忽略行首缩进；若必须精确，可在关键字增加独特空白/符号。

---

文档版本：v1.0
