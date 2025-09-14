package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// XGIT:BEGIN PARSER TYPES
type FileOp struct {
	Cmd   string            // 指令名：file.write / file.replace / ...
	Path  string            // 目标路径（由 header 的 "双引号" 提供）
	Body  string            // 正文（write/append/prepend 的默认内容；replace 无 with 时可用）
	Args  map[string]string // 参数区解析的 K=V 与多行块（键一律小写）
	Index int               // 预留
}
// 在定义 Patch 的文件里，把 Patch 改成这样
type Patch struct {
    Ops       []*FileOp
    CommitMsg string // 可选：提交说明
    Author    string // 可选：提交作者（形如 "Name <email>"）
	Repo      string // 仓库名
}

// XGIT:END PARSER TYPES

// XGIT:BEGIN PARSER
// 协议摘要：
//  Header:  必须且仅 1 个双引号参数（路径/名称），示例：=== file.replace: "path with spaces.txt" ===
//  Params:  出现在块体的最前部，直到遇到第一行“非参数”即结束参数区。两种形式：
//           1) 单行 K=V
//           2) 多行 K</>K：开始行 "K<"，结束行独占一行 ">K"；多行块内“非空行”必须以 1 个空格开头（缩进保护），解析时会剥掉该 1 个空格。
//           同键多次赋值时，后者覆盖前者。
//  Body:    参数区结束后至 "=== end ===" 的全部行（原样收集）。
//  EOF:     严格校验最后一个非空白行等于传入 eof（通常是 "=== PATCH EOF ==="）。
// ParsePatch 解析补丁文本为 Patch 结构
func ParsePatch(data string, eof string) (*Patch, error) {
	text := strings.ReplaceAll(data, "\r", "")
	if lastMeaningfulLine([]byte(text)) != eof {
		return nil, fmt.Errorf("严格 EOF 校验失败：期望 %q，实际 %q", eof, lastMeaningfulLine([]byte(text)))
	}
	lines := strings.Split(text, "\n")

	// 头部匹配
	reHead := regexp.MustCompile(`^===\s*([a-z]+(?:\.[a-z_]+)?)\s*:\s*(.*?)\s*===\s*$`)
	reKV := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*)$`) // 顶层 KV：commitmsg/author/repo

	// 参数识别（块内）
	reParamKV := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)
	reBlkStart := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)<$`)
	endMarker := func(key string) string { return ">" + key }

	var (
		p          = &Patch{Ops: make([]*FileOp, 0, 64)}
		cur        *FileOp
		inBody     = false
		paramsDone = false
		inHeader   = true // 第一个块开始前，解析顶层 KV
	)

	flush := func() {
		if inBody && cur != nil {
			p.Ops = append(p.Ops, cur)
		}
		cur = nil
		inBody = false
		paramsDone = false
	}

	startWrite := func(cmd, headerRaw string) error {
		flush()
		inHeader = false // 一旦进入块解析，头部 KV 就结束了

		cmd = strings.ToLower(strings.TrimSpace(cmd))
		header := strings.TrimSpace(headerRaw)
		val, ok := mustDoubleQuoted(header)
		if !ok {
			return fmt.Errorf("path/name 必须用双引号包裹：%q", header)
		}
		cur = &FileOp{
			Cmd:  cmd,
			Path: val,
			Args: map[string]string{},
			Body: "",
		}
		inBody = true
		return nil
	}

	// 主循环
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// 顶层 KV（只在第一个块前解析）
		if inHeader {
			if m := reKV.FindStringSubmatch(line); len(m) == 3 {
				key := strings.ToLower(strings.TrimSpace(m[1]))
				val := strings.TrimSpace(m[2])
				switch key {
				case "commitmsg":
					p.CommitMsg = val
					continue
				case "author":
					p.Author = val
					continue
				case "repo":
					p.Repo = val
					continue
				}
			}
		}

		// 匹配块头
		if m := reHead.FindStringSubmatch(line); len(m) == 3 {
			if err := startWrite(m[1], m[2]); err != nil {
				return nil, err
			}
			continue
		}

		// 结束一个块
		if strings.TrimSpace(line) == "=== end ===" {
			flush()
			continue
		}

		// 仅在块内解析
		if !inBody || cur == nil {
			continue
		}

		if !paramsDone {
			// 多行参数块 KEY<
			if m := reBlkStart.FindStringSubmatch(line); len(m) == 2 {
				key := strings.ToLower(m[1])
				end := endMarker(m[1])
				var b strings.Builder
				for j := i + 1; j < len(lines); j++ {
					l := lines[j]
					if l == end {
						cur.Args[key] = b.String()
						i = j
						goto nextLine
					}
					if strings.TrimSpace(l) == "" {
						b.WriteString("\n")
						continue
					}
					if !strings.HasPrefix(l, " ") {
						return nil, fmt.Errorf("多行块 %s< 的正文非空行必须以空格开头（行 %d）", key, j+1)
					}
					b.WriteString(l[1:])
					b.WriteString("\n")
				}
				return nil, fmt.Errorf("多行块 %s< 未找到结束标记 %s", key, end)
			}

			// 单行参数 K=V
			if m := reParamKV.FindStringSubmatch(line); len(m) == 3 {
				key := strings.ToLower(m[1])
				val := strings.TrimRight(m[2], "\n")
				cur.Args[key] = val
				continue
			}

			paramsDone = true
		}

		// 正文
		cur.Body += line + "\n"
	nextLine:
	}

	flush()
	return p, nil
}

// mustDoubleQuoted: 若 s 为 "xxxx" 形式，返回去引号的值和 true；否则 false
func mustDoubleQuoted(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1], true
	}
	return "", false
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
