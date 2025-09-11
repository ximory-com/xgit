/*
XGIT FileOps: 通用文本工具（换行规范化等）
*/
// XGIT:BEGIN GO:PACKAGE
package main
// XGIT:END GO:PACKAGE

// XGIT:BEGIN GO:IMPORTS
import "strings"
// XGIT:END GO:IMPORTS

// XGIT:BEGIN GO:FUNC_TEXT_UTILS
func normalizeLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
func toCRLF(s string) string {
	s = normalizeLF(s)
	return strings.ReplaceAll(s, "\n", "\r\n")
}
// XGIT:END GO:FUNC_TEXT_UTILS
