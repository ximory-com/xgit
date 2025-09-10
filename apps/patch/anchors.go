package main

// XGIT:BEGIN IMPORTS
// 说明：锚点样式（HTML/CSS/Go/通用）
import (
	"path/filepath"
	"strings"
	"fmt"
)
// XGIT:END IMPORTS

// XGIT:BEGIN ANCHOR_STYLE
// 说明：按扩展决定 begin/end 样式
func BeginEndMarkers(path, name string) (string, string) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm", ".jsx", ".tsx":
		return fmt.Sprintf("<!-- XGIT:BEGIN %s -->", name), fmt.Sprintf("<!-- XGIT:END %s -->", name)
	case ".css", ".scss":
		return fmt.Sprintf("/* XGIT:BEGIN %s */", name), fmt.Sprintf("/* XGIT:END %s */", name)
	case ".go":
		return fmt.Sprintf("// XGIT:BEGIN %s", name), fmt.Sprintf("// XGIT:END %s", name)
	default:
		return fmt.Sprintf("# XGIT:BEGIN %s", name), fmt.Sprintf("# XGIT:END %s", name)
	}
}
// XGIT:END ANCHOR_STYLE
