# 使用 git.diff 进行最小改动协作（Collab 手册）

> 目标：让人机（你 ↔ 我）通过 **最小 diff** 高效、可回溯地协作改写代码。

---

## 一、协作原则（TL;DR）
- **最小改动**：一次补丁只做必要修改，避免夹带与主题无关的改动（格式化/重排等）。
- **类型单一**：一份补丁只包含 **M-only（修改）**；新增/删除/改名请分开各自提交。
- **限定作用域**：尽可能只改一个子目录或少量文件，降低冲突面。
- **可复现**：所有补丁都能在同一基线上重复应用；必要时在提交信息里注明基线 commit。

---

## 二、标准协作流程

### 1. 发起需求（你）
- 用自然语言描述改动点：文件、函数、期望行为、约束（性能/兼容性/安全等）。
- 贴必要上下文（接口定义、传参与返回、现有日志/错误、复现场景）。

> 示例：
> - “请在 `tests/tool/app.go` 的 `main()` 里追加一行日志，打印 `build: <commit>`。”
> - “`pkg/auth/jwt.go` 的 `Validate()` 对过期时间判断不严，修成与 `leeway` 逻辑一致（保留 2s 余量）。”

### 2. 生成最小补丁（我）
- 只给出 **M-only** 的 `git.diff`（标准包裹），不混入 A/D/R。
- 如果必须新增/删除/改名，会单独回你 A-only / D-only / R-only 补丁。

### 3. 你本地应用（ApplyOnce）
- 将补丁文本保存为 `.patch` 文件，执行 `ApplyOnce`。
- 成功后会自动完成：检查 → 应用 → 提交 → 推送。

### 4. 验证与反馈
- 编译/跑测，确认行为达标。
- 如需继续迭代，再发起下一轮 M-only 补丁。

---

## 三、补丁模板（M-only）

```text
repo: xgit
commitmsg: <feat|fix|refactor|docs|chore>(<scope>): <subject>
author: <Your Bot> <bot@xgit.local>

=== git.diff: "" ===
diff --git a/<path> b/<path>
index 1111111..2222222 100644
--- a/<path>
+++ b/<path>
@@ -<start>,<n> +<start>,<m> @@
-<old line 1>
-<old line 2>
+<new line 1>
+<new line 2>
=== end ===

=== PATCH EOF ===
```

> 生成建议：用 `git add -N <file>` 然后 `git diff -- <file>`，只导出目标文件的变更，确保是“最小差异”。

---

## 四、提交信息规范（建议）

```
<type>(<scope>): <subject>

<body 可选，说明动机、要点、风险与测试>
```

- `type`：feat / fix / refactor / perf / docs / chore / test / build / ci
- `scope`：模块或目录名，如 `tests`, `pkg/auth`, `gitdiff`
- `subject`：一句话概括改动，中文或英文均可
- **示例**：
  - `fix(auth): tolerate 2s exp leeway in JWT validation`
  - `refactor(tests): split foo suite per A/M/R/D rules`
  - `chore(gitdiff): tighten M-only guard and logs`

---

## 五、Do & Don’t

### ✅ Do
- 保持 **最小修改**（diff 只含必要行）。
- **一次一个主题**（避免综合性补丁）。
- 提交信息明确、可检索。
- 对公共 API 改动，补充最小单测或注释。

### ❌ Don’t
- 不要把 **A/M/D/R** 混在一个补丁里。
- 不要在 M-only 中做重命名（已禁用 R+M）。
- 不要大范围格式化/重排，除非这是补丁主题。

---

## 六、常见问题（FAQ）

**Q1: 应用失败，提示 “hunk failed / while searching for …”**  
A: 你的工作区不是补丁的基线。先 `git pull --rebase`，或让我基于你当前 `HEAD` 重发补丁。

**Q2: 末行无换行 \\ No newline at end of file**  
A: 允许。我们的 `sanitizeDiff` 与 `git apply --recount` 能正确处理。

**Q3: 我只想看改了哪几行？**  
A: 日志已经打印文件粒度；需要更细粒度时可临时开启 “预检失败行上下文（±N）”模式。

**Q4: 可以一次改很多文件吗？**  
A: 可以，但仍建议按主题分块（如“日志增强一批”、“错误处理一批”），避免巨型提交。

---

## 七、最佳实践清单

- [ ] 只有 **M-only**（必要时 A/D/R 分别单提）。  
- [ ] 补丁最小、主题单一。  
- [ ] 提交信息包含动机与验证方式。  
- [ ] 编译与基本测试通过。  
- [ ] 若涉及接口/行为变化，更新相关文档或注释。

---

## 八、示例：从需求到补丁

**需求**：在 `tests/tool/app.go` 的 `main()` 里追加构建号日志。

**补丁**：
```text
repo: xgit
commitmsg: feat(tests): print build commit in tool/app.go
author: XGit Bot <bot@xgit.local>

=== git.diff: "" ===
diff --git a/tests/tool/app.go b/tests/tool/app.go
index 1111111..2222222 100644
--- a/tests/tool/app.go
+++ b/tests/tool/app.go
@@ -5,3 +5,4 @@ import "fmt"
 func main() {
     fmt.Println("tool modified and ready")
+    fmt.Println("build:", "<commit>")
 }
=== end ===

=== PATCH EOF ===
```

---

## 九、落地与回归

- 本手册与规范文件：`docs/gitdiff_spec_and_tests.md`  
- 回归建议：每次合并前跑一轮 **A → M → R → D → Mode** 的最小集；长期看可接入 CI。

> 有新的协作需求时，直接在会话里描述即可——我会回你 **M-only 最小补丁**，你拿去 ApplyOnce 就行。
