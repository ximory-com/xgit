# XGit 文档

- [变更记录](./changelog.md)
- [部署与域名](./vercel.md)
- [目录结构](./structure.md)
- [提交规范](./conventions.md)
- [对话摘录](./journal.md)
- [未来规划](./roadmap.md)

## ⚠️ 重要指引（2025-09）

- **文件写类 `file.*` 指令**（`file.write/append/prepend/replace/delete/move/chmod` 等）已弃用。  
- 请统一通过 **git.diff** 协议执行文件操作，当前支持：
  - A-only（新增）
  - M-only（修改）
  - D-only（删除）
  - R-only（改名，禁止 R+M 组合）
  - Mode-only（权限）
- `line.xxx` / `block.xxx` 不再作为执行指令推进，应在补丁生成层完成定位与替换，最终仍以 `git.diff` 提交。
