package main

import (
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"

	"xgit/apps/patch/fileops"
	"xgit/apps/patch/gitops"
)

func applyOp(repo string, op *FileOp, logger *DualLogger) error {

	switch op.Cmd {

	case "file.write":
		return fileops.FileWrite(repo, op.Path, []byte(op.Body), logger)

	case "file.append":
		return fileops.FileAppend(repo, op.Path, []byte(op.Body), logger)

	case "file.prepend":
		return fileops.FilePrepend(repo, op.Path, []byte(op.Body), logger)

	case "file.replace":
		pattern := argStr(op.Args, "pattern", "")
		if pattern == "" {
			return errors.New("file.replace: missing @pattern (body param)")
		}
		repl := op.Body

		isRegex := argBool(op.Args, "regex", false)
		icase := argBool(op.Args, "ci", false)
		lineFrom := argInt(op.Args, "start_line", 0)
		lineTo := argInt(op.Args, "end_line", 0)
		count := argInt(op.Args, "count", 0)
		ensureNL := argBool(op.Args, "ensure_eof_nl", false)
		multiline := argBool(op.Args, "multiline", false)

		mode := strings.TrimSpace(strings.ToLower(argStr(op.Args, "mode", "")))
		ignoreSpc := argBool(op.Args, "ignore_spaces", false)
		debugNoHit := argBool(op.Args, "debug", false)

		logf := func(format string, a ...any) {
			if logger != nil {
				logger.Log(format, a...)
			}
		}

		return fileops.FileReplace(
			repo, op.Path, pattern, repl,
			isRegex, icase,
			lineFrom, lineTo,
			count, ensureNL, multiline,
			mode, ignoreSpc, debugNoHit,
			logf,
		)

	case "file.delete":
		return fileops.FileDelete(repo, op.Path, logger)

	case "file.move":
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

	// ========== gitops 系列 ==========
	case "git.diff":
		return gitops.Diff(repo, op.Body, logger)

	case "git.revert":
		spec := strings.TrimSpace(argStr(op.Args, "spec", op.Body))
		if spec == "" {
			return errors.New("git.revert: missing spec")
		}
		strategy := strings.TrimSpace(argStr(op.Args, "strategy", "abort"))
		return gitops.Revert(repo, spec, strategy, logger)

	case "git.tag":
		name := strings.TrimSpace(argStr(op.Args, "name", ""))
		if name == "" {
			return errors.New("git.tag: missing name")
		}
		ref := strings.TrimSpace(argStr(op.Args, "ref", "HEAD"))
		message := argStr(op.Args, "message", "")
		annotate := argBool(op.Args, "annotate", message != "")
		force := argBool(op.Args, "force", false)
		return gitops.Tag(repo, name, ref, message, annotate, force, logger)

	case "line.insert_before":
		return fileops.LineInsertBefore(repo, op.Path, op.Body, op.Args, logger)
	case "line.insert_after":
		return fileops.LineInsertAfter(repo, op.Path, op.Body, op.Args, logger)
	case "line.replace_line":
		return fileops.LineReplaceLine(repo, op.Path, op.Body, op.Args, logger)
	case "line.delete_line":
		return fileops.LineDeleteLine(repo, op.Path, op.Args, logger)

	default:
		return errors.New("未知指令: " + op.Cmd)
	}
}
