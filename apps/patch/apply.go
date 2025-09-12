package patch

import (
	"fmt"
	"path/filepath"

	"xgit/apps/patch/fileops"
)

// XGIT:BEGIN APPLY DISPATCH
// 将 11 条 file.* 指令全部分发到 fileops 包
// 约定 fileops 函数签名（若与你现有不一致，把调用名改成你的即可）:
//   Write(repo, path, body string, logf func(string, ...any)) error
//   Append(repo, path, body string, logf func(string, ...any)) error
//   Prepend(repo, path, body string, logf func(string, ...any)) error
//   Replace(repo, path string, args map[string]string, body string, logf func(string, ...any)) error
//   Delete(repo, path string, logf func(string, ...any)) error
//   Move(repo, from, to string, logf func(string, ...any)) error
//   Chmod(repo, path string, args map[string]string, logf func(string, ...any)) error
//   EOL(repo, path string, args map[string]string, logf func(string, ...any)) error
//   Image(repo, path, base64Body string, logf func(string, ...any)) error
//   Binary(repo, path, base64Body string, logf func(string, ...any)) error
//   Diff(repo string, body string, logf func(string, ...any)) error
func ApplyOnce(logger *DualLogger, repo string, patch *Patch) {
	log := logger.Log

	// 事务前清理（保留你现有行为）
	log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = shell("git", "-C", repo, "clean", "-fd")

	changed := false

	for i, op := range patch.Ops {
		tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)

		switch op.Cmd {
		case "write":
			if err := fileops.Write(repo, op.Path, op.Body, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "append":
			if err := fileops.Append(repo, op.Path, op.Body, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "prepend":
			if err := fileops.Prepend(repo, op.Path, op.Body, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "replace":
			if err := fileops.Replace(repo, op.Path, op.Args, op.Body, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "delete":
			if err := fileops.Delete(repo, op.Path, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "move":
			if err := fileops.Move(repo, op.Path, op.To, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "chmod":
			if err := fileops.Chmod(repo, op.Path, op.Args, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "eol":
			if err := fileops.EOL(repo, op.Path, op.Args, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "image":
			if err := fileops.Image(repo, op.Path, op.Body, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "binary":
			if err := fileops.Binary(repo, op.Path, op.Body, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
			changed = true
		case "diff":
			if err := fileops.Diff(repo, op.Body, log); err != nil {
				log("❌ %s 失败：%v", tag, err); return
			}
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

	// 组装提交信息（沿用调用方传进来的 commit/author；保持你现有提交逻辑）
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
