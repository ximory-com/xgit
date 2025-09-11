# XGIT Anchor Rules v1.2
(本规范用于约束与指导在多语言代码库中埋设、查找、替换锚点，以支持 XGIT 补丁协议的 block 操作)

— 目标 —
1) 一致：不同语言/文件结构下锚点命名和注释风格一致可预期
2) 可追踪：函数/过程级别必须有锚点，便于最小粒度变更
3) 可修复：提供“合规性体检”与“自动修复”规则，把历史/第三方文件纳入体系

────────────────────────────────────────────────
0. 注释/语法风格（按扩展名自动选择）
- Go(.go)            // XGIT:BEGIN NAME  …  // XGIT:END NAME
- HTML(.html,.htm,.jsx,.tsx)  <!-- XGIT:BEGIN NAME --> … <!-- XGIT:END NAME -->
- CSS(.css,.scss)    /* XGIT:BEGIN NAME */ … /* XGIT:END NAME */
- 其他文本/配置(.md,.yml,.yaml,.toml,.ini,.conf,.env,.gitignore,Makefile,Dockerfile 等)
                     # XGIT:BEGIN NAME      …  # XGIT:END NAME
- 备注：NAME 仅允许 [A-Za-z0-9_-]；对大小写不敏感匹配，但存储时保持原样

────────────────────────────────────────────────
1. 文件级锚点（建议）
- 用于给文件关键结构分区：FILE_HEADER, IMPORTS, CONSTS, VARS, TYPES, MODULES, GLOBALS, etc.
- 命名建议（可按语言裁剪）：
  FILE_HEADER     文件头部（版权、包/模块声明、顶级注释）
  IMPORTS         依赖/导入区（强烈建议存在）
  CONSTS/VARS     常量/变量区（可选）
  TYPES           类型/结构体/类定义区（可选）

示例（Go）
  // XGIT:BEGIN FILE_HEADER
  // Package patch provides the patch watcher/runner.
  package patch
  // XGIT:END FILE_HEADER

  // XGIT:BEGIN IMPORTS
  import (
      "bufio"
      "bytes"
      "os"
  )
  // XGIT:END IMPORTS

────────────────────────────────────────────────
2. Imports 区锚点（强烈建议）
- 必须与语言语法相容，位置在导入声明的紧邻处；允许围住整个 import 块。
- 命名固定为：IMPORTS
- 若文件无 import 段（如纯函数工具文件），可省略；但若存在 import，就必须埋锚点。

示例（HTML，脚本块按子区做更细锚点，见第4节）
  <!-- XGIT:BEGIN IMPORTS -->
  <link rel="preconnect" href="https://fonts.gstatic.com">
  <!-- XGIT:END IMPORTS -->

────────────────────────────────────────────────
3. 函数 / 过程 / 方法 / 类 锚点（强制）
- 任何可独立演进的实体都需要独立锚点：
  Go/C/TS/JS 函数：     FUNC_<Name>
  类/结构体/接口：      CLASS_<Name> / TYPE_<Name>（按语言习惯二选一）
  Python 函数/类：      FUNC_<name> / CLASS_<Name>
- 命名中 <Name> 为源码标识，尽量与声明保持一致（区分大小写随语言）。
- 锚点覆盖范围：从声明行开始，到实体“自然结束”行（函数右花括号、类体结尾等）。

示例（Go）
  // XGIT:BEGIN FUNC_ApplyOnce
  func ApplyOnce(repo string, p *Patch, logger *DualLogger) error {
      // ...
      return nil
  }
  // XGIT:END FUNC_ApplyOnce

示例（Python）
  # XGIT:BEGIN FUNC_apply_once
  def apply_once(repo: str, patch: Patch, log: Logger) -> None:
      pass
  # XGIT:END FUNC_apply_once

示例（TypeScript 类）
  // XGIT:BEGIN CLASS_PatchRunner
  export class PatchRunner {
    // ...
  }
  // XGIT:END CLASS_PatchRunner

────────────────────────────────────────────────
4. HTML / CSS 细化建议
HTML：
- 页面结构建议锚：HTML_HEAD, HTML_BODY
- 脚本/样式块建议按用途细分：
  SCRIPT_main, SCRIPT_runtime, STYLE_page, STYLE_buttons 等
- 组件/片段可用组件名：COMP_<Name>，如 COMP_RepoCard

示例（HTML 脚本块）
  <!-- XGIT:BEGIN SCRIPT_main -->
  <script>
    function hello(n){ alert("Hi " + n); }
  </script>
  <!-- XGIT:END SCRIPT_main -->

CSS：
- 建议以选择器组为维度：CLASS_btnPrimary、ID_appHeader、MOD_card、UTIL_hidden
示例（CSS）
  /* XGIT:BEGIN CLASS_button */
  .button { padding: .5rem 1rem; border-radius: .375rem; }
  /* XGIT:END CLASS_button */

────────────────────────────────────────────────
5. JSON / YAML / XML（数据/配置）
- 允许锚点覆盖全文或关键节点（以“逻辑段”为单位），例如：
  JSON：SECTION_build, SECTION_routes
  YAML：SECTION_vercel, SECTION_workflow
  XML：SECTION_manifest, SECTION_strings

示例（YAML）
  # XGIT:BEGIN SECTION_vercel
  buildCommand: "pnpm build"
  outputDirectory: "dist"
  # XGIT:END SECTION_vercel

────────────────────────────────────────────────
6. @index 与 append_once
- 当同名锚点可能出现多次（例如重复的组件样板），使用 @index=1,2,... 精确命中。
- append_once：对目标锚体做“去尾空白”归一化后包含判断，若已包含，不重复追加。

────────────────────────────────────────────────
7. 合规性体检（Compliance Check）与自动修复
7.1 体检入口
- 针对任意文件，按扩展名推断注释风格，扫描以下规则：
  a) 导入区存在时，应存在 IMPORTS 锚点
  b) 发现函数/类/方法定义，必须存在对应 FUNC_/CLASS_/TYPE_ 锚点
  c) BEGIN/END 必须成对、嵌套正确；若发现孤立 BEGIN/END 需修复
  d) 同名锚点允许多对，但必须成对；多对时建议索引唯一化（记录出现顺序）

7.2 自动修复策略（建议实现）
- 缺失 IMPORTS：在导入块外层包裹 IMPORTS 锚对
- 缺失函数/类锚：以声明行为起点，自动包裹到语法块结束（大括号或缩进块结束）
- 孤立 BEGIN：若紧随文件末尾，自动补写对应 END（记录日志）
- 锚名非法字符：以 '-' 替换空格；剔除非 [A-Za-z0-9_-] 字符；大小写保持原样
- 嵌套错位：保守回退为最近合法配对方式，并记录警告

────────────────────────────────────────────────
8. 命名与大小写约定
- NAME 允许 [A-Za-z0-9_-]，推荐使用 UpperCamel（Go/TS/JS 类/导出项）、lower_snake（Python）
- 语言建议：
  Go：FUNC_ApplyOnce, CLASS_PatchRunner, TYPE_Patch
  Python：FUNC_apply_once, CLASS_PatchRunner
  TS/JS：FUNC_applyOnce, CLASS_PatchRunner
  HTML/CSS：SCRIPT_main, STYLE_page, CLASS_button

────────────────────────────────────────────────
9. 最佳实践
- 新文件优先预埋：FILE_HEADER / IMPORTS / TYPES / 每个函数
- 组件型前端文件：按“组件/片段”为单位埋锚（便于重排/替换）
- 长函数：内部可再细分次级锚（如 FUNC_X#STEP_parse_header），但次级锚用于阅读，不用于跨文件 block 修改

────────────────────────────────────────────────
10. 与 XGIT 补丁 block 的关系
- block 格式：=== block: <path>#<ANCHOR>[@index=N] mode=(replace|append|prepend|append_once) ===
- 建议优先对“函数级锚”做 replace；对配置段落做 append_once
- 允许同名锚多对，配合 @index 精确定位；若未指定 index，默认命中第 1 对

────────────────────────────────────────────────
11. 版本管理
- v1.2（当前）：函数级锚点强制；HTML/CSS 细化；合规体检规则；跨语言命名建议
- 兼容：v1.0/v1.1 的锚点仍然有效；建议逐步迁移到函数/过程强制锚点

（完）

