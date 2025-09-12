package patch

import (
	"fmt"
	"strings"
)

// XGIT:BEGIN APPLY DISPATCH
// 将 11 条 file.* 指令全部分发到同包内实现（fileops/*.go 应为 package patch）
func ApplyOnce(logger *DualLogger, repo string, patch *Patch) {
	log := logger.Log

	// 清理工作区（保持原有行为）
	log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = shell("git", "-C", repo, "clean", "-fd")

	changed := false

	for i, op := range patch.Ops {
		tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)

		switch op.Cmd {
		case "write":
			if err := Write(repo, op.Path, op.Body, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "append":
			if err := Append(repo, op.Path, op.Body, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "prepend":
			if err := Prepend(repo, op.Path, op.Body, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "replace":
			if err := Replace(repo, op.Path, op.Args, op.Body, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "delete":
			if err := Delete(repo, op.Path, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "move":
			if err := Move(repo, op.Path, op.To, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "chmod":
			if err := Chmod(repo, op.Path, op.Args, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "eol":
			if err := EOL(repo, op.Path, op.Args, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "image":
			if err := Image(repo, op.Path, op.Body, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "binary":
			if err := Binary(repo, op.Path, op.Body, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		case "diff":
			if err := Diff(repo, op.Body, log); err != nil { log("❌ %s 失败：%v", tag, err); return }
			changed = true
		default:
			log("⚠️ 未识别命令：%s（忽略）", tag)
		}
	}

	// 若无改动，直接返回
	if !changed {
		log("ℹ️ 无改动需要提交。")
		log("✅ 本次补丁完成")
		return
	}

	// 有改动则提交
	names, _, _ := shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		log("ℹ️ 无改动需要提交。")
		log("✅ 本次补丁完成")
		return
	}

	commit := "chore: apply patch"
	author := "XGit Bot <bot@xgit.local>"
	log("ℹ️ 提交说明：%s", commit)
	log("ℹ️ 提交作者：%s", author)
	_, _, _ = shell("git", "-C", repo, "commit", "--author", author, "-m", commit)
	log("✅ 已提交：%s", commit)

	log("🚀 正在推送（origin HEAD）…")
	if _, er, err := shell("git", "-C", repo, "push", "origin", "HEAD"); err != nil {
		log("❌ 推送失败：%s", er)
	} else {
		log("🚀 推送完成")
	}
	log("✅ 本次补丁完成")
}
// XGIT:END APPLY DISPATCH
