# XGit Web 可替换块 ID 约定

> 用于 `=== block: <path> :: <id> ===` 精准替换：
- apps/web/index.html :: hero        （顶部欢迎/登录区）
- apps/web/index.html :: toolbar     （导航操作区：返回官网、GitHub、语言切换等）
- apps/web/index.html :: repoList    （仓库列表卡片区域）
- apps/web/index.html :: repoDetail  （仓库详情/README 预览区）

埋点规则：
- 在文件内使用 HTML 注释：
  <!-- XGIT:BEGIN <id> --> ... <!-- XGIT:END <id> -->
- 若首次替换找不到该 <id>，脚本会将整个块 **追加到文件末尾**（不影响现有渲染）。
- 后续再发同 ID 的 block 补丁，即可原位替换。

