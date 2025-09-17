# XGIT Line & Block Protocol (Final)

## 1. Line 指令集

### 支持指令
- `line.delete`
- `line.replace`
- `line.insert`
- `line.append`

### 参数
- `lineno`：整型，直接按行号定位。  
- `keys`：宽松 AND 匹配关键字（忽略大小写、缩进）。  
- `nthl`：当 keys 命中多处时，取第 n 个（默认 1）。  
- `offset`：相对定位，`+1/-1`，只在 **单点模式**下允许。  
- `start-keys` / `end-keys`：范围模式参数。  
  - `start-keys` 必须唯一命中（或用 `nthl` 选择）。  
  - `end-keys` 命中规则：先宽松唯一 → 否则尝试第一个关键字强匹配唯一 → 否则多处取第一个 → 无法命中报错。  
  - `end-keys` 缺省时，范围延伸到 EOF。  
- **冲突规则**：  
  - `lineno` 与 `offset` 互斥。  
  - 范围模式下：`lineno` 解释为范围内的相对行号。  
  - 有 `start-keys` 时必须配对 `keys` 或 `lineno`，否则报错。  

### 行为规则
- **delete**：删除目标行。  
- **replace**：替换目标行（正文可多行）。  
- **insert**：在目标行前插入（正文可多行）。  
- **append**：在目标行后插入（正文可多行）。  
- 定位失败报错，不允许静默跳过。  
- 默认自动补全末尾换行。  

---

## 2. Block 指令集

### 支持指令
- `block.delete`
- `block.replace`

### 参数
- `start-keys` / `end-keys`：必需，用于范围定位。  
  - `start-keys` 必须唯一命中（或用 `nthb` 指定）。  
  - `end-keys` 遵循与 line 一致的命中规则。  
  - `end-keys` 缺省时，范围延伸到 EOF。  
- `nthb`：选择第几个 start 命中（默认 1）。  
- **不支持**：`lineno`、`keys`、`offset`。  

### 行为规则
- **delete**：整体删除范围内的所有行。  
- **replace**：整体替换范围内的所有行，正文可多行。  
- 范围必须合法（`end >= start`）。  

---

## 3. 匹配规则

- **宽松 AND 匹配**：  
  - 行首缩进忽略。  
  - 不区分大小写。  
  - 多关键字需全部包含。  
- **唯一命中目标**：  
  - start-keys 必须唯一命中（nth 可辅助）。  
  - end-keys 宽松唯一 → 强匹配唯一 → 多处取第一个 → 否则报错。  
- **nthl / nthb**：  
  - line 用 `nthl`，block 用 `nthb`，避免歧义。  

---

## 4. 错误处理

- 定位失败 → 报错并停止该指令。  
- 命中多处且未指定 nth → 报错。  
- 越界操作（lineno > 文件行数）→ 报错。  
- 不允许无效操作（无 allow_noop 机制）。  

---

## 5. 默认行为

- 自动补全文件末尾换行。  
- 每条指令执行完立即 `git add`。  
- 严格幂等：再次执行同一补丁结果应一致。  
