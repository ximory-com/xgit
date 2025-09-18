package gitops

import (
	"errors"
	"fmt"
	"strings"
)

// XGIT:BEGIN GITOPS RESET
// Reset 将仓库重置到指定提交状态（原错误命名为Revert的功能）
func Reset(repo, ref, mode string, logger DualLogger) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return errors.New("git.reset: 缺少目标提交 ref（如 HEAD~1 或提交 SHA）")
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "hard" // 默认硬重置，完全回到指定状态
	}

	var flag string
	switch mode {
	case "hard":
		flag = "--hard" // 重置HEAD、暂存区和工作目录
	case "mixed":
		flag = "--mixed" // 重置HEAD和暂存区，保留工作目录更改
	case "soft":
		flag = "--soft" // 仅重置HEAD，保留暂存区和工作目录
	default:
		return fmt.Errorf("git.reset: 不支持的 mode=%q（支持：hard|mixed|soft）", mode)
	}

	if logger != nil {
		logger.Log("🔄 git.reset: 重置到 %s（模式：%s）", ref, mode)
	}

	if _, err := runGit(repo, logger, "reset", flag, ref); err != nil {
		return fmt.Errorf("git.reset 执行失败：%w", err)
	}

	if logger != nil {
		logger.Log("✅ git.reset 完成：仓库已重置到 %s", ref)
	}
	return nil
}

// XGIT:END GITOPS RESET
