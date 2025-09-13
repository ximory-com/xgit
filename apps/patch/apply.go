package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xgit/apps/patch/preflight"
)

// ApplyOnce：用事务包裹“执行阶段”，成功后再统一提交/推送
// 同时把日志“截断写入 repo/patch.log”，并与控制台同步输出。
func ApplyOnce(logger *DualLogger, repo string, patch *Patch) {
	// 1) 打开/截断 patch.log
	logPath := filepath.Join(repo, "patch.log")
	f, ferr := os.Create(logPath) // 截断旧内容
	if ferr != nil && logger != nil {
		logger.Log("⚠️ 无法写入 patch.log：%v（将仅输出到控制台）", ferr)
	}
	writeFile := func(s string) {
		if f != nil {
			_, _ = f.WriteString(s)
			if !strings.HasSuffix(s, "\n") {
				_, _ = f.WriteString("\n")
			}
		}
	}
	log := func(format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		if logger != nil {
			logger.Log("%s", msg)
		}
		writeFile(msg)
	}
	logf := func(format string, a ...any) { log(format, a...) }
	defer func() { if f != nil { _ = f.Close() } }()

	// 预检（影子工作区 + 语言 runner）
	if err := runPreflightDryRun(repo, patch, logger); err != nil {
		log("❌ 预检失败：%v", err)
		return
	}

	// 2) 事务阶段
	err := WithGitTxn(repo, logf, func() error {
		// 先应用所有指令
		for i, op := range patch.Ops {
			tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)
			if e := applyOp(repo, op, logger); e != nil {
				log("❌ %s 失败：%v", tag, e)
				return e
			}
			log("✅ %s 成功", tag)
		}

		// 再收集本次改动并在“真实仓库”跑预检（失败则回滚）
		changed, _ := collectChangedFiles(repo)
		if len(changed) > 0 {
			logf("🧪 预检（真实仓库）：%d 个文件", len(changed))
			if err := preflightRun(repo, changed, logger); err != nil {
				logf("❌ 预检失败：%v", err)
				return err
			}
			logf("✅ 预检通过")
		} else {
			logf("ℹ️ 预检：无文件变更")
		}

		return nil
	})
	if err != nil {
		return // 事务内部已回滚并记录日志
	}

	// 3) 统一 stage/commit/push
	_ = runCmd("git", "-C", repo, "add", "-A")

	names, _ := runCmdOut("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		log("ℹ️ 无改动需要提交。")
		log("✅ 本次补丁完成")
		return
	}

	commit := "chore: apply patch"
	author := "XGit Bot <bot@xgit.local>"
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

// 仅为编译引用，确保预检包被链接（如你已在别处用到可删）
var _ = preflight.Register