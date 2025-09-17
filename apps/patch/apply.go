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
	// ---- lineno 约束：最多 1 个，且必须出现在第一个指令 ----
	hasLineNo := false
	for i, op := range patch.Ops {
		if strings.HasPrefix(op.Cmd, "line.") {
			if n := argInt(op.Args, "lineno", 0); n > 0 {
				if !hasLineNo {
					hasLineNo = true
					if i != 0 {
						logf("❌ 非法补丁：带 lineno 的指令必须放在首个指令（当前在 #%d）", i+1)
						return
					}
				} else {
					logf("❌ 非法补丁：同一批次包含多个 lineno 操作，补丁需拆分执行")
					return
				}
			}
		}
	}
	// ---- 约束结束 ----
	// ---- 约束结束 ----
	hasCommit := false
	for i, op := range patch.Ops {
		if op.Cmd == "git.commit" {
			if !hasCommit {
				hasCommit = true
				// git.commit 必须单独成批，且（自然）只能是第 1 条
				if len(patch.Ops) != 1 || i != 0 {
					logf("❌ 非法补丁：git.commit 必须单独使用且作为唯一指令（当前在 #%d，批内共 %d）", i+1, len(patch.Ops))
					return
				}
			} else {
				logf("❌ 非法补丁：同一批次包含多个 git.commit 指令，补丁需拆分执行")
				return
			}
		}
	}
	opts := TxnOpts{
		CleanAtStart:    !hasCommit, // 有 git.commit 就不要清理工作区
		RollbackOnError: true,       // 失败仍然回滚（按你现有策略）
	}
	// 事务阶段
	err = WithGitTxnOpts(repo, logf, opts, func() error {
		// 1) 先应用所有指令
		for i, op := range patch.Ops {
			tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)
			if e := applyOp(repo, op, logger); e != nil {
				logf("❌ %s 失败：%v", tag, e)
				return e
			}
		}
		return nil
	})

	if err != nil {
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
	// === 统一纳入索引 ===
	if _, err := runGit(repo, logger, "add", "-A", "--"); err != nil {
		log("❌ stage 失败：%v", err)
		return
	}

	// === 只看已暂存改动，决定是否提交 ===
	out, _ := runGit(repo, logger, "diff", "--cached", "--name-only", "-z")
	hasStaged := false
	for _, p := range strings.Split(out, "\x00") {
		if strings.TrimSpace(p) != "" {
			hasStaged = true
			break
		}
	}
	if !hasStaged {
		log("ℹ️ 无改动需要提交。")
		return
	}

	// === 提交 ===
	if err := runCmd("git", "-C", repo, "commit", "--author", author, "-m", commit); err != nil {
		log("❌ 提交失败：%v", err)
		return
	}
	log("✅ 已提交：%s", commit)

	// === 推送 ===
	log("🚀 正在推送（origin HEAD）…")
	if _, err := runGit(repo, logger, "push", "origin", "HEAD"); err != nil {
		log("❌ 推送失败：%v", err)
		return
	}
	log("🚀 推送完成")
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
