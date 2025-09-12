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
//  logf:     日志函数，可为 nil
func FileReplace(
	repo, rel, find, repl string,
	useRegex bool,
	icase bool,
	lineFrom, lineTo int,
	count int,
	ensureEOFNewline bool,
	multiline bool,
	// ✨ 新增“人类友好”选项
	mode string,          // "", contains_line, equals_line, contains_file, regex
	ignoreSpaces bool,    // 把空格视为任意空白（含全角/零宽/Tab）
	debugNoHit bool,      // 无匹配时输出诊断
	logf func(string, ...any),
) error {
	abs := filepath.Join(repo, rel)

	// 读入文件
	data, err := os.ReadFile(abs)
	if err != nil {
		logfSafe(logf, "❌ file.replace 读取失败：%s (%v)", rel, err)
		return err
	}

	// 保存原权限与 mtime
	fi, _ := os.Stat(abs)
	modeBits := os.FileMode(0o644)
	var mtime time.Time
	if fi != nil {
		modeBits = fi.Mode()
		mtime = fi.ModTime()
	}

	// EOL 识别与规范化
	isCRLF := bytes.Contains(data, []byte("\r\n"))
	text := normalizeLF(string(data))

	// 行范围
	lines := strings.Split(text, "\n")
	total := len(lines)
	start := 1
	if lineFrom > 0 { start = lineFrom }
	end := total
	if lineTo > 0 && lineTo <= total { end = lineTo }
	if start < 1 { start = 1 }
	if end < start { end = start }
	segment := strings.Join(lines[start-1:end], "\n")

	// === 构造匹配模式 ===
	pat, err := buildRegexByMode(find, mode, useRegex, icase, multiline, ignoreSpaces)
	if err != nil {
		logfSafe(logf, "❌ file.replace 正则编译失败：%v", err)
		return err
	}
	re := pat

	// === 执行替换 ===
	replaced := 0
	var segOut string

	if count > 0 {
		left := count
		segOut = re.ReplaceAllStringFunc(segment, func(m string) string {
			if left <= 0 { return m }
			replaced++
			left--
			return re.ReplaceAllString(m, repl) // 对单次命中替换
		})
	} else {
		segOut = re.ReplaceAllString(segment, repl)
		replaced = len(re.FindAllStringIndex(segment, -1))
	}

	// 诊断：无匹配时可输出“接近行”的可视化差异
	if replaced == 0 {
		if ensureEOFNewline && !strings.HasSuffix(text, "\n") {
			out := text + "\n"
			if isCRLF { out = toCRLF(out) }
			dir := filepath.Dir(abs)
			tmpf, err := os.CreateTemp(dir, ".xgit_replace_*")
			if err != nil { logfSafe(logf, "❌ file.replace 临时文件失败：%v", err); return err }
			tmp := tmpf.Name()
			defer os.Remove(tmp)
			if _, err := io.WriteString(tmpf, out); err != nil { tmpf.Close(); return err }
			if err := tmpf.Sync(); err != nil { tmpf.Close(); return err }
			if err := tmpf.Close(); err != nil { return err }
			_ = os.Chmod(tmp, modeBits)
			if err := os.Rename(tmp, abs); err != nil { logfSafe(logf, "❌ file.replace 覆盖失败：%v", err); return err }
			if !mtime.IsZero() { _ = os.Chtimes(abs, time.Now(), mtime) }
			logfSafe(logf, "✏️ file.replace 确保末尾换行：%s", rel)
			return nil
		}
		if debugNoHit {
			printNearMiss(logf, rel, lines, start, end, find, ignoreSpaces)
		}
		logfSafe(logf, "⚠️ file.replace 无匹配：%s（范围 %d-%d）", rel, start, end)
		return nil
	}

	// 拼回全文
	var builder strings.Builder
	if start > 1 {
		builder.WriteString(strings.Join(lines[:start-1], "\n"))
		builder.WriteString("\n")
	}
	builder.WriteString(segOut)
	if end < total {
		builder.WriteString("\n")
		builder.WriteString(strings.Join(lines[end:], "\n"))
	}
	result := builder.String()

	// 末尾换行
	if ensureEOFNewline && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	// 恢复原 EOL
	if isCRLF {
		result = toCRLF(result)
	}

	// 原子写入
	dir := filepath.Dir(abs)
	tmpf, err := os.CreateTemp(dir, ".xgit_replace_*")
	if err != nil { logfSafe(logf, "❌ file.replace 临时文件失败：%v", err); return err }
	tmp := tmpf.Name()
	defer os.Remove(tmp)

	if _, err := io.WriteString(tmpf, result); err != nil { _ = tmpf.Close(); return err }
	if err := tmpf.Sync(); err != nil { _ = tmpf.Close(); return err }
	if err := tmpf.Close(); err != nil { return err }
	_ = os.Chmod(tmp, modeBits)
	if err := os.Rename(tmp, abs); err != nil { logfSafe(logf, "❌ file.replace 覆盖失败：%v", err); return err }
	if !mtime.IsZero() { _ = os.Chtimes(abs, time.Now(), mtime) }

	if count > 0 {
		logfSafe(logf, "✏️ file.replace 完成：%s（命中 %d，最多 %d，范围 %d-%d）", rel, replaced, count, start, end)
	} else {
		logfSafe(logf, "✏️ file.replace 完成：%s（命中 %d，范围 %d-%d）", rel, replaced, start, end)
	}
	return nil
}

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
	text := normalizeLF(string(data)) // 来自 textutil.go

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
				out = toCRLF(out) // 来自 textutil.go
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
		result = toCRLF(result) // 来自 textutil.go
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

func logfSafe(logf func(string, ...any), format string, a ...any) {
	if logf != nil {
		logf(format, a...)
	}
}

// 区分大小写；count<=0 表示全部
func replaceCaseSensitive(s, find, repl string, count int) (string, int) {
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

// 不区分大小写；count<=0 表示全部
func replaceCaseInsensitive(s, find, repl string, count int) (string, int) {
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
\n// === friendly matching helpers ===\n\n// buildRegexByMode 将“人类友好”模式/开关编译为正则。\nfunc buildRegexByMode(find, mode string, useRegex, icase, multiline, ignoreSpaces bool) (*regexp.Regexp, error) {\n\tm := strings.ToLower(strings.TrimSpace(mode))\n\tif m == \"regex\" || useRegex {\n\t\t// 原生正则：仅叠加 flags\n\t\tflags := \"\"\n\t\tif icase { flags += \"(?i)\" }\n\t\tif multiline || m == \"contains_line\" || m == \"equals_line\" { flags += \"(?m)\" }\n\t\tflags += \"(?s)\"\n\t\tpat := flags + find\n\t\tif ignoreSpaces {\n\t\t\tpat = replaceAsciiSpaceWithAnyspace(pat)\n\t\t}\n\t\treturn regexp.Compile(pat)\n\t}\n\n\t// 字面量 → 正则安全\n\tpat := regexp.QuoteMeta(find)\n\tif ignoreSpaces {\n\t\tpat = replaceAsciiSpaceWithAnyspace(pat)\n\t}\n\n\tswitch m {\n\tcase \"\", \"contains_line\":\n\t\t// 行内包含\n\t\treturn regexp.Compile(\"(?m)(?s)\" + pat)\n\tcase \"equals_line\":\n\t\treturn regexp.Compile(\"(?m)(?s)^\" + pat + \"$\")\n\tcase \"contains_file\":\n\t\treturn regexp.Compile(\"(?s)\" + pat)\n\tdefault:\n\t\t// 未知 mode 退回到“精确行为”\n\t\tflags := \"\"\n\t\tif icase { flags += \"(?i)\" }\n\t\tif multiline { flags += \"(?m)\" }\n\t\tflags += \"(?s)\"\n\t\treturn regexp.Compile(flags + pat)\n\t}\n}\n\n// replaceAsciiSpaceWithAnyspace 把 ASCII 空格替换为“任意空白+”，含全角/零宽等。\nfunc replaceAsciiSpaceWithAnyspace(p string) string {\n\treturn strings.ReplaceAll(p, \" \", `(?:[\\p{Z}\\s\\u3000\\u200B\\u200C\\u200D\\uFEFF]+)`) // \"+\" 表示至少一个\n}\n\n// printNearMiss 在无命中时打印一些“近似候选行”的可视化差异，便于诊断。\nfunc printNearMiss(logf func(string, ...any), rel string, lines []string, start, end int, want string, ignoreSpaces bool) {\n\tif logf == nil { return }\n\tpreview := 0\n\tfor i := start - 1; i < end && i < len(lines); i++ {\n\t\tline := lines[i]\n\t\tif strings.Contains(line, strings.TrimSpace(want)) || (ignoreSpaces && strings.Contains(collapseSpaces(line), collapseSpaces(strings.TrimSpace(want)))) {\n\t\t\tlogf(\"🔍 近似候选 %s:%d: %s\", rel, i+1, visualizeSpaces(line))\n\t\t\tpreview++\n\t\t\tif preview >= 2 { break }\n\t\t}\n\t}\n\tif preview == 0 {\n\t\t// 兜底给出范围内首尾两行的可视化\n\t\tif start-1 < len(lines) && start-1 >= 0 { logf(\"🔍 范围首行 %s:%d: %s\", rel, start, visualizeSpaces(lines[start-1])) }\n\t\tif end-1 < len(lines) && end-1 >= 0 { logf(\"🔍 范围末行 %s:%d: %s\", rel, end, visualizeSpaces(lines[end-1])) }\n\t}\n}\n\n// collapseSpaces：把各种空白折叠成 ASCII 空格\nfunc collapseSpaces(s string) string {\n\trs := []rune(s)\n\tvar b strings.Builder\n\tlastSpace := false\n\tfor _, r := range rs {\n\t\tif isAnySpace(r) {\n\t\t\tif !lastSpace { b.WriteRune(' ') }\n\t\t\tlastSpace = true\n\t\t\tcontinue\n\t\t}\n\t\tb.WriteRune(r)\n\t\tlastSpace = false\n\t}\n\treturn b.String()\n}\n\n// visualizeSpaces：把不可见空白可视化\nfunc visualizeSpaces(s string) string {\n\treplacer := strings.NewReplacer(\n\t\t\"\\t\", \"→\",\n\t\t\" \", \"␠\",\n\t\t\"\\r\", \"␍\",\n\t\t\"\\n\", \"␊\",\n\t\t\"\\u3000\", \"　\", // 全角空格\n\t)\n\t// 处理零宽空白\n\ts = strings.ReplaceAll(s, \"\\u200B\", \"<ZWSP>\")\n\ts = strings.ReplaceAll(s, \"\\u200C\", \"<ZWNJ>\")\n\ts = strings.ReplaceAll(s, \"\\u200D\", \"<ZWJ>\")\n\ts = strings.ReplaceAll(s, \"\\uFEFF\", \"<BOM>\")\n\treturn replacer.Replace(s)\n}\n\nfunc isAnySpace(r rune) bool {\n\tif r == '\\u3000' || r == '\\u200B' || r == '\\u200C' || r == '\\u200D' || r == '\\uFEFF' { return true }\n\treturn r == ' ' || r == '\\t' || r == '\\n' || r == '\\r' || strings.ContainsRune(\"\\u0085\\u00A0\\u1680\\u2000\\u2001\\u2002\\u2003\\u2004\\u2005\\u2006\\u2007\\u2008\\u2009\\u200A\\u2028\\u2029\\u202F\\u205F\\u3000\", r)\n}\n=== end ===

# 3) 新验证脚本：覆盖中文 + 空白容错 + 行包含/整行
