# Go 语言锚点规范（XGIT 专用）

目的：让 Go 源码在自动补丁时可精确定位、可替换/追加/删除、可分区维护；避免整文件“豆腐块式”覆盖。

---

## 1. 文件级锚点（必须）

1) 包头锚点
// XGIT:BEGIN PACKAGE
package yourpkg
// XGIT:END PACKAGE

2) Imports 区锚点
// XGIT:BEGIN IMPORTS
import (
    "fmt"
    "os"
)
// XGIT:END IMPORTS

3) 文件头注释锚点（推荐）
// XGIT:BEGIN FILE_DOC
// 文件说明：功能/边界/维护人/注意事项
// XGIT:END FILE_DOC

规则：
- PACKAGE 与 IMPORTS 必须存在且唯一
- FILE_DOC 可选但推荐

---

## 2. 顶级声明锚点（强制）

1) 常量区
// XGIT:BEGIN CONST
const (
    AppVersion = "1.0.0"
)
// XGIT:END CONST

2) 变量区
// XGIT:BEGIN VAR
var (
    DebugMode = false
)
// XGIT:END VAR

3) 类型区
// XGIT:BEGIN TYPE
type DualLogger struct {
    Console io.Writer
    File    *os.File
}
// XGIT:END TYPE

规则：
- 同一文件建议各一个 CONST/VAR/TYPE 区块
- 如需拆分，用 SECTION 锚点标记

---

## 3. 函数锚点（强制）

每个函数必须独立锚点，便于替换或追加。

// XGIT:BEGIN FUNC_Foo
func Foo() {
    fmt.Println("bar")
}
// XGIT:END FUNC_Foo

命名规范：
- FUNC_前缀 + 函数名
- 同文件函数不可重名

---

## 4. 区段锚点（辅助）

当某文件很长，可人为拆段：

// XGIT:BEGIN SECTION_HTTP
...HTTP 相关函数...
// XGIT:END SECTION_HTTP

// XGIT:BEGIN SECTION_UTIL
...工具函数...
// XGIT:END SECTION_UTIL

---

## 5. 使用规则

- 替换：mode=replace
- 追加：mode=append
- 前置：mode=prepend
- 唯一追加：mode=append_once
- 删除：用空 body + replace

---

## 6. 强制要求

1. 新文件必须含 PACKAGE + IMPORTS 锚点
2. 函数必须有 FUNC_ 锚点
3. 禁止在锚点外写不可控逻辑
4. 规范必须被后续补丁严格遵守
