package main

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// XGIT:BEGIN PARSER TYPES
type FileOp struct {
	Cmd   string            // 形如 "file.write" / "file.move" 等
	Path  string            // 目标路径；move 时为源路径
	To    string            // move 目标路径
	Body  string            // 需要主体的指令：write/append/prepend/image/binary/diff/replace(可选)
	Args  map[string]string // 通用参数：kv 形式（mode/style/pattern/to/flags/...）
	Index int               // 预留
}
type Patch struct {
	Ops []*FileOp
}
// XGIT:END PARSER TYPES

// XGIT:BEGIN PARSER
// 解析补丁（支持 11 条 file.* 指令）
// 头：=== file.<cmd>: <header> ===
// 体：可选（需要体的命令才读）
// 尾：=== end ===
func ParsePatch(data string, eof string) (*Patch, error) {
	// 统一换行
	text := strings.ReplaceAll(data, "\r", "")
	lines := strings.Split(text, "\n")

	// 严格 EOF：最后一个非空行必须等于 eof
	last := ""
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" {
			last = t
		}
	}
	if last != eof {
		return nil, fmt.Errorf("严格 EOF 校验失败：期望 %q，实际 %q", eof, last)
	}

	var (
		p         = &Patch{Ops: make([]*FileOp, 0, 64)}
		inBody    = false
		cur       *FileOp
		// 统一头部：=== <kind>.<cmd>: <header> ===
		reHead    = regexp.MustCompile(`^===\s*([a-z]+(?:\.[a-z_]+)?)\s*:\s*(.*?)\s*===\s*$`)
		reMoveSep = regexp.MustCompile(`\s*->\s*`)
	)

	startWrite := func(cmd, header string) {
		// 收口之前的块（如果在 body 中）
		if inBody && cur != nil {
			p.Ops = append(p.Ops, cur)
			inBody = false
			cur = nil
		}
		cur = &FileOp{Cmd: strings.ToLower(cmd), Args: map[string]string{}}

		h := strings.TrimSpace(header)
		kind, action := "", cur.Cmd
		if i := strings.Index(cur.Cmd, "."); i > -1 {
			kind = cur.Cmd[:i]
			action = cur.Cmd[i+1:]
		} else {
			// 兼容意外输入（无点），当成单段指令
			kind = cur.Cmd
		}

		if kind != "file" {
			// 暂只支持 file.*
			return
		}

		switch action {
		case "move":
			// 1) 先尝试 A -> B 语法
			if parts := reMoveSep.Split(h, 2); len(parts) == 2 {
				cur.Path = strings.TrimSpace(trimQuotes(parts[0]))
				cur.To = strings.TrimSpace(trimQuotes(parts[1]))
			} else {
				// 2) 退化到 path + kv（优先把第一段非 kv 当作路径，余下解析成 kv）
				path2, kv2 := splitPathAndKVs(h)
				cur.Path = path2
				if v, ok := kv2["to"]; ok {
					cur.To = v
					delete(kv2, "to")
				}
				// 其余 kv 丢进 Args（命令未使用则忽略，不污染路径）
				for k, v := range kv2 {
					cur.Args[k] = v
				}
			}
			// move 无体
			p.Ops = append(p.Ops, cur)
			cur = nil
			inBody = false

		case "delete", "chmod", "eol", "replace":
			// 统一解析：取首段非 kv 作为路径；其余 key=value 进入 Args
			path, kv := splitPathAndKVs(h)
			cur.Path = path
			for k, v := range kv {
				cur.Args[k] = v
			}
			// delete / chmod / eol：无体；replace 体可选（若无 to/with，则 body 作为 replacement）
			if action == "replace" {
				inBody = (cur.Args["to"] == "" && cur.Args["with"] == "")
			} else {
				p.Ops = append(p.Ops, cur)
				cur = nil
				inBody = false
			}

		default:
			// write/append/prepend/image/binary/diff ……需要体；头部可能跟无关 kv（忽略到 Args）
			pth, kv := splitPathAndKVs(h)
			cur.Path = pth
			for k, v := range kv {
				cur.Args[k] = v
			}
			inBody = true
		}
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\n")

		if m := reHead.FindStringSubmatch(line); len(m) == 3 {
			startWrite(m[1], m[2])
			continue
		}
		if strings.TrimSpace(line) == "=== end ===" {
			if inBody && cur != nil {
				p.Ops = append(p.Ops, cur)
				inBody = false
				cur = nil
			}
			continue
		}
		if inBody && cur != nil {
			cur.Body += line + "\n"
		}
	}
	return p, nil
}

// --- 解析辅助 ---

// splitFirstField: 老实现（保留以防他处复用）
func splitFirstField(h string) (first string, rest string) {
	h = strings.TrimSpace(h)
	if h == "" {
		return "", ""
	}
	sc := bufio.NewScanner(strings.NewReader(h))
	sc.Split(bufio.ScanWords)
	if sc.Scan() {
		first = sc.Text()
		rest = strings.TrimSpace(strings.TrimPrefix(h, first))
	}
	return first, rest
}

// parseKVs: 仅解析 k=v（v 可带引号），忽略非 k=v token
func parseKVs(s string) map[string]string {
	m := map[string]string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return m
	}
	// 以空格分割多个 kv；kv 为 k=v（v 允许引号）
	toks := splitBySpacesRespectQuotes(s)
	for _, t := range toks {
		if k, v, ok := cutKV(t); ok {
			m[strings.ToLower(k)] = trimQuotes(v)
		}
	}
	return m
}

// splitPathAndKVs 将 header 拆成：路径（支持带空格/引号）+ 参数 kv
// 规则：按 splitBySpacesRespectQuotes 切分 token；从头累积“非 k=v” token 构成路径，直到遇到第一个 k=v；后续均当 kv。
func splitPathAndKVs(s string) (string, map[string]string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", map[string]string{}
	}
	toks := splitBySpacesRespectQuotes(s)

	pathTokens := make([]string, 0, len(toks))
	kvStart := -1
	for i, t := range toks {
		if _, _, ok := cutKV(t); ok {
			kvStart = i
			break
		}
		pathTokens = append(pathTokens, t)
	}
	// 组装 kv
	kv := map[string]string{}
	if kvStart != -1 {
		for _, t := range toks[kvStart:] {
			if k, v, ok := cutKV(t); ok {
				kv[strings.ToLower(strings.TrimSpace(k))] = trimQuotes(v)
			}
		}
	}
	// 路径 = 去引号后再用单个空格拼接（保持用户原始空格数量对语义无影响）
	path := strings.TrimSpace(strings.Join(pathTokens, " "))
	path = trimQuotes(path)
	return path, kv
}

func cutKV(s string) (k, v string, ok bool) {
	if i := strings.IndexByte(s, '='); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
	}
	return "", "", false
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func splitBySpacesRespectQuotes(s string) []string {
	var out []string
	var b strings.Builder
	inQ := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQ == 0 && (c == '"' || c == '\'') {
			inQ = c
			b.WriteByte(c)
			continue
		}
		if inQ != 0 {
			b.WriteByte(c)
			if c == inQ {
				inQ = 0
			}
			continue
		}
		if c == ' ' || c == '\t' {
			if b.Len() > 0 {
				out = append(out, strings.TrimSpace(b.String()))
				b.Reset()
			}
			continue
		}
		b.WriteByte(c)
	}
	if b.Len() > 0 {
		out = append(out, strings.TrimSpace(b.String()))
	}
	return out
}

// XGIT:BEGIN GO:FUNC_LAST_MEANINGFUL_LINE
// lastMeaningfulLine 返回字节流中“最后一个非空白行”（去掉行尾 \r）的原样文本。
// 说明：用于严格 EOF 校验，配合 watcher 在进入执行前做最后一道门。
func lastMeaningfulLine(b []byte) string {
	last := ""
	start := 0
	for start <= len(b) {
		// 逐行切分（按 \n），兼容最后一行无换行
		nl := bytes.IndexByte(b[start:], '\n')
		var line []byte
		if nl < 0 {
			line = b[start:]
			start = len(b) + 1
		} else {
			line = b[start : start+nl]
			start += nl + 1
		}
		// 去掉行尾 \r
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if strings.TrimSpace(string(line)) != "" {
			last = string(line)
		}
	}
	return last
}
// XGIT:END GO:FUNC_LAST_MEANINGFUL_LINE
// XGIT:END PARSER
