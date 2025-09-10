package main

// XGIT:BEGIN IMPORTS
// 说明：补丁解析（commitmsg/author + file/block），严格 EOF
import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)
// XGIT:END IMPORTS

// XGIT:BEGIN PARSER
// 说明：解析补丁文本为结构化对象
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

func ParsePatch(patchFile, eof string) (*Patch, error) {
	b, err := os.ReadFile(patchFile)
	if err != nil {
		return nil, err
	}
	// 严格 EOF（最后一个非空行）
	lastMeaningful := ""
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		s := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(s) != "" {
			lastMeaningful = s
		}
	}
	if lastMeaningful != eof {
		return nil, fmt.Errorf("严格 EOF 校验失败：期望『%s』，实得『%s』", eof, lastMeaningful)
	}

	p := &Patch{}
	lines := strings.Split(string(b), "\n")

	// 头字段
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.HasPrefix(line, "commitmsg:") && p.Commit == "" {
			p.Commit = strings.TrimSpace(strings.TrimPrefix(line, "commitmsg:"))
		} else if strings.HasPrefix(line, "author:") && p.Author == "" {
			p.Author = strings.TrimSpace(strings.TrimPrefix(line, "author:"))
		}
		if strings.HasPrefix(line, "=== ") {
			break
		}
	}

	// 块抓取
	in := 0 // 0 无；1 file；2 block
	curPath := ""
	curBody := &strings.Builder{}
	curBlk := BlockChunk{Index: 1, Mode: "replace"}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		if in == 0 {
			if m := rFile.FindStringSubmatch(line); len(m) > 0 {
				in = 1
				curPath = NormPath(m[1])
				curBody.Reset()
				continue
			}
			if m := rBlock.FindStringSubmatch(line); len(m) > 0 {
				in = 2
				curBlk = BlockChunk{
					Path:   NormPath(m[1]),
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

		if in != 0 && line == "=== end ===" {
			if in == 1 {
				p.Files = append(p.Files, FileChunk{Path: curPath, Content: curBody.String()})
			} else {
				curBlk.Body = curBody.String()
				p.Blocks = append(p.Blocks, curBlk)
			}
			in, curPath = 0, ""
			curBody.Reset()
			continue
		}
		if line == eof {
			break
		}
		if in != 0 {
			curBody.WriteString(line)
			curBody.WriteByte('\n')
		}
	}
	return p, nil
}
// XGIT:END PARSER
