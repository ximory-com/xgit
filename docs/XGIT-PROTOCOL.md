# XGit 指令集协议 (终稿)

本协议定义 **XGit 补丁系统**的完整指令集、书写规范与执行规则。  
指令共分为四大类：file、line、block、git，总计 **19 个指令**。

---

## 📑 导航索引

- [总体规范](#总体规范)
- [书写规范](#书写规范)
- [参数说明](#参数说明)
- [指令集索引](#指令集索引)
  - [file.* 指令 (9 个)](#file-指令集)
  - [line.* 指令 (4 个)](#line-指令集)
  - [block.* 指令 (2 个)](#block-指令集)
  - [git.* 指令 (4 个)](#git-指令集)
- [错误处理规范](#错误处理规范)
- [示例](#示例)

---

## 总体规范

1. **补丁文件结构**
   - 文件头包含 `repo`、`commitmsg`、`author` 等元信息。
   - 一个补丁文件包含 **1 个或多个指令片段**。
   - 补丁以 `=== PATCH EOF ===` 结尾。

2. **事务执行**
   - 一个补丁整体作为事务执行；任意指令失败将触发回滚。  
   - 特殊指令 `git.commit` 不会清理工作区。

3. **预检机制**
   - 文件型指令 (file.*、line.*、block.*) 在执行后会调用 `preflightOne` 进行预检，确保修改合法。  
   - 预检失败即中止并回滚。

---

## 书写规范

1. **参数区与正文区**
   - 参数区紧跟在指令头之后。  
   - 正文区在参数区结束后立即开始。  
   - **两者之间禁止空行**（避免产生无意义的空行插入）。

2. **指令格式**
   ```
   === <指令名>: "<path>" ===
   参数...
   正文内容 (可多行)
   === end ===
   ```

3. **结尾**
   - 所有补丁必须以 `=== PATCH EOF ===` 结尾。

---

## 参数说明

- **lineno**  
  绝对行号，1-based。必须唯一，越界则报错。  
  在区间模式下表示“范围内的第 n 行”。

- **keys**  
  关键字定位。宽松 AND 匹配：行需同时包含所有关键字（忽略前导空白，默认大小写不敏感）。

- **nthl** (line 专用)  
  当 keys 匹配多行时，取第 n 行。缺省时必须唯一命中，否则报错。

- **start-keys / end-keys**  
  范围定位关键字。  
  - start 必须唯一命中（可用 nthb 指定）。  
  - end 宽松匹配：唯一则直接使用，多处则尝试强匹配第一个关键字；若仍多处，取第一个；若完全未命中则失败。  
  - end 缺省则视为 EOF。

- **nthb** (block 专用)  
  当 start-keys 匹配多处时，取第 n 个。

- **offset**  
  行偏移。仅在 **非区间模式**下允许。默认为 0。

---

## 指令集索引

### file 指令集

共 9 个：
- `file.write`
- `file.delete`
- `file.move`
- `file.copy`
- `file.chmod`
- `file.mkdir`
- `file.rmdir`
- `file.symlink`
- `file.touch`

### line 指令集

共 4 个：
- `line.delete`
- `line.replace`
- `line.insert`
- `line.append`

### block 指令集

共 2 个：
- `block.delete`
- `block.replace`

### git 指令集

共 4 个：
- `git.diff` (保留，复杂性高，暂不推荐使用)
- `git.revert`
- `git.tag`
- `git.commit`

---

## 错误处理规范

1. **未知指令** → `❌ 未知指令`
2. **参数缺失** → `❌ <cmd>: 缺少必要参数`
3. **keys 未命中** → 输出样本前后若干行以辅助定位
4. **keys 多处命中** → 提示行号列表，建议增加关键字或使用 lineno/nth
5. **lineno 越界** → 明确提示有效范围
6. **范围越界** → 明确提示 start/end/总行数

---

## 示例

### line.replace

```
repo: xgit
commitmsg: refactor: update title
author: XGit Bot <bot@xgit.local>

=== line.replace: "tests/line/index.html" ===
keys<
<title>Old</title>
>keys
    <title>New Title</title>
=== end ===

=== PATCH EOF ===
```

### block.delete

```
repo: xgit
commitmsg: refactor: remove function
author: XGit Bot <bot@xgit.local>

=== block.delete: "tests/code/sample.go" ===
start-keys<
func OldFunc(
>start-keys
end-keys<
}
>end-keys
=== end ===

=== PATCH EOF ===
```

### git.commit

```
repo: xgit
commitmsg: chore: sync from iCloud
author: XGit Bot <bot@xgit.local>

=== git.commit: "" ===
=== end ===

=== PATCH EOF ===
```

---

✅ 本协议作为权威规范，覆盖 **全部 19 个现有指令**，并明确了参数、规则、错误处理与示例。
