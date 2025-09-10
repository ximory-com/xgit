package main

import (
	"strings"
	"time"
)

// 说明：applyOnce 执行一次补丁的完整事务：清理→写文件/区块→提交→推送。
// 注意：此文件仅修复调用点以匹配多文件拆分后的导出符号：DualLogger.Log / Shell / WriteFile。
func applyOnce(logger *DualLogger, repo string, p *Patch) {
	logger.Log("▶ 开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))
	logger.Log("ℹ️ 仓库：%s", repo)

	// 清理（auto）
	logger.Log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = Shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = Shell("git", "-C", repo, "clean", "-fd")

	// 写文件（写入后会统一加入暂存）
	for _, f := range p.Files {
		if err := WriteFile(repo, f.Path, f.Content, logger.Log); err != nil {
			logger.Log("❌ 写入失败：%s (%v)", f.Path, err)
			return
		}
	}

	// 区块（命中锚区后会自动加入暂存）
	for _, b := range p.Blocks {
		if err := applyBlock(repo, b, logger.Log); err != nil {
			logger.Log("❌ 区块失败：%s #%s (%v)", b.Path, b.Anchor, err)
			return
		}
	}

	// 若无改动则直接结束
	names, _, _ := Shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.Log("ℹ️ 无改动需要提交。")
		logger.Log("✅ 本次补丁完成")
		return
	}

	// 提交 & 推送
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
