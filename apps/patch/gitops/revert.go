package gitops

import (
	"errors"
	"fmt"
	"strings"
)

// XGIT:BEGIN GITOPS REVERT
// Revert 撤销指定提交的更改（真正的git revert功能）
func Revert(repo, ref string, noCommit bool, logger DualLogger) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return errors.New("git.revert: 缺少要撤销的提交 ref")
	}

	args := []string{"revert"}
	if noCommit {
		args = append(args, "--no-commit") // 不自动提交，仅应用更改到暂存区
	}
	args = append(args, ref)

	if logger != nil {
		if noCommit {
			logger.Log("↩️ git.revert: 撤销提交 %s（不自动提交）", ref)
		} else {
			logger.Log("↩️ git.revert: 撤销提交 %s（自动创建新提交）", ref)
		}
	}

	if _, err := runGit(repo, logger, args...); err != nil {
		return fmt.Errorf("git.revert 执行失败：%w", err)
	}

	if logger != nil {
		logger.Log("✅ git.revert 完成：已撤销提交 %s", ref)
	}
	return nil
}

// XGIT:END GITOPS REVERT
