package main

import (
	"bytes"
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// XGIT:BEGIN PARSER TYPES
type FileOp struct {
	Cmd   string            // write/append/prepend/replace/delete/move/chmod/eol/image/binary/diff
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
	text := strings.ReplaceAll(data, "\r", "")
	lines := strings.Split(text, "\n")

	// 严格 EOF：最后一个非空行
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
		p          = &Patch{Ops: make([]*FileOp, 0, 64)}
		inBody     = false
		cur        *FileOp
		// 统一头部：=== <kind>.<cmd>: <header> ===
		reHead 	   = regexp.MustCompile(`^===\s*([a-z]+(?:\.[a-z_]+)?)\s*:\s*(.*?)\s*===\s*$`)
		reMoveSep  = regexp.MustCompile(`\s*->\s*`)
	)

	startWrite := func(cmd, header string) {
		// 收口之前的块
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

		switch kind {
		case "file":
			switch action {
			case "move":
				parts := reMoveSep.Split(h, 2)
				if len(parts) == 2 {
					cur.Path = strings.TrimSpace(parts[0])
					cur.To = strings.TrimSpace(parts[1])
				} else {
					cur.Path = h
				}
				// move 无体
				p.Ops = append(p.Ops, cur)
				cur = nil
				inBody = false
			case "delete", "chmod", "eol", "replace":
				// delete: 只要 path
				// chmod/eol/replace: header 支持 path 后跟 kv
				// 先切出 path（首个空格前）
				path, rest := splitFirstField(h)
				cur.Path = path
				kv := parseKVs(rest)
				if len(kv) > 0 {
					for k, v := range kv {
						cur.Args[k] = v
					}
				}
				// delete/ chmod / eol：无体；replace 既可 header 给 replacement，也可 body 给 replacement
				if cur.Cmd == "replace" {
					// 需要体？看有没有 to/with 参数；没有则体作为 replacement
					inBody = (cur.Args["to"] == "" && cur.Args["with"] == "")
				} else if cur.Cmd == "delete" {
					p.Ops = append(p.Ops, cur)
					cur = nil
					inBody = false
				} else { // chmod/eol
					p.Ops = append(p.Ops, cur)
					cur = nil
					inBody = false
				}
			default:
				// write/append/prepend/image/binary/diff ……需要体
				cur.Path = h
				inBody = true
			}
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

func cutKV(s string) (k, v string, ok bool) {
	if i := strings.IndexByte(s, '='); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
	}
	return "", "", false
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1:len(s)-1]
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
