# XGit 命令协议（统一最终稿）
> 版本：v1.0-final  
> 说明：本文档覆盖 XGit 内部 `file` / `line` / `block` / `git` 四大类指令，共 19 条（file 9、line 4、block 2、git 4）。  
> 目的：形成唯一权威的指令协议，用于补丁编写、patchd/ApplyOnce 实现与 webapp 生成补丁的互操作性。

---

## 目录（导航）
- [总则与补丁包装格式](#总则与补丁包装格式)  
- [日志与 patch.log 行为](#日志与-patchlog-行为)  
- [事务、预检与回滚策略](#事务预检与回滚策略)  
- [指令索引（19 条）](#指令索引19-条)  
  - [file 系列（9）](#file-系列9)  
  - [line 系列（4）](#line-系列4)  
  - [block 系列（2）](#block-系列2)  
  - [git 系列（4）](#git-系列4)  
- [定位（keys / lineno / nth / offset / 区间）精确规则](#定位keys--lineno--nth--offset--区间精确规则)  
- [文本体/参数区格式与语法（严格）](#文本体参数区格式与语法严格)  
- [示例补丁（常用场景）](#示例补丁常用场景)  
- [常见错误与提示消息（建议日志文案）](#常见错误与提示消息建议日志文案)  
- [变更记录 / 兼容性备注](#变更记录--兼容性备注)

---

## 总则与补丁包装格式

### 补丁外壳（强制）
补丁必须被我们定义的包裹格式包裹（单个补丁中可以包含若干按顺序要执行的指令块）：

```
repo: <logical-repo-name>
commitmsg: <commit message>
author: Name <email>

=== <cmd> "<path-or-empty>" [可选 header KV...] ===
<参数区（若有）——以 KV 或多行 keys...，详见下文>
<body（正文区）——若指令需要正文>
=== end ===

...（可以继续多个指令块）
=== PATCH EOF ===
```

- `repo:`：逻辑名，映射 `.repos` 文件中的真实工作区路径；若缺省或空白使用 `default`（或由上层处理）。
- `commitmsg:`、`author:`：为最终提交的默认值（ApplyOnce/commit 阶段使用）。`git.commit` 指令有特殊约束（见下）。
- 每个指令块以 `=== <cmd> "<path>" ===` 开始，并以 `=== end ===` 结束。最后必须有 `=== PATCH EOF ===`（整包结尾）。
- **正文区和参数区之间绝对不能有空行**。
- 路径都必须是仓库相对路径，禁止绝对路径、禁止 `..` 越界。

---

## 日志与 patch.log 行为

- 控制台输出（logger）与磁盘日志 `patch.log` 必须**一致**（控制台输出的内容最后以完整文本写入 `patch.log`），但 `patch.log` 只保存**最近一次**补丁的执行日志（不增量追加）。
- 当 `patch.log` 被误删时，下次补丁执行应**重新创建**并写入本次执行的完整日志。
- 日志中应包含补丁 ID/hash、时间戳、每个指令的成功/失败行、最终提交/推送状态，以及任何回滚信息。

---

## 事务、预检与回滚策略

- **ApplyOnce（整体补丁）**在单一 Git 事务中执行：
  - 在执行前记录当前 HEAD（`preHead`），并清理临时工作区（若事务模式要求）。事务失败则回滚到 `preHead` 并恢复工作区。
  - 提供可选 `opts` 以允许不清理工作区（如 `git.commit` 场景）。
- **预检（preflight）**：
  - 对会修改已有文件内容的指令执行预检。
  - 对新增文件（A）通常跳过覆盖式预检（避免把默认模板覆盖真实新增内容）。
- **回滚**：
  - 任何步骤返回错误，整个补丁回滚，并在日志中输出回滚原因与 preHead。
- **原子性**：单个补丁内的多条指令按顺序执行；发生错误立即停止并回滚；若需要分批次逻辑，请拆补丁。

---

## 指令索引（19 条）

### file 系列（9）
1. file.write
2. file.append
3. file.prepend
4. file.delete
5. file.move
6. file.chmod
7. file.eol
8. file.binary
9. file.image

### line 系列（4）
1. line.insert_before
2. line.insert_after
3. line.replace
4. line.delete_line

### block 系列（2）
1. block.replace
2. block.delete

### git 系列（4）
1. git.diff
2. git.revert
3. git.tag
4. git.commit

---

## 定位（keys / lineno / nth / offset / 区间）精确规则

详见前述说明。

---

## 文本体/参数区格式与语法（严格）

- **正文区与参数区之间必须有且仅有一个空行**。
- `keys< ... >keys` 块内不要包含多余空行。

---

## 示例补丁（常用场景）

（省略部分示例，保持简洁，完整示例见详细文档）

---

## 常见错误与提示消息

- 定位失败：keys 未命中
- 定位失败：多处命中
- git.diff 未能应用 hunk

---

## 变更记录

- 废弃 file.replace / file.diff
- line.replace_line 简化为 line.replace

---
