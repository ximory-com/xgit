 patch/fileops/replace.go

 XGIT FileOps: file.replace (Enhanced)
 说明：在单文件范围内执行文本替换（支持正则/字面量、大小写开关、行号范围、次数限制、EOL 保持、原子写入、保留权限与 mtime）。
 用法（dispatcher 侧示例）：
   logf := func(format string, a ...any) { if logger != nil { logger.Log(format, a...) } }
   err := fileops.FileReplace(repo, rel,
       pattern, repl,                  find / repl
       isRegex, icase,                 regex / ci(不区分大小写)
       lineFrom, lineTo,               行范围（1-based，闭区间；0 表示不限）
       count, ensureEOFNewline,        次数上限、末尾换行
       multiline,                      正则 (?m)
       logf,
   )
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

 FileReplace 在文本文件中按选项执行替换。
 参数：
  repo: 仓库根
  rel : 相对路径
  find: 查找（字面或正则）
  repl: 替换文本（来自补丁指令体）
  useRegex: 是否使用正则
  icase:    不区分大小写（字面量：手动实现；正则：通过 (?i)）
  lineFrom/lineTo: 行范围（1-based，闭区间；0=不限）
  count:    替换次数上限（<=0 表示全部）
  ensureEOFNewline: 替换后是否保证末尾有换行
  multiline: 正则多行模式（(?m)）
  logf:     日志函数，可为 nil
func FileReplace(
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

	 读入文件
	data, err := os.ReadFile(abs)
	if err != nil {
		logfSafe(logf, "❌ file.replace 读取失败：%s (%v)", rel, err)
		return err
	}

	 保存原权限与 mtime
	fi, _ := os.Stat(abs)
	mode := os.FileMode(0o644)
	 mtime time.Time
	if fi != nil {
		mode = fi.Mode()
		mtime = fi.ModTime()
	}

	 识别并记录原 EOL（CRLF/LF）
	isCRLF := bytes.Contains(data, []byte("\r\n"))
	text := normalizeLF(string(data))

	 计算行范围
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

	 执行替换
	replaced := 0
	 segOut string

	if useRegex {
		flags := ""
		if icase {
			flags += "(?i)"
		}
		if multiline {
			flags += "(?m)"
		}
		flags += "(?s)"  让 '.' 跨行
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

    if replaced == 0 {
         仅确保 EOF 换行：即使没有文本替换，也要生效
        if ensureEOFNewline && !strings.HasSuffix(text, "\n") {
             恢复原 EOL 风格并原子写入
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
            _ = os.Chmod(tmp, mode)
            if err := os.Rename(tmp, abs); err != nil { logfSafe(logf, "❌ file.replace 覆盖失败：%v", err); return err }
            if !mtime.IsZero() { _ = os.Chtimes(abs, time.Now(), mtime) }

            logfSafe(logf, "✏️ file.replace 确保末尾换行：%s", rel)
            return nil
        }
        logfSafe(logf, "⚠️ file.replace 无匹配：%s（范围 %d-%d）", rel, start, end)
        return nil
    }

	 拼回全文
	 builder strings.Builder
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

	 末尾换行保证
	if ensureEOFNewline && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	 恢复为原 EOL
	if isCRLF {
		result = toCRLF(result)
	}

	 原子写入：临时文件 → fsync → rename
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

 ===== helpers =====

func logfSafe(logf func(string, ...any), format string, a ...any) {
	if logf != nil {
		logf(format, a...)
	}
}

 区分大小写；count<=0 表示全部
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

 不区分大小写；count<=0 表示全部
func replaceCaseInsensitive(s, find, repl string, count int) (string, int) {
	if find == "" {
		return s, 0
	}
	 b strings.Builder
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
		 原文切片按原 s 写回
		b.WriteString(s[i : i+j])
		b.WriteString(repl)
		i += j + len(find)
		repld++
	}
	b.WriteString(s[i:])
	return b.String(), repld
}