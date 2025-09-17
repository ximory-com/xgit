package fileops

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//
// 外部依赖（由上层注入/同包提供）：
//

// RunGit 可选注入：若不为 nil，则用于 stage 文件
var RunGit func(repo string, logger DualLogger, args ...string) (string, error)

// PreflightOne 可选注入：若不为 nil，则用于对单文件做预检
var PreflightOne func(repo, rel string, logger DualLogger) error

//
// 公共小工具
//

// 读为“行模型”：每个元素末尾尽量带 '\n'
func readLines(abs string) ([]string, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Split(bufio.ScanLines)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text()+"\n")
	}
	// 如果文件非空且末尾无 \n，则把最后一行视为“无换行行”
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if len(lines) == 0 {
			lines = []string{""}
		} else {
			lines[len(lines)-1] = strings.TrimRight(lines[len(lines)-1], "\n")
		}
	}
	return lines, nil
}

// 写回（保持 0644；自动建父目录）
func writeLines(abs string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l)
	}
	return os.WriteFile(abs, []byte(sb.String()), 0o644)
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

// 把正文拆成“行模型”（每行都带 '\n'）
func splitPayload(body string) []string {
	if body == "" {
		return nil
	}
	raw := strings.Split(body, "\n")
	out := make([]string, 0, len(raw))
	for i, s := range raw {
		if i == len(raw)-1 {
			if s == "" {
				continue // 正文以 \n 结束，忽略最后这条空行
			}
			out = append(out, s+"\n")
		} else {
			out = append(out, s+"\n")
		}
	}
	return out
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

//
// keys 解析 + “宽松唯一命中”
//

// explodeKeys: 支持多行 / 竖线 / 逗号（全部视为“备选关键词集”）
func explodeKeys(v string) []string {
	v = strings.ReplaceAll(v, "\r\n", "\n")
	v = strings.ReplaceAll(v, "\r", "\n")
	parts := make([]string, 0, 4)
	for _, seg := range strings.Split(v, "\n") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		splitBy := func(x string) bool { return strings.Contains(seg, x) }
		if splitBy("|") {
			for _, s := range strings.Split(seg, "|") {
				ss := strings.TrimSpace(s)
				if ss != "" {
					parts = append(parts, ss)
				}
			}
			continue
		}
		if splitBy(",") {
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

// 在 [from..] 范围内做“宽松唯一命中”：
// 规则：忽略大小写、忽略行首缩进；先尝试“任一 key 唯一命中”；若均不唯一，再尝试“两个 key AND”；再尝试“全部 AND”。
// 返回：绝对行号(1-based)。若多于 1 且 nth>0 则选第 nth；否则报错。
func pickUniqueLoose(lines []string, keys []string, from int, nth int) (int, []int, error) {
	norm := func(s string) string {
		return strings.ToLower(strings.TrimLeft(s, " \t"))
	}
	N := len(lines)
	if from < 1 {
		from = 1
	}
	if from > N {
		return 0, nil, errors.New("起点超界")
	}
	L := make([]string, N)
	for i := 0; i < N; i++ {
		L[i] = norm(lines[i])
	}
	K := make([]string, 0, len(keys))
	for _, k := range keys {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk != "" {
			K = append(K, kk)
		}
	}
	if len(K) == 0 {
		return 0, nil, errors.New("缺少 keys")
	}

	inRange := func(i int) bool { return i+1 >= from }

	// 尝试：任一 key 单独命中
	var cands []int
	for _, k := range K {
		cands = cands[:0]
		for i := 0; i < N; i++ {
			if inRange(i) && strings.Contains(L[i], k) {
				cands = append(cands, i+1)
			}
		}
		if len(cands) == 1 {
			return cands[0], cands, nil
		}
	}

	// 尝试：两两 AND
	if len(K) >= 2 {
		for a := 0; a < len(K); a++ {
			for b := a + 1; b < len(K); b++ {
				cands = cands[:0]
				ka, kb := K[a], K[b]
				for i := 0; i < N; i++ {
					if !inRange(i) {
						continue
					}
					if strings.Contains(L[i], ka) && strings.Contains(L[i], kb) {
						cands = append(cands, i+1)
					}
				}
				if len(cands) == 1 {
					return cands[0], cands, nil
				}
			}
		}
	}

	// 尝试：全部 AND
	cands = cands[:0]
OUTER:
	for i := 0; i < N; i++ {
		if !inRange(i) {
			continue
		}
		for _, k := range K {
			if !strings.Contains(L[i], k) {
				continue OUTER
			}
		}
		cands = append(cands, i+1)
	}
	if len(cands) == 0 {
		return 0, nil, fmt.Errorf("keys 未命中")
	}
	if len(cands) == 1 {
		return cands[0], cands, nil
	}
	if nth > 0 && nth <= len(cands) {
		return cands[nth-1], cands, nil
	}
	return 0, cands, fmt.Errorf("keys 多处命中 %v（可用 nth=1..%d 选择）", cands, len(cands))
}

//
// 作用域（范围）解析：start-keys / end-keys / nthb
//

type scope struct{ start, end int } // [start..end] 闭区间，1-based；end==len(lines) 表示到 EOF

// 解析作用域。无 start-keys → 全文。end-keys 缺省 → 到 EOF。
// start 需要唯一命中；若多处则用 nthb 选择。
// end 在 start 之后搜索；若多处取“第一处”。
func resolveScope(lines []string, args map[string]string) (scope, error) {
	N := len(lines)
	full := scope{start: 1, end: N}

	startKeys := strings.TrimSpace(args["start-keys"])
	if startKeys == "" {
		return full, nil
	}
	keysS := explodeKeys(startKeys)
	nthb := parseInt(args["nthb"])
	si, _, err := pickUniqueLoose(lines, keysS, 1, nthb)
	if err != nil {
		return scope{}, fmt.Errorf("start-keys 定位失败：%v", err)
	}

	endKeys := strings.TrimSpace(args["end-keys"])
	if endKeys == "" {
		return scope{start: si, end: N}, nil
	}
	keysE := explodeKeys(endKeys)
	// end 从 si+1 开始找；允许多处，取第一处
	ei, list, err := pickUniqueLoose(lines, keysE, si+1, 1)
	if err != nil {
		return scope{}, fmt.Errorf("end-keys 定位失败：%v", err)
	}
	_ = list // 仅用于调试时查看
	if ei < si {
		return scope{}, fmt.Errorf("非法范围：end(%d) < start(%d)", ei, si)
	}
	return scope{start: si, end: ei}, nil
}

//
// 行定位（在作用域内进一步定位）：lineno / keys / nthl / offset
//

// resolveLineInScope: 在作用域内找到目标“基准行”
// 1) 若提供 lineno → 直接使用“相对作用域”的行号（1-based）
// 2) 否则用 keys（宽松唯一命中），若多处 → 用 nthl 选择
// 3) 若提供 offset（仅当无作用域时有效），在基准行上做 ± 偏移
func resolveLineInScope(lines []string, sc scope, args map[string]string) (int, error) {
	relLine := parseInt(args["lineno"])
	keys := strings.TrimSpace(args["keys"])
	nthl := parseInt(args["nthl"])
	offset := 0
	offRaw := strings.TrimSpace(args["offset"])
	if offRaw != "" && (strings.HasPrefix(offRaw, "+") || strings.HasPrefix(offRaw, "-")) {
		sign := 1
		s := offRaw
		if s[0] == '+' {
			s = s[1:]
		} else if s[0] == '-' {
			sign = -1
			s = s[1:]
		}
		offset = parseInt(s) * sign
	}

	// 有作用域时，不允许 offset
	hasScope := !(sc.start == 1 && sc.end == len(lines))
	if hasScope && offRaw != "" {
		return 0, errors.New("offset 仅在无作用域时可用")
	}

	// 1) lineno 优先（范围内相对行号）
	if relLine > 0 {
		abs := sc.start + relLine - 1
		if abs < sc.start || abs > sc.end {
			return 0, fmt.Errorf("lineno=%d 超出作用域范围 [%d..%d]", relLine, sc.start, sc.end)
		}
		return abs, nil
	}

	// 2) keys 宽松匹配（仅在作用域内）
	if keys == "" {
		return 0, errors.New("缺少 lineno 或 keys")
	}
	K := explodeKeys(keys)
	// 在 [sc.start..sc.end] 内找
	idx, cands, err := pickUniqueLoose(lines, K, sc.start, nthl)
	if err != nil {
		return 0, fmt.Errorf("keys 定位失败：%v", err)
	}
	if idx > sc.end { // pickUniqueLoose 可能跨出范围（极端），再兜底
		return 0, fmt.Errorf("keys 命中行 %d 超出作用域 [%d..%d]", idx, sc.start, sc.end)
	}
	_ = cands

	// 3) offset（仅无作用域）
	if !hasScope && offset != 0 {
		dst := idx + offset
		if dst < 1 || dst > len(lines) {
			return 0, fmt.Errorf("offset 后行号超界（%d）", dst)
		}
		return dst, nil
	}
	return idx, nil
}

// stage+预检（若外部注入）
func stageAndPreflight(repo, rel string, logger DualLogger) error {
	if RunGit != nil {
		_, _ = RunGit(repo, logger, "add", "--", rel)
	}
	if PreflightOne != nil {
		if err := PreflightOne(repo, rel, logger); err != nil {
			if logger != nil {
				logger.Log("❌ 预检失败：%s (%v)", rel, err)
			}
			return err
		}
	}
	return nil
}
