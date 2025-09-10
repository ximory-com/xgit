// XGIT:BEGIN PACKAGE
package main
// XGIT:END PACKAGE

// XGIT:BEGIN IMPORTS
import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)
// XGIT:END IMPORTS

// XGIT:BEGIN PARSER
// 解析得到的结构体（与其它文件解耦，避免重名冲突）
type Patch struct {
	Commit string
	Author string
	Files  []FileChunk
	Blocks []BlockChunk
}

type FileChunk struct {
	Path    string
	Content string
}

type BlockChunk struct {
	Path   string
	Anchor string
	Mode   string // replace/append/prepend/append_once
	Index  int
	Body   string
}

var (
	rFile  = regexp.MustCompile(`^=== file:\s*(.+?)\s*===$`)
	rBlock = regexp.MustCompile(`^=== block:\s*([^#\s]+)#([A-Za-z0-9_-]+)(?:@index=(\d+))?(?:\s+mode=(replace|append|prepend|append_once))?\s*===$`)
)

// ParsePatch 解析补丁文本（严格 EOF 已由 watcher 判定）
func ParsePatch(raw []byte, eof string) (*Patch, error) {
	// 最后一条非空行校验（双保险）
	last := ""
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) != "" {
			last = line
		}
	}
	if last != eof {
		return nil, fmt.Errorf("严格 EOF 校验失败：期望『%s』，实得『%s』", eof, last)
	}

	lines := strings.Split(strings.ReplaceAll(string(raw), "\r", ""), "\n")
	p := &Patch{}

	// 读取头
	for i := 0; i < len(lines); i++ {
		s := strings.TrimSpace(lines[i])
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "commitmsg:") && p.Commit == "" {
			p.Commit = strings.TrimSpace(strings.TrimPrefix(s, "commitmsg:"))
			continue
		}
		if strings.HasPrefix(s, "author:") && p.Author == "" {
			p.Author = strings.TrimSpace(strings.TrimPrefix(s, "author:"))
			continue
		}
		if strings.HasPrefix(s, "===") {
			break
		}
	}

	// 读取块
	in := 0 // 0 none; 1 file; 2 block
	curPath := ""
	var curBody strings.Builder
	curBlk := BlockChunk{Index: 1, Mode: "replace"}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if in == 0 {
			if m := rFile.FindStringSubmatch(line); len(m) > 0 {
				in = 1
				curPath = normPathLocal(m[1])
				curBody.Reset()
				continue
			}
			if m := rBlock.FindStringSubmatch(line); len(m) > 0 {
				in = 2
				curBlk = BlockChunk{
					Path:   normPathLocal(m[1]),
					Anchor: m[2],
					Index:  1,
					Mode:   "replace",
				}
				if m[3] != "" {
					fmt.Sscanf(m[3], "%d", &curBlk.Index)
				}
				if m[4] != "" {
					curBlk.Mode = m[4]
				}
				curBody.Reset()
				continue
			}
			continue
		}

		if line == "=== end ===" {
			if in == 1 {
				p.Files = append(p.Files, FileChunk{Path: curPath, Content: curBody.String()})
			} else {
				curBlk.Body = curBody.String()
				p.Blocks = append(p.Blocks, curBlk)
			}
			in = 0
			curPath = ""
			curBody.Reset()
			continue
		}

		if line == eof {
			break
		}

		curBody.WriteString(line)
		curBody.WriteByte('\n')
	}

	return p, nil
}

// 本地路径规范，不与其它 util 同名，避免重定义
func normPathLocal(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.ReplaceAll(p, "//", "/")
	dir, base := splitDirBase(p)
	name, ext := splitNameExt(base)
	extL := strings.ToLower(ext)
	if ext == "" || extL == "md" {
		name = strings.ToUpper(name)
	} else {
		name = strings.ToLower(name)
	}
	base2 := name
	if extL != "" {
		base2 = name + "." + extL
	}
	if dir == "" || dir == "." {
		return base2
	}
	if strings.HasSuffix(dir, "/") {
		return dir + base2
	}
	return dir + "/" + base2
}

func splitDirBase(p string) (string, string) {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ".", p
	}
	if i == 0 {
		return "/", p[1:]
	}
	return p[:i], p[i+1:]
}

func splitNameExt(b string) (string, string) {
	i := strings.LastIndex(b, ".")
	if i < 0 {
		return b, ""
	}
	return b[:i], b[i+1:]
}
// XGIT:END PARSER
