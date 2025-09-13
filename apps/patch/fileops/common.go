package fileops

import (
	"strings"
)
// 仅声明所需能力；主包里的 DualLogger 已实现 Log(...)，能自动满足此接口
type DualLogger interface {
	Log(format string, a ...any)
}

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