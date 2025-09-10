package main

// XGIT:BEGIN IMPORTS
// 说明：apply 流程（clean → files → blocks → commit & push）
import (
	"strings"
	"time"
)

// XGIT:END IMPORTS

// XGIT:BEGIN APPLY
// 说明：执行一次补丁；无改动则直接返回
func applyOnce(logger *DualLogger, repo string, p *Patch) {
	logger.Log("▶ 开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))
	logger.Log("ℹ️ 仓库：%s", repo)

	// 清理（auto）
	logger.Log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = Shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = Shell("git", "-C", repo, "clean", "-fd")

	// 写文件
	for _, f := range p.Files {
		if err := WriteFileAndStage(repo, f.Path, f.Content, logger.Log); err != nil {
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

	// 检查缓存区
	names, _, _ := Shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.Log("ℹ️ 无改动需要提交。")
		logger.Log("✅ 本次补丁完成")
		return
	}

	commit := p.Commit
	if strings.TrimSpace(commit) == "" {
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
// 可追加提交前校验、lint、或 PR 工作流（将来可扩展）

// XGIT:END APPLY
