package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// =======================================================
// XGIT 解析器 v2（仅支持 11 条 file.* 指令；明确移除旧版 `file:`）
//
// 支持的头：
//   === file.<op>: <head> ===
//   <op> ∈ { write | append | prepend | replace | delete | move | copy | mkdir | chmod | eol | binary }
//
// 统一严格 EOF：最后一个非空行必须等于传入的 eofMark。
// 写类操作（write/append/prepend/replace）为了兼容现有 apply，会同步填充到 Patch.Files。
// 其余操作进入 Patch.Ops，后续由 apply 分派执行。
// =======================================================

// ----- 公开类型（与现有代码对齐）-----
type Patch struct {
	Commit string
	Author string
	Files  []FileChunk   // 仅写类操作会落这里（write/append/prepend/replace），便于旧 apply 过渡
	Blocks []BlockChunk  // 预留：区块功能后续接入
	Ops    []FileOp      // 新：完整的 11 类文件操作
}

// 兼容旧结构：文件块
type FileChunk struct {
	Path    string
	Content string
	Mode    string // write/append/prepend/replace
}

// 兼容旧结构：区块（预留）
type BlockChunk struct {
	Path   string
	Anchor string
	Mode   string // replace/append/prepend/append_once
	Index  int
	Body   string
}

// 新增：统一文件操作描述
type FileOp struct {
	Op string // write/append/prepend/replace/delete/move/copy/mkdir/chmod/eol/binary

	// 路径相关
	Path string // 目标路径
	From string // move/copy 源路径
	Dir  string // mkdir 目录路径

	// 内容相关
	Body string // write/append/prepend/replace/binary 的正文（binary 为 Base64 文本）

	// 替换相关（file.replace）
	Find        string
	ReplaceWith string
	Regex       bool
	Flags       string // i,m,s 等

	// chmod/eol 参数
	Chmod   string // "+x" / "755" / "644" 等
	EOL     string // "lf" / "crlf"
	EnsureNL bool  // true => 末行补换行

	// 写类模式（冗余，便于旧逻辑）
	Mode string
}

// ----- 指令头正则 -----
var (
	// 新版：=== file.<op>: head ===
	rHeadFileOp = regexp.MustCompile(`^===\s*file\.(write|append|prepend|replace|delete|move|copy|mkdir|chmod|eol|binary):\s*(.+?)\s*===$`)

	// 旧版：明确报错
	rHeadLegacyFile = regexp.MustCompile(`^===\s*file:\s*(.+?)\s*===$`)

	// 结尾
	rEnd = regexp.MustCompile(`^===\s*end\s*===$`)
)

// ----- 入口 -----
func ParsePatch(patchFile string, eofMark string) (*Patch, error) {
	raw, err := os.ReadFile(patchFile)
	if err != nil {
		return nil, err
	}
	// 严格 EOF（最后一个非空行）
	last := lastMeaningfulLine(raw)
	if last != eofMark {
		return nil, fmt.Errorf("严格 EOF 校验失败：期望『%s』，实得『%s』", eofMark, last)
	}

	p := &Patch{}
	lines := splitLines(raw)

	// 读取头字段（直到遇到第一个 '=== '）
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.HasPrefix(line, "commitmsg:") && p.Commit == "" {
			p.Commit = strings.TrimSpace(line[len("commitmsg:"):])
			continue
		}
		if strings.HasPrefix(line, "author:") && p.Author == "" {
			p.Author = strings.TrimSpace(line[len("author:"):])
			continue
		}
		if strings.HasPrefix(line, "repo:") {
			// repo 由外层处理
			continue
		}
		if strings.HasPrefix(line, "===") {
			break
		}
	}

	// 块解析
	in := 0 // 0=无；2=file.op；3=（预留）block
	var curBody bytes.Buffer
	var curOp FileOp

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		if in == 0 {
			// 明确拒绝旧版
			if m := rHeadLegacyFile.FindStringSubmatch(line); len(m) > 0 {
				return nil, fmt.Errorf("已移除旧版头 '=== file: <path> ==='，请改用 '=== file.write: <path> ==='")
			}
			// 新版 file.<op>
			if m := rHeadFileOp.FindStringSubmatch(line); len(m) > 0 {
				in = 2
				curBody.Reset()
				op := strings.ToLower(m[1])
				head := strings.TrimSpace(m[2])
				curOp = parseOpHead(op, head)
				continue
			}
			continue
		}

		if rEnd.MatchString(line) {
			switch in {
			case 2: // file.<op>
				body := curBody.String()
				switch curOp.Op {
				case "write", "append", "prepend", "replace", "binary":
					curOp.Body = normalizeBody(body)
					// 映射进 Files 以兼容旧 apply（仅写类）
					if curOp.Op == "write" || curOp.Op == "append" || curOp.Op == "prepend" || curOp.Op == "replace" {
						p.Files = append(p.Files, FileChunk{
							Path:    curOp.Path,
							Content: curOp.Body,
							Mode:    curOp.Op,
						})
					}
				default:
					curOp.Body = body // 通常为空，保留
				}
				p.Ops = append(p.Ops, curOp)
			}
			in = 0
			curBody.Reset()
			continue
		}

		// 累积正文
		if in != 0 {
			curBody.WriteString(line)
			curBody.WriteByte('\n')
		}
	}

	return p, nil
}

// ----- 解析头参数 -----
func parseOpHead(op string, head string) FileOp {
	op = strings.ToLower(strings.TrimSpace(op))
	head = strings.TrimSpace(head)

	// 支持 head 内 KV（空格或 & 分隔）
	kv := parseKV(head)

	switch op {
	case "write", "append", "prepend":
		return FileOp{Op: op, Path: normPath(head), Mode: op}
	case "replace":
		// 允许： path?find=...&with=...&regex=1&flags=im
		path := head
		if i := strings.IndexAny(head, " ?"); i >= 0 {
			path = strings.TrimSpace(head[:i])
		}
		return FileOp{
			Op:          op,
			Path:        normPath(path),
			Find:        kv["find"],
			ReplaceWith: kv["with"],
			Regex:       asBool(kv["regex"]),
			Flags:       kv["flags"],
			Mode:        "replace",
		}
	case "delete":
		return FileOp{Op: op, Path: normPath(head)}
	case "move":
		from, to := splitArrow(head)
		return FileOp{Op: op, From: normPath(from), Path: normPath(to)}
	case "copy":
		from, to := splitArrow(head)
		return FileOp{Op: op, From: normPath(from), Path: normPath(to)}
	case "mkdir":
		return FileOp{Op: op, Dir: normPath(head)}
	case "chmod":
		mode := firstNonEmpty(kv["mode"], kv["chmod"])
		path := firstNonEmpty(kv["path"], head)
		return FileOp{Op: op, Path: normPath(path), Chmod: mode}
	case "eol":
		path := firstNonEmpty(kv["path"], head)
		style := strings.ToLower(firstNonEmpty(kv["style"], kv["eol"]))
		ensure := asBool(firstNonEmpty(kv["ensure_nl"], kv["ensurenl"], kv["nl"]))
		return FileOp{Op: op, Path: normPath(path), EOL: style, EnsureNL: ensure}
	case "binary":
		return FileOp{Op: op, Path: normPath(head)}
	default:
		// 未知操作：保守降级为 write（避免中断）
		return FileOp{Op: "write", Path: normPath(head), Mode: "write"}
	}
}

// ----- 工具函数 -----
func lastMeaningfulLine(raw []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(raw))
	last := ""
	for sc.Scan() {
		s := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(s) != "" {
			last = s
		}
	}
	return last
}

func splitLines(raw []byte) []string {
	return strings.Split(string(raw), "\n")
}

// 路径规范：*.md 或无扩展 => 文件名大写；其余 => 文件名小写；扩展小写；去首尾空白与 ./ 前缀
func normPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.ReplaceAll(p, "//", "/")
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	name, ext := base, ""
	if i := strings.LastIndex(base, "."); i >= 0 {
		name, ext = base[:i], base[i+1:]
	}
	extL := strings.ToLower(ext)
	if ext == "" || extL == "md" {
		name = strings.ToUpper(name)
	} else {
		name = strings.ToLower(name)
	}
	if extL != "" {
		base = name + "." + extL
	} else {
		base = name
	}
	if dir == "." {
		return base
	}
	return filepath.Join(dir, base)
}

func normalizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func splitArrow(head string) (string, string) {
	// "from => to" 或 "from=>to"
	arr := strings.Split(head, "=>")
	if len(arr) >= 2 {
		return strings.TrimSpace(arr[0]), strings.TrimSpace(strings.Join(arr[1:], "=>"))
	}
	return head, head
}

func parseKV(s string) map[string]string {
	out := map[string]string{}
	rest := ""
	if i := strings.Index(s, "?"); i >= 0 {
		rest = s[i+1:]
	} else if i := strings.Index(s, " "); i >= 0 {
		rest = s[i+1:]
	}
	if rest == "" {
		return out
	}
	parts := fieldsOrAmp(rest)
	for _, seg := range parts {
		if seg == "" {
			continue
		}
		k, v, ok := strings.Cut(seg, "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return out
}

func fieldsOrAmp(s string) []string {
	if strings.Contains(s, "&") {
		ps := strings.Split(s, "&")
		for i := range ps {
			ps[i] = strings.TrimSpace(ps[i])
		}
		return ps
	}
	return strings.Fields(s)
}

func asBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "1", "t", "true", "yes", "on":
		return true
	case "0", "f", "false", "no", "off", "":
		return false
	default:
		if n, err := strconv.Atoi(s); err == nil {
			return n != 0
		}
		return false
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
