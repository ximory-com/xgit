package fileops

import (
	"bytes"
	"fmt"
	"os/exec"
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

// runGit 执行 git 命令（自动 -C repo），返回合并输出（stdout+stderr）
func runGit(repo string, logger DualLogger, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("git %s 失败：%v\n%s", strings.Join(args, " "), err, out.String())
	}
	return out.String(), nil
}

// ensureNL: 判断参数或默认值
func ensureNL(args map[string]string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(args["ensure_nl"]))
	switch v {
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}
	return def
}

// XGIT:END GO:FUNC_TEXT_UTILS
