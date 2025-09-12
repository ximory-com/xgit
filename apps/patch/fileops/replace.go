// XGIT FileOps: file.replace (Stable v2.1 minimal)
// - 兼容原行为（精确匹配、范围、count、EOL/mtime/权限/原子写入）
// - 新增人类友好开关：mode / ignore_spaces / debug
//   mode: "", contains_line, equals_line, contains_file, regex
//   ignore_spaces: 把 ASCII 空格视作“任意空白+”（含全角/零宽/Tab），仅作用于匹配
//   debug: 未命中时输出可视化候选行，便于定位

package fileops

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// FileReplace 在文本文件中按选项执行替换。
func FileReplace(
	repo, rel, find, repl string,
	useRegex bool,
	icase bool,
	lineFrom, lineTo int,
	count int,
	ensureEOFNewline bool,
	multiline bool,
	// New friendly options:
	mode string,        // "", contains_line, equals_line, contains_file, regex
	ignoreSpaces bool,  // 将 ASCII 空格当作“任意空白+”（含全角/零宽/Tab）
	debugNoHit bool,    // 未命中时打印诊断
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

	// 识别并记录原 EOL（CRLF/LF），并规范化为 LF 进行逻辑处理
	isCRLF := bytes.Contains(data, []byte("\r\n"))
	text := normalizeLF(string(data))

	// 计算行范围
	lines := strings.Split(text, "\n")
	total := len(lines)
	start := 1
	if lineFrom > 0 {
		start = lineFrom
	}
	end := total
	if lineTo > 0 && lineTo <= total {
		end = lineTo
	}
	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}

	segment := strings.Join(lines[start-1:end], "\n")

	// 构造正则
	re, err := buildRegexByMode(find, mode, useRegex, icase, multiline, ignoreSpaces)
	if err != nil {
		logfSafe(logf, "❌ file.replace 正则编译失败：%v", err)
		return err
	}

	// 执行替换
	replaced := 0
	var segOut string

	if count > 0 {
		left := count
		segOut = re.ReplaceAllStringFunc(segment, func(m string) string {
			if left <= 0 {
				return m
			}
			replaced++
			left--
			// 对“单次命中”进行替换
			return re.ReplaceAllString(m, repl)
		})
	} else {
		segOut = re.ReplaceAllString(segment, repl)
		replaced = len(re.FindAllStringIndex(segment, -1))
	}

	// 未命中：仅 ensure_eof_nl 或日志并返回
	if replaced == 0 {
		if ensureEOFNewline && !strings.HasSuffix(text, "\n") {
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
			_ = os.Chmod(tmp, modeBits)
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

	// 末尾换行保证
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
	_ = os.Chmod(tmp, modeBits)
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

// buildRegexByMode 将“人类友好”模式/开关编译为正则。
func buildRegexByMode(find, mode string, useRegex, icase, multiline, ignoreSpaces bool) (*regexp.Regexp, error) {
	m := strings.ToLower(strings.TrimSpace(mode))

	// 正则 flags
	flags := ""
	if icase {
		flags += "(?i)"
	}
	if multiline || m == "contains_line" || m == "equals_line" {
		flags += "(?m)"
	}
	flags += "(?s)" // '.' 跨行

	if useRegex || m == "regex" {
		pat := flags + find
		if ignoreSpaces {
			pat = replaceAsciiSpaceWithAnyspace(pat)
		}
		return regexp.Compile(pat)
	}

	// 字面量 → 正则安全，再按模式包裹
	pat := regexp.QuoteMeta(find)
	if ignoreSpaces {
		pat = replaceAsciiSpaceWithAnyspace(pat)
	}

	switch m {
	case "", "contains_line":
		// 行内包含
		return regexp.Compile("(?m)(?s)" + pat)
	case "equals_line":
		// 整行相等
		return regexp.Compile("(?m)(?s)^" + pat + "$")
	case "contains_file":
		// 全文包含
		return regexp.Compile("(?s)" + pat)
	default:
		// 未知 mode 回退到原有精确行为
		return regexp.Compile(flags + pat)
	}
}

// 将 ASCII 空格替换为“任意空白+”，含 Unicode 空白/全角/零宽/Tab。
func replaceAsciiSpaceWithAnyspace(p string) string {
	return strings.ReplaceAll(p, " ", `(?:[\p{Z}\s\u3000\u200B\u200C\u200D\uFEFF]+)`)
}

// 无命中时输出“近似候选行”的可视化差异，便于诊断。
func printNearMiss(logf func(string, ...any), rel string, lines []string, start, end int, want string, ignoreSpaces bool) {
	if logf == nil {
		return
	}
	wantTrim := strings.TrimSpace(want)
	wantFold := collapseSpaces(wantTrim)

	preview := 0
	for i := start - 1; i < end && i < len(lines); i++ {
		if i < 0 {
			continue
		}
		line := lines[i]
		ok := strings.Contains(line, wantTrim)
		if !ok && ignoreSpaces {
			ok = strings.Contains(collapseSpaces(line), wantFold)
		}
		if ok {
			logf("🔍 近似候选 %s:%d: %s", rel, i+1, visualizeSpaces(line))
			preview++
			if preview >= 2 {
				break
			}
		}
	}
	if preview == 0 {
		if start-1 >= 0 && start-1 < len(lines) {
			logf("🔍 范围首行 %s:%d: %s", rel, start, visualizeSpaces(lines[start-1]))
		}
		if end-1 >= 0 && end-1 < len(lines) && end != start {
			logf("🔍 范围末行 %s:%d: %s", rel, end, visualizeSpaces(lines[end-1]))
		}
	}
}

// collapseSpaces：把各种空白折叠成 ASCII 空格，仅用于诊断/近似比较。
func collapseSpaces(s string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		if isAnySpace(r) {
			if !lastSpace {
				b.WriteRune(' ')
			}
			lastSpace = true
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return b.String()
}

// visualizeSpaces：把不可见空白可视化，仅用于日志。
func visualizeSpaces(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\t':
			b.WriteString("→")
		case ' ':
			b.WriteString("␠")
		case '\r':
			b.WriteString("␍")
		case '\n':
			b.WriteString("␊")
		case '\u3000':
			b.WriteString("　") // 全角空格
		case '\u200B':
			b.WriteString("<ZWSP>")
		case '\u200C':
			b.WriteString("<ZWNJ>")
		case '\u200D':
			b.WriteString("<ZWJ>")
		case '\uFEFF':
			b.WriteString("<BOM>")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isAnySpace：判断是否各种空白字符
func isAnySpace(r rune) bool {
	if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
		return true
	}
	switch r {
	case '\u00A0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004',
		'\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200A',
		'\u2028', '\u2029', '\u202F', '\u205F', '\u3000',
		'\u200B', '\u200C', '\u200D', '\uFEFF':
		return true
	}
	return unicode.IsSpace(r)
}
