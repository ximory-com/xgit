package main

import (
	"fmt"
	"os"
	"strings"

	"xgit/apps/patch/preflight"
)

func runPreflightDryRun(repo string, patch *Patch, logger *DualLogger) error {
	logf := func(format string, a ...any) { if logger != nil { logger.Log(format, a...) } }

	// 1) 建影子工作区
	shadow, err := os.MkdirTemp("", "xgit_preflight_*")
	if err != nil {
		return fmt.Errorf("创建影子工作区失败：%w", err)
	}
	defer os.RemoveAll(shadow)

	if err := runCmd("git", "-C", repo, "worktree", "add", "--detach", shadow, "HEAD"); err != nil {
		return fmt.Errorf("git worktree add 失败：%w", err)
	}
	defer func() { _ = runCmd("git", "-C", repo, "worktree", "remove", "--force", shadow) }()

	// 2) 在影子上干跑补丁（不 commit / 不 push）
	for i, op := range patch.Ops {
		tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)
		if e := applyOp(shadow, op, logger); e != nil {
			logf("❌ 预检执行失败（影子）%s：%v", tag, e)
			return e
		}
	}

	// 3) 收集改动文件
	out, _ := runCmdOut("git", "-C", shadow, "status", "--porcelain")
	changed := make([]string, 0, 32)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 格式：XY<space>path
		if len(line) > 3 {
			changed = append(changed, strings.TrimSpace(line[3:]))
		}
	}
	if len(changed) == 0 {
		logf("ℹ️ 预检：无文件变更")
		return nil
	}

	// 4) 调用 preflight 注册中心
	if err := preflightRun(shadow, changed, logger); err != nil {
		return err
	}

	logf("✅ 预检通过（文件数：%d）", len(changed))
	return nil
}

// 预检：对 files 中的每个文件选择合适的 Runner 并执行
func preflightRun(repo string, files []string, logger *DualLogger) error {
	logf := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}
	for _, f := range files {
		rel := strings.TrimSpace(f)
		if rel == "" {
			continue
		}
		lang := preflight.DetectLangByExt(rel)
		if lang == "" {
			lang = "unknown"
		}
		logf("🧪 预检 %s (%s)", rel, lang)
		if r := preflight.Lookup(rel); r != nil {
			changed, err := r.Run(repo, rel, logf)
			if err != nil {
				return fmt.Errorf("预检失败 %s: %w", rel, err)
			}
			if changed {
				logf("🛠️ 预检已修改 %s", rel)
			} else {
				logf("✔ 预检通过，无需修改：%s", rel)
			}
		} else {
			logf("ℹ️ 无匹配的预检器：%s", rel)
		}
	}
	return nil
}