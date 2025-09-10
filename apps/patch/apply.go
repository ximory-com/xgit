// Package patch: one-shot apply (clean -> files -> blocks -> commit/push)
// XGIT:BEGIN APPLY_HEADER
package main

import (
	"path/filepath"
	"strings"
	"time"
)

// XGIT:END APPLY_HEADER

// XGIT:BEGIN APPLY
func applyOnce(logger *DualLogger, repo string, p *Patch) {
	logger.Log("▶ 开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))
	logger.Log("ℹ️ 仓库：%s", repo)

	// 清理（auto）
	logger.Log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = Shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = Shell("git", "-C", repo, "clean", "-fd")

	// 删除（优先 git rm，不在索引里则物理删）
	for _, rel := range p.Deletes {
		abs := filepath.Join(repo, rel)
		if _, _, err := Shell("git", "-C", repo, "rm", "-rf", "--", rel); err != nil {
			_ = RemoveAll(abs)
		}
		logger.Log("🗑️ 删除：%s", rel)
	}

	// 写文件
	for _, f := range p.Files {
		if err := WriteFile(repo, f.Path, f.Content, logger.Log); err != nil {
			logger.Log("❌ 写入失败：%s (%v)", f.Path, err)
			return
		}
	}

	// 区块
	for _, b := range p.Blocks {
		if err := ApplyBlock(repo, b, logger.Log); err != nil {
			logger.Log("❌ 区块失败：%s #%s (%v)", b.Path, b.Anchor, err)
			return
		}
	}

	// 无改动直接返回
	names, _, _ := Shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.Log("ℹ️ 无改动需要提交。")
		logger.Log("✅ 本次补丁完成")
		return
	}

	// 提交 & 推送
	commit := strings.TrimSpace(p.Commit)
	if commit == "" {
		commit = "chore: apply patch"
	}
	author := strings.TrimSpace(p.Author)
	if author == "" {
		author = "XGit Bot <bot@xgit.local>"
	}
	logger.Log("ℹ️ 提交说明：%s", commit)
	logger.Log("ℹ️ 提交作者：%s", author)
	_, _, _ = Shell("git", "-C", repo, "commit", "--author", author, "-m", commit)
	logger.Log("✅ 已提交：%s", commit)

	logger.Log("🚀 正在推送（origin HEAD）…")
	if _, er, err := Shell("git", "-C", repo, "push", "origin", "HEAD"); err != nil {
		logger.Log("❌ 推送失败：%s", er)
	} else {
		logger.Log("🚀 推送完成")
	}
	logger.Log("✅ 本次补丁完成")
}
// XGIT:END APPLY
