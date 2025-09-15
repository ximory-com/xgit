# Git Diff 规范与测试用例

本文档定义了 **XGit Patch 系统**中的 `git.diff` 使用规范、约定模式与规则，并提供了一整套 **标准化测试补丁**，用于验证实现的正确性与稳定性。

---

## 一、设计目标

- 以 **最小可靠集** 实现 `git.diff` 应用：
  - 新建 (A-only)
  - 修改 (M-only)
  - 改名 (R-only)
  - 删除 (D-only)
  - 权限变更 (Mode-only)
- 严格区分不同操作类型，避免逻辑混淆。
- 保证 **落盘一致性**（内容、行数、哈希）和 **操作可回溯性**（日志、提交）。
- 遇到复杂或不常见的组合操作时，直接禁止，要求拆分为多个补丁。

---

## 二、补丁包裹格式

每个补丁需包含以下头部字段：

```
repo: <逻辑名>
commitmsg: <提交说明>
author: <作者>
```

补丁内容以围栏分隔：

```
=== git.diff: "" ===
... 标准 diff 内容 ...
=== end ===

=== PATCH EOF ===
```

### 示例头部

```
repo: xgit
commitmsg: test(gitdiff): add smoke files
author: XGit Bot <bot@xgit.local>
```

---

## 三、操作类别与规则

### 1. A-only（新增文件）
- `new file mode 100644`
- `--- /dev/null`
- `+++ b/<path>`
- 可直接携带内容 hunk（即新文件的完整内容）。

### 2. M-only（修改文件）
- `diff --git a/<path> b/<path>`
- `--- a/<path>`
- `+++ b/<path>`
- 包含一个或多个 `@@` 内容块。

### 3. R-only（改名文件）
- `similarity index 100%`
- `rename from <old>`
- `rename to <new>`
- **禁止携带内容 hunk**，否则需拆分为 R-only + M-only。

### 4. D-only（删除文件）
- `deleted file mode 100644`
- `--- a/<path>`
- `+++ /dev/null`
- 补丁中可能包含原文件内容，但应用时只执行 `git rm -f`。

### 5. Mode-only（权限变更）
- `old mode 100644`
- `new mode 100755`
- 不涉及内容修改。

### 6. 禁止的组合
- **R+M**（改名 + 修改内容） → 必须拆分为两步。  
- **C+M**（复制 + 修改内容） → 不支持。  
- **Submodule / symlink** → 不支持。  
- 其它不常见组合 → 不支持。

---

## 四、日志与校验机制

- 打印补丁大小、行数、哈希。  
- 每类操作输出明确标记：  
  - `✅ 新建 <file>`  
  - `🗑️ 删除 <file>`  
  - `🔧 改名 <from> → <to>`  
  - `✏️ 修改 <file>`  
  - `🔑 权限 <old> → <new>`  
- 新建文件做 **行数校验**（落盘后实际行数必须等于补丁声明的行数）。  
- 删除 / 改名做 **存在性校验**（from 必须存在，to 不得存在）。  
- 所有操作保证提交与推送。

---

## 五、标准测试用例

以下为五大基元操作的标准补丁示例，可直接复制使用。

### A-only（新建）
```
repo: xgit
commitmsg: test(gitdiff): add new test file (A-only)
author: XGit Bot <bot@xgit.local>

=== git.diff: "" ===
diff --git a/tests/add/new_add.txt b/tests/add/new_add.txt
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/tests/add/new_add.txt
@@ -0,0 +3 @@
+line 1
+line 2
+line 3
=== end ===

=== PATCH EOF ===
```

### M-only（修改）
```
repo: xgit
commitmsg: test(gitdiff): modify foo_mix.go (M-only)
author: XGit Bot <bot@xgit.local>

=== git.diff: "" ===
diff --git a/tests/foo/foo_mix.go b/tests/foo/foo_mix.go
index 1111111..2222222 100644
--- a/tests/foo/foo_mix.go
+++ b/tests/foo/foo_mix.go
@@ -1,5 +1,6 @@
 package main

-// foo_mix.go original
-func HelloMix() string {
-    return "hi"
+// foo_mix.go modified
+func HelloMix() string {
+    return "hi-modified"
 }
+// appended line
=== end ===

=== PATCH EOF ===
```

### R-only（改名）
```
repo: xgit
commitmsg: test(gitdiff): rename new_add.txt → renamed_add.txt (R-only)
author: XGit Bot <bot@xgit.local>

=== git.diff: "" ===
diff --git a/tests/add/new_add.txt b/tests/add/renamed_add.txt
similarity index 100%
rename from tests/add/new_add.txt
rename to tests/add/renamed_add.txt
=== end ===

=== PATCH EOF ===
```

### D-only（删除）
```
repo: xgit
commitmsg: test(gitdiff): delete renamed_add.txt (D-only)
author: XGit Bot <bot@xgit.local>

=== git.diff: "" ===
diff --git a/tests/add/renamed_add.txt b/tests/add/renamed_add.txt
deleted file mode 100644
index 2222222..0000000
--- a/tests/add/renamed_add.txt
+++ /dev/null
@@ -1,3 +0,0 @@
-line 1
-line 2
-line 3
=== end ===

=== PATCH EOF ===
```

### Mode-only（权限变更）
```
repo: xgit
commitmsg: test(gitdiff): change foo_mix.go mode 644 → 755 (Mode-only)
author: XGit Bot <bot@xgit.local>

=== git.diff: "" ===
diff --git a/tests/foo/foo_mix.go b/tests/foo/foo_mix.go
old mode 100644
new mode 100755
=== end ===

=== PATCH EOF ===
```

---

## 六、使用方法

1. 将上述任意补丁保存为 `.patch` 文件；  
2. 使用 `ApplyOnce` 执行（会自动做事务提交与推送）；  
3. 验证日志输出与实际文件状态；  
4. 推荐跑一轮全集合回归：A → M → R → D → Mode。

---

✅ 本文档即为 `git.diff` 的 **约定规范 + 测试套件**，保证后续改动均可快速回归验证。
