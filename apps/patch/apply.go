package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// XGIT:BEGIN APPLY
// 说明：一次性应用补丁（先 delete，后 file/block；失败中止）
func applyOnce(logger *DualLogger, repo string, p *Patch) {
	logger.log("▶ 开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))
	logger.log("ℹ️ 仓库：%s", repo)

	// 清理（auto）
	logger.log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = shell("git", "-C", repo, "clean", "-fd")

	// 1) 删除
	for _, d := range p.Deletes {
		rel := strings.TrimSpace(d.Path)
		if rel == "" {
			continue
		}
		abs := filepath.Join(repo, rel)
		// 先尝试 git rm（若已跟踪会直接进入暂存区）
		if _, _, err := shell("git", "-C", repo, "rm", "-rf", "--", rel); err != nil {
			// 不在索引里：物理删除
			_ = os.RemoveAll(abs)
		}
		logger.log("🗑️ 删除：%s", rel)
	}

	// 2) file 写入
	for _, f := range p.Files {
		if err := writeFile(repo, f.Path, f.Content, logger.log); err != nil {
			logger.log("❌ 写入失败：%s (%v)", f.Path, err)
			return
		}
	}

	// 3) block 应用
	for _, b := range p.Blocks {
		if err := applyBlock(repo, b, logger.log); err != nil {
			logger.log("❌ 区块失败：%s #%s (%v)", b.Path, b.Anchor, err)
			return
		}
	}

	// 是否有改动
	names, _, _ := shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.log("ℹ️ 无改动需要提交。")
		logger.log("✅ 本次补丁完成")
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
	logger.log("ℹ️ 提交说明：%s", commit)
	logger.log("ℹ️ 提交作者：%s", author)
	_, _, _ = shell("git", "-C", repo, "commit", "--author", author, "-m", commit)
	logger.log("✅ 已提交：%s", commit)

	logger.log("🚀 正在推送（origin HEAD）…")
	if _, er, err := shell("git", "-C", repo, "push", "origin", "HEAD"); err != nil {
		logger.log("❌ 推送失败：%s", er)
	} else {
		logger.log("🚀 推送完成")
	}
	logger.log("✅ 本次补丁完成")
}
// XGIT:END APPLY
