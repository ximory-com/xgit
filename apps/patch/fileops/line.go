package fileops

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 依赖于外层提供：
// - type DualLogger interface{ Log(format string, a ...any) }
// - runGit(repo string, logger DualLogger, args ...string) (string, error)
//
// 约定：icase(默认1)、ensure_nl(默认1)、allow_noop(默认0)

func LineInsertBefore(repo, rel string, body string, args map[string]string, logger DualLogger) error {
	loc, err := resolveLine(repo, rel, args)
	if err != nil {
		return fmt.Errorf("line.insert_before: %w", err)
	}
	lines, e := readLines(filepath.Join(repo, rel))
	if e != nil {
		return e
	}
	insert := splitPayload(body)
	lines = insertAt(lines, loc-1, insert) // before → 在目标行前插
	if ensureNL(args, true) {
		lines = ensureTrailingNL(lines)
	}
	if err := writeLines(filepath.Join(repo, rel), lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("➕ insert_before %s:L%d (+%d)", rel, loc, len(insert))
	}
	_, _ = runGit(repo, logger, "add", "--", rel)
	return nil
}

func LineInsertAfter(repo, rel string, body string, args map[string]string, logger DualLogger) error {
	loc, err := resolveLine(repo, rel, args)
	if err != nil {
		return fmt.Errorf("line.insert_after: %w", err)
	}
	lines, e := readLines(filepath.Join(repo, rel))
	if e != nil {
		return e
	}
	insert := splitPayload(body)
	lines = insertAt(lines, loc, insert) // after → 在目标行后插
	if ensureNL(args, true) {
		lines = ensureTrailingNL(lines)
	}
	if err := writeLines(filepath.Join(repo, rel), lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("➕ insert_after  %s:L%d (+%d)", rel, loc, len(insert))
	}
	_, _ = runGit(repo, logger, "add", "--", rel)
	return nil
}

func LineReplaceLine(repo, rel string, body string, args map[string]string, logger DualLogger) error {
	loc, err := resolveLine(repo, rel, args)
	if err != nil {
		return fmt.Errorf("line.replace_line: %w", err)
	}
	lines, e := readLines(filepath.Join(repo, rel))
	if e != nil {
		return e
	}
	newLines := splitPayload(body)
	old := lines[loc-1 : loc] // 仅做日志/幂等比对
	noop := len(newLines) == 1 && strings.TrimRight(newLines[0], "\n") == strings.TrimRight(lines[loc-1], "\n")
	if noop && !allowNoop(args) {
		if logger != nil {
			logger.Log("ℹ️ replace_line noop：%s:L%d 内容未变化", rel, loc)
		}
		return nil
	}
	// 用新行替换“这一行”，注意 replace_line 语义是“整行替换”，但允许多行（按你的规范）
	lines = splice(lines, loc-1, 1, newLines)
	if ensureNL(args, true) {
		lines = ensureTrailingNL(lines)
	}
	if err := writeLines(filepath.Join(repo, rel), lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("✏️ replace_line %s:L%d (1→%d)", rel, loc, len(newLines))
		logger.Log("   -old: %q", strings.TrimRight(old[0], "\n"))
		if len(newLines) == 1 {
			logger.Log("   +new: %q", strings.TrimRight(newLines[0], "\n"))
		} else {
			logger.Log("   +new: %d lines", len(newLines))
		}
	}
	_, _ = runGit(repo, logger, "add", "--", rel)
	return nil
}

func LineDeleteLine(repo, rel string, args map[string]string, logger DualLogger) error {
	loc, err := resolveLine(repo, rel, args)
	if err != nil {
		return fmt.Errorf("line.delete_line: %w", err)
	}
	lines, e := readLines(filepath.Join(repo, rel))
	if e != nil {
		return e
	}
	if loc < 1 || loc > len(lines) {
		if allowNoop(args) {
			if logger != nil {
				logger.Log("ℹ️ delete_line noop：%s:L%d 超界/不存在", rel, loc)
			}
			return nil
		}
		return fmt.Errorf("delete_line: %s:L%d 超界/不存在", rel, loc)
	}
	removed := strings.TrimRight(lines[loc-1], "\n")
	lines = splice(lines, loc-1, 1, nil)
	if ensureNL(args, true) {
		lines = ensureTrailingNL(lines)
	}
	if err := writeLines(filepath.Join(repo, rel), lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("🗑️ delete_line  %s:L%d (-1) '%s'", rel, loc, removed)
	}
	_, _ = runGit(repo, logger, "add", "--", rel)
	return nil
}

// ---------- 定位/辅助 ----------

func resolveLine(repo, rel string, args map[string]string) (int, error) {
	path := filepath.Join(repo, rel)
	lines, err := readLines(path)
	if err != nil {
		return 0, err
	}
	// 1) 行号优先
	if n := parseInt(args["lineno"]); n > 0 {
		if n < 1 || n > len(lines) {
			return 0, fmt.Errorf("定位失败：行号 %d 超界（1..%d）", n, len(lines))
		}
		return n, nil
	}
	// 2) keys 宽松 AND 定位（忽略大小写、忽略行首缩进）
	var keys []string
	if v := strings.TrimSpace(args["keys"]); v != "" {
		keys = explodeKeys(v)
	}
	if len(keys) == 0 {
		return 0, errors.New("定位失败：缺少 lineno>0 或 keys")
	}
	icase := parseBoolDefault1(args["icase"])
	hits := make([]int, 0, 1)
	for i, raw := range lines {
		cand := strings.TrimLeft(raw, " \t")
		if icase {
			cand = strings.ToLower(cand)
		}
		ok := true
		for _, k := range keys {
			kk := k
			if icase {
				kk = strings.ToLower(kk)
			}
			if !strings.Contains(cand, kk) {
				ok = false
				break
			}
		}
		if ok {
			hits = append(hits, i+1)
		}
	}
	switch len(hits) {
	case 0:
		return 0, fmt.Errorf("定位失败：keys 未命中。样本前后：\n%s", sampleAround(lines, keys, 2))
	case 1:
		return hits[0], nil
	default:
		if len(hits) > 5 {
			hits = hits[:5]
		}
		return 0, fmt.Errorf("定位失败：多处命中 %v，请增加 keys 或改用 lineno", hits)
	}
}

func readLines(abs string) ([]string, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	// 统一读取为“以 \n 结尾的行切片”
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Split(bufio.ScanLines)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text()+"\n")
	}
	// 如果源文件为空或不以 \n 结尾，Scanner 不会补最后一行的 \n，这里手动处理：
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if len(lines) == 0 {
			lines = []string{""}
		} else {
			lines[len(lines)-1] = strings.TrimRight(lines[len(lines)-1], "\n")
		}
	}
	return lines, nil
}

func writeLines(abs string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	sb := strings.Builder{}
	for _, l := range lines {
		sb.WriteString(l)
	}
	return os.WriteFile(abs, []byte(sb.String()), 0o644)
}

func splitPayload(body string) []string {
	// 原样保留，确保每行带 \n；空正文返回空切片（允许插入空）
	if body == "" {
		return nil
	}
	// 规范：以 \n 分割，保留 \n
	raw := strings.Split(body, "\n")
	out := make([]string, 0, len(raw))
	for i, s := range raw {
		if i == len(raw)-1 {
			// body 末尾可能有/没有 \n；如果没有，补一个，以保证行模型一致
			if s == "" {
				// 末尾空行 → 代表 body 以 \n 结束，上一行已带 \n，这里忽略
				continue
			}
			out = append(out, s+"\n")
		} else {
			out = append(out, s+"\n")
		}
	}
	return out
}

func insertAt(lines []string, idx int, insert []string) []string {
	if idx < 0 {
		idx = 0
	}
	if idx > len(lines) {
		idx = len(lines)
	}
	head := append([]string{}, lines[:idx]...)
	tail := append([]string{}, lines[idx:]...)
	return append(append(head, insert...), tail...)
}

func splice(lines []string, start, del int, insert []string) []string {
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := start + del
	if end > len(lines) {
		end = len(lines)
	}
	head := append([]string{}, lines[:start]...)
	tail := append([]string{}, lines[end:]...)
	return append(append(head, insert...), tail...)
}

func ensureTrailingNL(lines []string) []string {
	if len(lines) == 0 {
		return []string{""}
	}
	last := lines[len(lines)-1]
	if !strings.HasSuffix(last, "\n") {
		lines[len(lines)-1] = last + "\n"
	}
	return lines
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
func parseBoolDefault1(s string) bool {
	if strings.TrimSpace(s) == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
func allowNoop(args map[string]string) bool {
	switch strings.ToLower(strings.TrimSpace(args["allow_noop"])) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func explodeKeys(v string) []string {
	v = strings.ReplaceAll(v, "\r\n", "\n")
	v = strings.ReplaceAll(v, "\r", "\n")
	// 支持三种：多行、竖线分隔、逗号分隔（宽松）
	parts := make([]string, 0, 4)
	for _, seg := range strings.Split(v, "\n") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if strings.Contains(seg, "|") {
			for _, s := range strings.Split(seg, "|") {
				ss := strings.TrimSpace(s)
				if ss != "" {
					parts = append(parts, ss)
				}
			}
			continue
		}
		if strings.Contains(seg, ",") {
			for _, s := range strings.Split(seg, ",") {
				ss := strings.TrimSpace(s)
				if ss != "" {
					parts = append(parts, ss)
				}
			}
			continue
		}
		parts = append(parts, seg)
	}
	return parts
}

func sampleAround(lines []string, keys []string, k int) string {
	// 简化：返回文件头尾各 k 行（避免再次模糊匹配）
	sb := strings.Builder{}
	max := len(lines)
	if k > max {
		k = max
	}
	sb.WriteString("  [head]\n")
	for i := 0; i < k; i++ {
		fmt.Fprintf(&sb, "   %4d| %s", i+1, lines[i])
	}
	sb.WriteString("  [tail]\n")
	for i := max - k; i < max; i++ {
		if i < 0 {
			continue
		}
		fmt.Fprintf(&sb, "   %4d| %s", i+1, lines[i])
	}
	return sb.String()
}
