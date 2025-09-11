/*
XGIT FileOps: file.replace
说明：在单文件范围内做替换（支持纯文本/正则、大小写敏感、行号范围）
*/
// XGIT:BEGIN GO:PACKAGE
package main
// XGIT:END GO:PACKAGE

// XGIT:BEGIN GO:IMPORTS
import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)
// XGIT:END GO:IMPORTS

// XGIT:BEGIN GO:FUNC_FILE_REPLACE
// FileReplace 文本替换 —— 协议: file.replace
// 如果 useRegex=true 则 treat 'find' 为正则；否则为字面量
// 行范围 [lineStart, lineEnd]，<=0 表示不限制
func FileReplace(repo, rel, find, repl string, caseSensitive, useRegex bool, lineStart, lineEnd int, logger *DualLogger) error {
	abs := filepath.Join(repo, rel)
	b, err := os.ReadFile(abs)
	if err != nil { return err }
	lines := strings.Split(normalizeLF(string(b)), "\n")
	L := len(lines)

	start := 1; if lineStart > 0 { start = lineStart }
	end   := L; if lineEnd   > 0 && lineEnd <= L { end = lineEnd }
	if start < 1 { start = 1 }
	if end < start { end = start }

	segment := strings.Join(lines[start-1:end], "\n")
	var out string

	if useRegex {
		flags := 0
		re := (*regexp.Regexp)(nil)
		if caseSensitive {
			re = regexp.MustCompile(find)
		} else {
			re = regexp.MustCompile("(?i)" + find)
		}
		out = re.ReplaceAllString(segment, repl)
	} else {
		if caseSensitive {
			out = strings.ReplaceAll(segment, find, repl)
		} else {
			// 不区分大小写：逐行处理
			out = replaceAllCaseInsensitive(segment, find, repl)
		}
	}

	lines[start-1] = "" // 重写段
	copySlice := append([]string{}, lines[:start-1]...)
	copySlice = append(copySlice, strings.Split(out, "\n")...)
	if end < L { copySlice = append(copySlice, lines[end:]...) }

	newS := strings.Join(copySlice, "\n")
	if len(newS) > 0 && newS[len(newS)-1] != '\n' { newS += "\n" }
	if err := os.WriteFile(abs, []byte(newS), 0o644); err != nil { return err }

	if logger != nil { logger.Log("🪄 file.replace 完成：%s (range=%d..%d, regex=%v, ci=%v)", rel, start, end, useRegex, !caseSensitive) }
	return nil
}

func replaceAllCaseInsensitive(s, find, repl string) string {
	if find == "" { return s }
	lf := strings.ToLower(find)
	var b strings.Builder
	i := 0
	ls := strings.ToLower(s)
	for {
		idx := strings.Index(ls[i:], lf)
		if idx < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+idx])
		b.WriteString(repl)
		i += idx + len(find)
	}
	return b.String()
}
// XGIT:END GO:FUNC_FILE_REPLACE
