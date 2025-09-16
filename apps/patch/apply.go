package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ApplyOnce：增加 patchFile 参数用于从文件头读取 repo: 兜底（拿不到可传 ""）
func ApplyOnce(logger *DualLogger, repo string, patch *Patch, patchFile string) {
	// 0) 解析真实仓库路径（优先 Patch.Repo，其次补丁头 repo:，最后 .repos 的 default）
	patchDir := "."
	if strings.TrimSpace(patchFile) != "" {
		patchDir = filepath.Dir(patchFile)
	}
	selectedRepo, err := resolveRepoFromPatch(patchDir, patch, patchFile)
	if err != nil {
		if logger != nil {
			logger.Log("❌ 仓库解析失败：%v", err)
		}
		return
	}
	repo = selectedRepo

	// 1) 统一日志：若外部未传，则在补丁同目录创建/覆盖 patch.log
	if logger == nil {
		lg, _ := NewDualLogger(patchDir)
		logger = lg
	}
	log := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}
	logf := func(format string, a ...any) { log(format, a...) }

	// 2) 事务阶段
	err = WithGitTxn(repo, logf, func() error {
		// 1) 先应用所有指令
		for i, op := range patch.Ops {
			tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)
			if e := applyOp(repo, op, logger); e != nil {
				logf("❌ %s 失败：%v", tag, e)
				return e
			}
		}

		// 2) 预检：不再过滤新增，直接对所有改动文件执行预检
		changed, _ := collectChangedFiles(repo) // 仍使用你现有实现
		if len(changed) == 0 {
			// 不在这里打印“无改动需要提交”，交给事务外统一处理
			return nil
		}

		logf("🧪 预检（真实仓库）：%d 个文件", len(changed))
		if err := preflightRun(repo, changed, logger); err != nil {
			logf("❌ 预检失败：%v", err)
			return err
		}
		logf("✅ 预检通过")
		return nil
	})

	if err != nil {
		log("❌ git.diff 事务失败：%v", err)
		return
	}

	// 3) 提交 & 推送
	changed, _ := collectChangedFiles(repo)
	if len(changed) == 0 {
		log("ℹ️ 无改动需要提交。")
		return
	}

	commit := strings.TrimSpace(patch.CommitMsg)
	if commit == "" {
		commit = "chore: apply file ops patch"
	}
	author := strings.TrimSpace(patch.Author)
	if author == "" {
		author = "XGit Bot <bot@xgit.local>"
	}

	log("ℹ️ 提交说明：%s", commit)
	log("ℹ️ 提交作者：%s", author)

	_ = runCmd("git", "-C", repo, "commit", "--author", author, "-m", commit)
	log("✅ 已提交：%s", commit)

	log("🚀 正在推送（origin HEAD）…")
	if _, err := runCmdOut("git", "-C", repo, "push", "origin", "HEAD"); err != nil {
		log("❌ 推送失败：%v", err)
	} else {
		log("🚀 推送完成")
	}
	log("✅ 本次补丁完成")
}

// 统一仓库解析：Patch.Repo > 头部 repo: > .repos default
func resolveRepoFromPatch(patchDir string, patch *Patch, patchFile string) (string, error) {
	baseDir := patchDir
	repos, def := LoadRepos(baseDir)

	target := strings.TrimSpace(patch.Repo)
	if target == "" {
		target = HeaderRepoName(patchFile)
		if target == "" {
			target = def
		}
	}
	if target == "" {
		return "", fmt.Errorf("无法解析目标仓库（Patch.Repo/头部 repo:/.repos default 皆为空）")
	}
	real := repos[target]
	if real == "" {
		return "", fmt.Errorf("repo 映射缺失：%s", target)
	}
	return real, nil
}
