package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"xgit/apps/patch/fileops"
	"xgit/apps/patch/gitops"
)

// ========== 小工具：从 map 中读取参数（带默认值） ==========
func argBool(m map[string]string, key string, def bool) bool {
	if v, ok := m[key]; ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return def
}
func argInt(m map[string]string, key string, def int) int {
	if v, ok := m[key]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}
func argStr(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// XGIT:BEGIN APPLY DISPATCH
// 将 11 条 file.* 指令全部分发到同包内实现（fileops/*.go），保持日志风格一致
func applyOp(repo string, op *FileOp, logger *DualLogger) error {
	switch op.Cmd {

	case "file.write":
		return fileops.FileWrite(repo, op.Path, []byte(op.Body), logger)

	case "file.append":
		return fileops.FileAppend(repo, op.Path, []byte(op.Body), logger)

	case "file.prepend":
		return fileops.FilePrepend(repo, op.Path, []byte(op.Body), logger)

	case "file.replace": {
		// 约定：复杂文本参数（pattern 等）由解析器写入 op.Args；正文作为替换体（可为空表示删除命中片段）
		pattern := argStr(op.Args, "pattern", "")
		if pattern == "" {
			return errors.New("file.replace: missing @pattern (body param)")
		}
		repl := op.Body

		// 传统选项
		isRegex   := argBool(op.Args, "regex", false)
		icase     := argBool(op.Args, "ci", false)
		lineFrom  := argInt (op.Args, "start_line", 0)
		lineTo    := argInt (op.Args, "end_line",   0)
		count     := argInt (op.Args, "count", 0)
		ensureNL  := argBool(op.Args, "ensure_eof_nl", false)
		multiline := argBool(op.Args, "multiline", false)

		// 人类友好附加参数（可选）
		mode       := strings.TrimSpace(strings.ToLower(argStr(op.Args, "mode", ""))) // "", contains_line, equals_line, contains_file, regex
		ignoreSpc  := argBool(op.Args, "ignore_spaces", false)
		debugNoHit := argBool(op.Args, "debug", false)

		logf := func(format string, a ...any) { if logger != nil { logger.Log(format, a...) } }

		return fileops.FileReplace(
			repo, op.Path, pattern, repl,
			isRegex, icase,
			lineFrom, lineTo,
			count, ensureNL, multiline,
			mode, ignoreSpc, debugNoHit,
			logf,
		)
	}

	case "file.delete":
		return fileops.FileDelete(repo, op.Path, logger)

	case "file.move":
		// 新协议：目标路径来自正文第一行，解析器已写入 op.Args["to"]
		to := strings.TrimSpace(op.Args["to"])
		if to == "" {
			return errors.New("file.move: 缺少目标路径（正文第一行）")
		}
		return fileops.FileMove(repo, op.Path, to, logger)

	case "file.chmod":
		modeStr := strings.TrimSpace(op.Args["mode"])
		if modeStr == "" {
			return errors.New("file.chmod: 缺少 mode（八进制，如 644/755）")
		}
		u, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			return errors.New("file.chmod: 解析 mode 失败（只支持八进制数值，例如 644/755）")
		}
		return fileops.FileChmod(repo, op.Path, os.FileMode(u), logger)

	case "file.eol":
		style := strings.ToLower(strings.TrimSpace(argStr(op.Args, "style", "lf")))
		ensureNL := argBool(op.Args, "ensure_nl", true)
		return fileops.FileEOL(repo, op.Path, style, ensureNL, logger)

	case "file.image":
		raw := strings.TrimSpace(op.Body)
		if raw == "" {
			return errors.New("file.image: 缺少 base64 内容")
		}
		bin, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return errors.New("file.image: base64 解码失败")
		}
		return fileops.FileImage(repo, op.Path, string(bin), logger)

	case "file.binary":
		raw := strings.TrimSpace(op.Body)
		if raw == "" {
			return errors.New("file.binary: 缺少 base64 内容")
		}
		bin, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return errors.New("file.binary: base64 解码失败")
		}
		return fileops.FileBinary(repo, op.Path, string(bin), logger)

	case "file.diff":
		// 传入 header 的路径，便于 diff.go 在缺少文件头时自动包装
		return fileops.FileDiff(repo, op.Path, op.Body, logger)

	// ========== gitops 系列 ==========
	case "git.diff":
		return gitops.Diff(repo, op.Body, logger)

	case "git.revert":
		return gitops.Revert(repo, op.Body, logger)

	case "git.tag":
		return gitops.Tag(repo, op.Body, logger)

	default:
		return errors.New("未知指令: " + op.Cmd)
	}
}
// XGIT:END APPLY DISPATCH

// XGIT:BEGIN APPLY ONCE
// ApplyOnce：用事务包裹“执行阶段”，成功后再统一提交/推送
func ApplyOnce(logger *DualLogger, repo string, patch *Patch) {
	log := logger.Log
	logf := func(format string, a ...any) { if logger != nil { logger.Log(format, a...) } }

	// 1) 事务阶段：逐条执行 file.*，任一失败则回滚到补丁前 HEAD
	err := WithGitTxn(repo, logf, func() error {
		for i, op := range patch.Ops {
			tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)
			if e := applyOp(repo, op, logger); e != nil {
				log("❌ %s 失败：%v", tag, e)
				return e
			}
			log("✅ %s 成功", tag)
		}
		return nil
	})
	if err != nil {
		// 事务内部已回滚并输出日志
		return
	}

	// 2) 成功后统一 stage/commit/push（不置于事务内）
	_ = runCmd("git", "-C", repo, "add", "-A")

	names, _ := runCmdOut("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.Log("ℹ️ 无改动需要提交。")
		logger.Log("✅ 本次补丁完成")
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
// XGIT:END APPLY ONCE

// XGIT:BEGIN GIT_TXN_HELPERS
// 说明：仅保留 runCmd / runCmdOut + WithGitTxn；不再提供 shell() 简化器
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return nil
}
func runCmdOut(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
func gitRevParseHEAD(repo string) (string, error) {
	return runCmdOut("git", "-C", repo, "rev-parse", "--verify", "HEAD")
}
func gitResetHard(repo, rev string) error {
	if rev == "" {
		return runCmd("git", "-C", repo, "reset", "--hard")
	}
	return runCmd("git", "-C", repo, "reset", "--hard", rev)
}
func gitCleanFD(repo string) error {
	return runCmd("git", "-C", repo, "clean", "-fd")
}

// WithGitTxn：在 repo 上开启一次 Git 事务：fn() 出错则回滚到补丁前状态。
func WithGitTxn(repo string, logf func(string, ...any), fn func() error) error {
	preHead, _ := gitRevParseHEAD(repo)
	_ = gitResetHard(repo, "")
	_ = gitCleanFD(repo)

	var err error
	defer func() {
		if err != nil {
			if preHead != "" {
				_ = gitResetHard(repo, preHead)
			} else {
				_ = gitResetHard(repo, "")
			}
			_ = gitCleanFD(repo)
			if logf != nil {
				logf("↩️ 回滚到补丁前状态：%s", preHead)
			}
		}
	}()

	if e := fn(); e != nil {
		err = e
		return err
	}
	return nil
}
// XGIT:END GIT_TXN_HELPERS
