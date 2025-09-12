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
	Cmd   string            // 指令名：file.write / file.replace / block.xxx / ...
	Path  string            // 目标路径（仅 file.*）；必须由 header 中的 "双引号" 包裹给出
	Body  string            // 正文（对 write/append/prepend/image/binary/diff/replace 等需要体的指令）
	Args  map[string]string // 参数：全部来自 body 中的 @key … @end
	Index int               // 预留
}
type Patch struct {
	Ops []*FileOp
}
// XGIT:END PARSER TYPES

// XGIT:BEGIN PARSER
// 新协议要点：
// 1) header 仅允许一个参数：
//    - file.*   ："绝对或相对路径"，必须双引号包裹
//    - block.*  ："块名称"，必须双引号包裹
//    任何其它 header 内的 kv 或多余内容 → 直接报错
// 2) 除 path/名称外，其它参数全部放入 body，采用 @key … @end 形式（多行安全）
//    例如：
//      @pattern
//      ^foo.*$
//      @end
//      @with
//      bar
//      @end
//      @regex
//      true
//      @end
// 3) 对 write/append/prepend/image/binary/diff：正文内容默认就是 Body；也支持 @content 块覆盖。
// 4) 对 replace：replacement 优先取 @with；若缺失则使用 Body 作为替换体。
// 5) 对 move：目标路径从 @to 获取。
func ParsePatch(data string, eof string) (*Patch, error) {
	text := strings.ReplaceAll(data, "\r", "")
	lines := strings.Split(text, "\n")

	// 严格 EOF：最后一个非空白行必须等于 eof
	if lastMeaningfulLine([]byte(data)) != eof {
		return nil, fmt.Errorf("严格 EOF 校验失败：期望 %q", eof)
	}

	var (
		p          = &Patch{Ops: make([]*FileOp, 0, 64)}
		inBody     = false
		cur        *FileOp
		// 统一头部：=== <kind>.<cmd>: <header> ===
		reHead 	   = regexp.MustCompile(`^===\s*([a-z]+(?:\.[a-z_]+)?)\s*:\s*(.*?)\s*===\s*$`)
	)

	startWrite := func(cmd, headerRaw string) error {
		// 收口前一块
		if inBody && cur != nil {
			p.Ops = append(p.Ops, cur)
			inBody = false
			cur = nil
		}
		cmd = strings.ToLower(strings.TrimSpace(cmd))
		cur = &FileOp{Cmd: cmd, Args: map[string]string{}}

		// 解析 header：仅允许一个被双引号包裹的参数
		header := strings.TrimSpace(headerRaw)
		pathOrName, rest := splitFirstField(header)
		if rest != "" {
			return fmt.Errorf("header 仅允许一个参数，发现多余内容：%q", rest)
		}
		val, ok := mustDoubleQuoted(pathOrName)
		if !ok {
			return fmt.Errorf("path/name 必须用双引号包裹：%q", pathOrName)
		}

		// 分类：file.* 用 Path；block.* 用 Args["block_name"]
		kind := strings.SplitN(cmd, ".", 2)[0]
		if kind == "file" {
			cur.Path = val
		} else if kind == "block" {
			cur.Args["block_name"] = val
		} else {
			// 其它命名空间：按需拓展；默认也存到 Args["name"]
			cur.Args["name"] = val
		}

		// 是否需要正文体？
		switch cmd {
		case "file.delete", "file.chmod", "file.eol", "file.move":
			// 这些指令不强制正文，但允许 body 参数块（例如 file.move 的 @to）
			inBody = true
		case "file.replace", "file.write", "file.append", "file.prepend", "file.image", "file.binary", "file.diff":
			inBody = true
		default:
			// 其它未知命令：允许 body，交由上层决定
			inBody = true
		}
		return nil
	}

	// 逐行扫描
	var bodyBuf []string
	var inParam bool
	var paramKey string
	var paramBuf []string

	flushBlock := func() {
		if cur != nil {
			// 参数块优先：若存在 @content，则 Body 取其值
			if content, ok := cur.Args["@content"]; ok {
				cur.Body = content
				delete(cur.Args, "@content")
			}
			// 对 replace：替换体优先 @with；否则用 Body
			if cur.Cmd == "file.replace" {
				if with, ok := cur.Args["@with"]; ok {
					cur.Body = with
					delete(cur.Args, "@with")
				}
			}
			p.Ops = append(p.Ops, cur)
		}
		inBody = false
		cur = nil
		bodyBuf = bodyBuf[:0]
		inParam = false
		paramKey = ""
		paramBuf = paramBuf[:0]
	}

	setArg := func(k, v string) {
		if k == "" {
			return
		}
		// 所有参数 key 统一小写；保留 @pattern/@with/@content 以便后处理
		lk := strings.ToLower(k)
		switch lk {
		case "@pattern", "@with", "@content", "@to":
			cur.Args[lk] = v
		default:
			cur.Args[lk] = strings.TrimSpace(v)
		}
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\n")

		// 头部
		if m := reHead.FindStringSubmatch(line); len(m) == 3 {
			if err := startWrite(m[1], m[2]); err != nil {
				return nil, err
			}
			continue
		}
		// 尾部
		if strings.TrimSpace(line) == "=== end ===" {
			if inBody && cur != nil {
				// 先收尾最后一个未闭合的参数块
				if inParam {
					setArg(paramKey, strings.Join(paramBuf, "\n"))
					inParam = false
					paramKey = ""
					paramBuf = paramBuf[:0]
				}
				// 若无 @content 且不是 replace 的 @with 模式，则把 bodyBuf 作为 Body
				if cur.Body == "" && len(bodyBuf) > 0 {
					cur.Body = strings.Join(bodyBuf, "\n") + "\n"
				}
				flushBlock()
			}
			continue
		}

		// body：参数块语法
		if inBody && cur != nil {
			trim := strings.TrimSpace(line)
			// 开始参数块：@key
			if !inParam && strings.HasPrefix(trim, "@") && trim != "@end" {
				inParam = true
				paramKey = trim
				paramBuf = paramBuf[:0]
				continue
			}
			// 结束参数块：@end
			if inParam && trim == "@end" {
				setArg(paramKey, strings.Join(paramBuf, "\n"))
				inParam = false
				paramKey = ""
				paramBuf = paramBuf[:0]
				continue
			}
			// 收集参数块内容 / 普通正文
			if inParam {
				paramBuf = append(paramBuf, line)
			} else {
				bodyBuf = append(bodyBuf, line)
			}
		}
	}

	return p, nil
}

// 工具：切第一个“词”（按空白），返回 first, rest
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

// 工具：双引号强校验；返回内部文本与是否有效
func mustDoubleQuoted(s string) (string, bool) {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1:len(s)-1], true
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
