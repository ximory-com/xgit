# XGit Patch 协议摘要（git.diff）

## 支持的操作类型
- **A-only**：新增文件（可携带完整内容）
- **M-only**：修改文件（一个或多个 hunk）
- **D-only**：删除文件
- **R-only**：改名文件（不带内容修改）
- **Mode-only**：权限变更（如 100644 ↔ 100755）

## 禁止/限制
- **禁止 R+M**：改名与改内容必须拆分为两步补丁
- A 与 M/D/R **互斥**于同一补丁
- 二进制（图片/大文件）暂不在最小集范围（未来按需开启）

## 执行流程（ApplyOnce）
1. 清洗与预检：`sanitizeDiff` → `git apply --check --recount`
2. 结构化处理：删除/改名走 `git rm` / `git mv`，成功后剔除对应块
3. 应用剩余补丁：成功后校验新增文件行数等
4. 提交与推送：`commitmsg/author` 来自补丁头

## 日志规范（示例）
- `✅ 新建  <path>`
- `✏️ 修改  <path>`
- `🗑️ 删除  <path>`
- `🔧 改名  <from> → <to>`
- `🔑 权限  <old> → <new>`

## 参考文档
- `docs/gitdiff_spec_and_tests.md`
- `docs/gitdiff_collab.md`
