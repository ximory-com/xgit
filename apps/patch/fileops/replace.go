// patch/fileops/replace.go
//
// XGIT FileOps: file.replace (Enhanced)
// 说明：在单文件范围内执行文本替换（支持正则/字面量、大小写开关、行号范围、次数限制、EOL 保持、原子写入、保留权限与 mtime）。
// 用法（dispatcher 侧示例）：
//   logf := func(format string, a ...any) { logger.Log(format, a...) }
//   err := fileops.FileReplace(repo, rel,
//       pattern, repl,                 // find / repl
//       isRegex, icase,                // regex / ci(不区分大小写)
//       lineFrom, lineTo,              // 行范围（1-based，闭区间；0 表示不限）
//       count, ensureEOFNewline,       // 次数上限、末尾换行
//       multiline,                     // 正则 (?m)
//       logf,
//   )
package fileops

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileReplace 在文本文件中按选项执行替换。
// 参数：
//  repo: 仓库根
//  rel : 相对路径
//  find: 查找（字面或正则）
//  repl: 替换文本（来自补丁指令体）
//  useRegex: 是否使用正则
//  icase:    不区分大小写（字面量：手动实现；正则：通过 (?i)）
//  lineFrom/lineTo: 行范围（1-based，闭区间；0=不限）
//  count:    替换次数上限（<=0 表示全部）
//  ensureEOFNewline: 替换后是否保证末尾有换行
//  multiline: 正则多行模式（(?m)）
//  logf:     日志函数，可为 nil FileReplace(
	repo, rel, find, repl string,
	useRegex bool,
	icase bool,
	lineFrom, lineTo int,
	count int,
	ensureEOFNewline bool,
	multiline bool,
	logf func(string, ...any),
) error {
	abs := filepath.Join(repo, rel)

	// 1) 读入文件
	data, err := os.ReadFile(abs)
	if err != nil {
		logfSafe(logf, "❌ file.replace 读取失败：%s (%v)", rel, err)
		return err
	}

	// 保存原权限与 mtime
	fi, _ := os.Stat(abs)
	mode := os.FileMode(0o644)
	var mtime time.Time
	if fi != nil {
		mode = fi.Mode()
		mtime = fi.ModTime()
	}

	// 识别并记录原 EOL（CRLF/LF），统一转为 LF 处理
	isCRLF := bytes.Contains(data, []byte("\r\n"))
	text := normalizeLF(string(data))

	// 2) 计算行范围（安全钳制，不越界；1-based → 0-based）
	lines := strings.Split(text, "\n")
	total := len(lines)
	start := 1
	if lineFrom > 0 {
		start = lineFrom
	}
	end := total
	if lineTo > 0 {
		end = lineTo
	}
	if start < 1 {
		start = 1
	}
	// 转为切片下标并钳制到 [0,total]
	si := start - 1
	if si < 0 {
		si = 0
	}
	if si > total {
		si = total
	}
	ei := end
	if ei < si {
		ei = si
	}
	if ei > total {
		ei = total
	}
	segment := strings.Join(lines[si:ei], "\n")

	// 3) 执行替换
	replaced := 0
	var segOut string

	if useRegex {
		flags := ""
		if icase {
			flags += "(?i)"
		}
		if multiline {
			flags += "(?m)"
		}
		flags += "(?s)" // 让 '.' 跨行
		re, err := regexp.Compile(flags + find)
		if err != nil {
			logfSafe(logf, "❌ file.replace 正则编译失败：%s (%v)", find, err)
			return err
		}
		if count > 0 {
			left := count
			segOut = re.ReplaceAllStringFunc(segment, func(m string) string {
				if left <= 0 {
					return m
				}
				replaced++
				left--
				return re.ReplaceAllString(m, repl)
			})
		} else {
			segOut = re.ReplaceAllString(segment, repl)
			replaced = len(re.FindAllStringIndex(segment, -1))
		}
	} else {
		if icase {
			segOut, replaced = replaceCaseInsensitive(segment, find, repl, count)
		} else {
			segOut, replaced = replaceCaseSensitive(segment, find, repl, count)
		}
	}

	// 4) 无匹配时的 ensure_eof_nl 处理：仍可补末尾换行
	if replaced == 0 {
		if ensureEOFNewline && !strings.HasSuffix(text, "\n") {
			// 恢复原 EOL 并原子写入
			out := text + "\n"
			if isCRLF {
				out = toCRLF(out)
			}
			dir := filepath.Dir(abs)
			tmpf, err := os.CreateTemp(dir, ".xgit_replace_*")
			if err != nil {
				logfSafe(logf, "❌ file.replace 临时文件失败：%v", err)
				return err
			}
			tmp := tmpf.Name()
			defer os.Remove(tmp)
			if _, err := io.WriteString(tmpf, out); err != nil {
				tmpf.Close()
				return err
			}
			if err := tmpf.Sync(); err != nil {
				tmpf.Close()
				return err
			}
			if err := tmpf.Close(); err != nil {
				return err
			}
			_ = os.Chmod(tmp, mode)
			if err := os.Rename(tmp, abs); err != nil {
				logfSafe(logf, "❌ file.replace 覆盖失败：%v", err)
				return err
			}
			if !mtime.IsZero() {
				_ = os.Chtimes(abs, time.Now(), mtime)
			}
			logfSafe(logf, "✏️ file.replace 确保末尾换行：%s", rel)
			return nil
		}
		logfSafe(logf, "⚠️ file.replace 无匹配：%s（范围 %d-%d）", rel, start, end)
		return nil
	}

	// 5) 拼回全文（基于 si/ei，避免越界与多余换行）
	var builder strings.Builder
	if si > 0 {
		builder.WriteString(strings.Join(lines[:si], "\n"))
		builder.WriteString("\n")
	}
	builder.WriteString(segOut)
	if ei < total {
		builder.WriteString("\n")
		builder.WriteString(strings.Join(lines[ei:], "\n"))
	}
	result := builder.String()

	// 末尾换行保证
	if ensureEOFNewline && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	// 恢复为原 EOL
	if isCRLF {
		result = toCRLF(result)
	}

	// 6) 原子写入：临时文件 → fsync → rename；保留权限/mtime
	dir := filepath.Dir(abs)
	tmpf, err := os.CreateTemp(dir, ".xgit_replace_*")
	if err != nil {
		logfSafe(logf, "❌ file.replace 临时文件失败：%v", err)
		return err
	}
	tmp := tmpf.Name()
	defer os.Remove(tmp)

	if _, err := io.WriteString(tmpf, result); err != nil {
		_ = tmpf.Close()
		return err
	}
	if err := tmpf.Sync(); err != nil {
		_ = tmpf.Close()
		return err
	}
	if err := tmpf.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp, mode)
	if err := os.Rename(tmp, abs); err != nil {
		logfSafe(logf, "❌ file.replace 覆盖失败：%v", err)
		return err
	}
	if !mtime.IsZero() {
		_ = os.Chtimes(abs, time.Now(), mtime)
	}

	if count > 0 {
		logfSafe(logf, "✏️ file.replace 完成：%s（命中 %d，最多 %d，范围 %d-%d）", rel, replaced, count, start, end)
	} else {
		logfSafe(logf, "✏️ file.replace 完成：%s（命中 %d，范围 %d-%d）", rel, replaced, start, end)
	}
	return nil
}

// ===== helpers =====
 logfSafe(logf func(string, ...any), format string, a ...any) {
	if logf != nil {
		logf(format, a...)
	}
}
 normalizeLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
 toCRLF(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// 区分大小写；count<=0 表示全部 replaceCaseSensitive(s, find, repl string, count int) (string, int) {
	if find == "" {
		return s, 0
	}
	if count <= 0 {
		n := strings.Count(s, find)
		return strings.ReplaceAll(s, find, repl), n
	}
	out := s
	repld := 0
	for repld < count {
		idx := strings.Index(out, find)
		if idx < 0 {
			break
		}
		out = out[:idx] + repl + out[idx+len(find):]
		repld++
	}
	return out, repld
}

// 不区分大小写；count<=0 表示全部 replaceCaseInsensitive(s, find, repl string, count int) (string, int) {
	if find == "" {
		return s, 0
	}
	var b strings.Builder
	ls := strings.ToLower(s)
	lf := strings.ToLower(find)
	i, repld := 0, 0
	for {
		if count > 0 && repld >= count {
			break
		}
		j := strings.Index(ls[i:], lf)
		if j < 0 {
			break
		}
		b.WriteString(s[i : i+j]) // 原文切片按原 s 写回
		b.WriteString(repl)
		i += j + len(find)
		repld++
	}
	b.WriteString(s[i:])
	return b.String(), repld
}