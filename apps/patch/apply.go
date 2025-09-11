package main

// 只实现 file 覆盖提交与推送（不依赖 block）。
// 导出：ApplyOnce

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ApplyOnce 在 repo 内应用一次补丁（仅 file），自动 reset/clean、add、commit、push。
func ApplyOnce(logger *DualLogger, repo string, p *Patch) {
	logger.Log("▶ 开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))
	logger.Log("ℹ️ 仓库：%s", repo)

	// 自动清理（等价 REQUIRE_CLEAN=auto）
	logger.Log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = Shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = Shell("git", "-C", repo, "clean", "-fd")

	// 写入文件（覆盖）
	for _, fc := range p.Files {
		if err := applyWriteFile(repo, fc.Path, fc.Content, logger); err != nil {
			logger.Log("❌ 写入失败：%s (%v)", fc.Path, err)
			return
		}
	}

	// 若没有任何改动（看缓存区）
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

// —— 内部辅助（避免与其他文件重名冲突，前缀 apply*） ——

func applyStage(repo, rel string, logger *DualLogger) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return
	}
	if _, _, err := Shell("git", "-C", repo, "add", "--", rel); err != nil {
		logger.Log("⚠️ 自动加入暂存失败：%s", rel)
	} else {
		logger.Log("🧮 已加入暂存：%s", rel)
	}
}

func applyWriteFile(repo, rel, content string, logger *DualLogger) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return err
	}
	// 统一 LF；保证末尾换行
	content = strings.ReplaceAll(content, "\r", "")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return err
	}
	logger.Log("✅ 写入文件：%s", rel)
	applyStage(repo, rel, logger)
	return nil
}
