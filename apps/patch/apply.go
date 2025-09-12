package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"xgit/apps/patch/fileops"
)

/********** small arg helpers **********/
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

/********** dispatcher for 11 file.* ops **********/
func applyOp(repo string, op *FileOp, logger *DualLogger) error {
	switch op.Cmd {
	case "file.write":
		return fileops.FileWrite(repo, op.Path, []byte(op.Body), logger)

	case "file.append":
		return fileops.FileAppend(repo, op.Path, []byte(op.Body), logger)

	case "file.prepend":
		return fileops.FilePrepend(repo, op.Path, []byte(op.Body), logger)

	case "file.replace": {
		// 新协议：pattern 等复杂参数从 body 参数区进入 op.Args；正文作为替换体（可为空）
		pattern := argStr(op.Args, "pattern", "")
		if pattern == "" {
			return errors.New("file.replace: missing @pattern (body param)")
		}
		repl := op.Body

		// flags/range
		isRegex   := argBool(op.Args, "regex", false)
		icase     := argBool(op.Args, "ci", false)
		lineFrom  := argInt (op.Args, "start_line", 0)
		lineTo    := argInt (op.Args, "end_line",   0)
		count     := argInt (op.Args, "count", 0)
		ensureNL  := argBool(op.Args, "ensure_eof_nl", false)
		multiline := argBool(op.Args, "multiline", false)

		// 新增人类友好开关
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
		// 新协议：parser 已把正文第一行写入 op.Args["to"]
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
		return fileops.FileDiff(repo, op.Body, logger)

	default:
		return errors.New("未知指令: " + op.Cmd)
	}
}

/********** ApplyOnce: sequential apply + commit/push (kept for CLI) **********/
func ApplyOnce(logger *DualLogger, repo string, patch *Patch) {
	log := logger.Log

	// 预清理（与事务版独立存在）
	log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = shell("git", "-C", repo, "clean", "-fd")

	for i, op := range patch.Ops {
		tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)
		if err := applyOp(repo, op, logger); err != nil {
			log("❌ %s 失败：%v", tag, err)
			return
		}
		log("✅ %s 成功", tag)
	}

	// 统一 stage 并提交
	_, _, _ = shell("git", "-C", repo, "add", "-A")
	names, _, _ := shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.Log("ℹ️ 无改动需要提交。")
		logger.Log("✅ 本次补丁完成")
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

/********** ApplyPatch: transactional apply (rolls back on error) **********/
func ApplyPatch(repo string, ops []FileOp, logger DualLogger) error {
	logf := func(format string, a ...any) { logger.Log(format, a...) }
	return WithGitTxn(repo, logf, func() error {
		for i, op := range ops {
			if err := applyOp(repo, &op, &logger); err != nil {
				return fmt.Errorf("op#%d: %w", i+1, err)
			}
		}
		return nil
	})
}

/********** tiny shell helpers (stdout, stderr, err) **********/
func shell(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	stdout := string(out)
	if err != nil {
		return "", stdout, err
	}
	return stdout, "", nil
}

/********** git helpers + txn **********/
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
