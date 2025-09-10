// Package patch: block apply (anchors + 4 modes)
// XGIT:BEGIN BLOCK_HEADER
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// XGIT:END BLOCK_HEADER

// XGIT:BEGIN BLOCK_APPLY
// ApplyBlock: 区块应用（嵌套锚点 + replace/append/prepend/append_once），命中后自动 stage
func ApplyBlock(repo string, blk BlockChunk, logf func(string, ...any)) error {
	file := filepath.Join(repo, blk.Path)
	_ = os.MkdirAll(filepath.Dir(file), 0o755)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		_ = os.WriteFile(file, []byte(""), 0o644)
	}

	begin, end := BeginEndMarkers(blk.Path, blk.Anchor)
	data, _ := os.ReadFile(file)
	txt := strings.ReplaceAll(string(data), "\r", "")

	type pair struct{ s, e int }
	pairs := make([]pair, 0)
	var stack []int
	lines := strings.Split(txt, "\n")
	for i, l := range lines {
		if strings.Contains(l, begin) {
			stack = append(stack, i)
		}
		if strings.Contains(l, end) && len(stack) > 0 {
			s := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			pairs = append(pairs, pair{s: s, e: i})
		}
	}
	// insertion sort（文件通常不大）
	for i := 1; i < len(pairs); i++ {
		j := i
		for j > 0 && pairs[j-1].s > pairs[j].s {
			pairs[j-1], pairs[j] = pairs[j], pairs[j-1]
			j--
		}
	}

	body := strings.ReplaceAll(blk.Body, "\r", "")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	if blk.Index >= 1 && blk.Index <= len(pairs) {
		p := pairs[blk.Index-1]
		head := strings.Join(lines[:p.s+1], "\n")
		mid := strings.Join(lines[p.s+1:p.e], "\n")
		tail := strings.Join(lines[p.e:], "\n")

		switch blk.Mode {
		case "replace":
			mid = body
		case "append", "append_once":
			if blk.Mode == "append_once" && normalizedContains(mid, body) {
				_ = os.WriteFile(file, []byte(strings.Join([]string{head, mid, tail}, "\n")), 0o644)
				if logf != nil {
					logf("ℹ️ append_once：内容已存在，跳过（%s #%s @index=%d）", blk.Path, blk.Anchor, blk.Index)
				}
				Stage(repo, blk.Path, logf)
				return nil
			}
			if mid == "" {
				mid = body
			} else {
				mid = mid + "\n" + body
			}
		case "prepend":
			if mid == "" {
				mid = body
			} else {
				mid = body + "\n" + mid
			}
		default:
			mid = body
		}

		result := strings.Join([]string{head, mid, tail}, "\n")
		result = strings.ReplaceAll(result, "\n\n\n\n", "\n\n\n")
		_ = os.WriteFile(file, []byte(result), 0o644)
		if logf != nil {
			logf("🧩 命中锚区：%s #%s (mode=%s, @index=%d)", blk.Path, blk.Anchor, blk.Mode, blk.Index)
		}
		Stage(repo, blk.Path, logf)
		return nil
	}

	// 无锚点：尾部新建完整锚区
	var buf bytes.Buffer
	if len(lines) > 0 {
		buf.WriteString(strings.Join(lines, "\n"))
		if !strings.HasSuffix(buf.String(), "\n") {
			buf.WriteString("\n")
		}
	}
	buf.WriteString(begin + "\n")
	buf.WriteString(body)
	buf.WriteString(end + "\n")
	_ = os.WriteFile(file, buf.Bytes(), 0o644)
	if logf != nil {
		logf("✅ 新建锚区并写入：%s #%s (mode=%s, @index=%d)", blk.Path, blk.Anchor, blk.Mode, blk.Index)
	}
	Stage(repo, blk.Path, logf)
	return nil
}

func normalizedContains(haystack, needle string) bool {
	norm := func(s string) string {
		ss := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
		for i := range ss {
			ss[i] = strings.TrimRight(ss[i], " \t")
		}
		return strings.Join(ss, "\n")
	}
	return strings.Contains(norm(haystack), norm(needle))
}
// XGIT:END BLOCK_APPLY
