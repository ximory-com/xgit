package main

import (
	"fmt"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
	"xgit/apps/patch/fileops"
)

// -------------- 小工具：从 map 里取布尔/整型，带默认值 --------------
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

// -------------- dispatcher：把 11 条文件指令接到 fileops --------------
func applyOp(repo string, op *FileOp, logger *DualLogger) error {
	switch op.Cmd { // 假设 parser 里把指令名放在 op.Cmd；若是 op.Name/Kind，自行替换字段名
	case "file.write":
		// 写文本/任意字节（这里按文本对待，统一 LF 在 fileops 里处理）
		return fileops.FileWrite(repo, op.Path, []byte(op.Body), logger)

	case "file.append":
		return fileops.FileAppend(repo, op.Path, []byte(op.Body), logger)

	case "file.prepend":
		return fileops.FilePrepend(repo, op.Path, []byte(op.Body), logger)

	case "file.replace":
		// 支持：
		//  - pattern: 正则/字面（取决于 mode）
		//  - repl:    替换文本
		//  - mode:    "regex" | "literal"（默认 literal）
		//  - icase:   不区分大小写（默认 false）
		//  - multiline: 多行模式（默认 false）
		//  - line_from / line_to: 作用的行范围（默认 0 表示不限制）
		pattern := argStr(op.Args, "pattern", "")
		repl := argStr(op.Args, "repl", "")
		if pattern == "" {
			return errors.New("file.replace: 缺少 pattern")
		}
		mode := strings.ToLower(strings.TrimSpace(argStr(op.Args, "mode", "literal")))
		isRegex := (mode == "regex" || mode == "re")
		icase := argBool(op.Args, "icase", false)
		lineFrom := argInt(op.Args, "line_from", 0)
		lineTo := argInt(op.Args, "line_to", 0)

		return fileops.FileReplace(repo, op.Path, pattern, repl, isRegex, icase, lineFrom, lineTo, logger)

	case "file.delete":
		return fileops.FileDelete(repo, op.Path, logger)

	case "file.move":
		// 需要 op.Args["to"] 作为目标
		to := strings.TrimSpace(op.Args["to"])
		if to == "" {
			return errors.New("file.move: 缺少 to")
		}
		return fileops.FileMove(repo, op.Path, to, logger)

	case "file.chmod":
		// 目前 fileops 版本期望 os.FileMode；仅支持八进制（如 644/755）
		modeStr := strings.TrimSpace(op.Args["mode"])
		if modeStr == "" {
			return errors.New("file.chmod: 缺少 mode（八进制，如 644/755）")
		}
		// 允许带前导 0
		u, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			return errors.New("file.chmod: 解析 mode 失败（只支持八进制数值，例如 644/755）")
		}
		return fileops.FileChmod(repo, op.Path, os.FileMode(u), logger)

	case "file.eol":
		// style=lf|crlf，ensure_nl=bool
		style := strings.ToLower(strings.TrimSpace(argStr(op.Args, "style", "lf")))
		ensureNL := argBool(op.Args, "ensure_nl", true)
		return fileops.FileEOL(repo, op.Path, style, ensureNL, logger)

	case "file.image":
		// op.Body 为 base64
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



// XGIT:BEGIN APPLY DISPATCH
// 将 11 条 file.* 指令全部分发到同包内实现（fileops/*.go 应为 package patch）
func ApplyOnce(logger *DualLogger, repo string, patch *Patch) {
	log := logger.Log

	// 清理工作区（保持原有行为）
	log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = shell("git", "-C", repo, "clean", "-fd")


	// 需要：import "fmt" 和 "strings"

	for i, op := range patch.Ops {
		tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)

		if err := applyOp(repo, op, logger); err != nil {
			log("❌ %s 失败：%v", tag, err)
			return
		}
		log("✅ %s 成功", tag)
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
