package gitops

import (
	"errors"
	"fmt"
	"strings"
)

// XGIT:BEGIN GITOPS TAG
// 创建或更新标签。
func Tag(repo, name, ref, message string, force, push bool, logger DualLogger) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("git.tag: 缺少标签名 name")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}

	// 构造命令参数
	var args []string
	if message != "" {
		args = []string{"tag", "-a", name, ref, "-m", message}
	} else {
		args = []string{"tag", name, ref}
	}
	if force {
		args = append(args, "-f")
	}

	if logger != nil {
		if message != "" {
			logger.Log("🏷️  git.tag 附注标签：%s -> %s", name, ref)
		} else {
			logger.Log("🏷️  git.tag 轻量标签：%s -> %s", name, ref)
		}
	}
	if _, err := runGit(repo, logger, args...); err != nil {
		return fmt.Errorf("git.tag 失败：%w", err)
	}
	if logger != nil {
		logger.Log("✅ git.tag 本地创建/更新完成：%s", name)
	}

	if push {
		if logger != nil {
			logger.Log("🚀 推送标签到远端：origin %s", name)
		}
		if _, err := runGit(repo, logger, "push", "origin", name); err != nil {
			return fmt.Errorf("git.tag: 推送标签失败：%w", err)
		}
		if logger != nil {
			logger.Log("✅ 标签推送完成：%s", name)
		}
	}
	return nil
}
// XGIT:END GITOPS TAG
