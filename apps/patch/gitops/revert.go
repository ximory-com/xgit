package gitops

import (
	"errors"
	"fmt"
	"strings"
)

// XGIT:BEGIN GITOPS REVERT
// 将仓库重置到指定提交（语义同 `git reset`，非“反向提交”的 git revert）。
func Revert(repo, ref, mode string, logger DualLogger) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return errors.New("git.revert: 缺少 ref（如 HEAD~1 或提交 SHA）")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "hard"
	}
	var flag string
	switch mode {
	case "hard":
		flag = "--hard"
	case "mixed":
		flag = "--mixed"
	case "soft":
		flag = "--soft"
	default:
		return fmt.Errorf("git.revert: 不支持的 mode=%q（hard|mixed|soft）", mode)
	}

	if logger != nil {
		logger.Log("↩️  git.revert 到 %s（%s）", ref, mode)
	}
	if _, err := runGit(repo, logger, "reset", flag, ref); err != nil {
		return fmt.Errorf("git.revert 失败：%w", err)
	}
	if logger != nil {
		logger.Log("✅ git.revert 完成：%s（%s）", ref, mode)
	}
	return nil
}
// XGIT:END GITOPS REVERT
