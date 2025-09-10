package main

// XGIT:BEGIN IMPORTS
// 说明：区块操作（查找/创建锚点 + replace/append/prepend/append_once）
import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"bytes"
)
// XGIT:END IMPORTS

// XGIT:BEGIN WRITE_AND_STAGE
// 说明：写文件并立即加入暂存
func WriteFileAndStage(repo string, rel string, content string, logf func(string, ...any)) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return err
	}
	content = strings.ReplaceAll(content, "\r", "")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return err
	}
	logf("✅ 写入文件：%s", rel)
	Stage(repo, rel, logf)
	return nil
}
// 这里可以扩展写入策略（如权限、模板化等）

// XGIT:END WRITE_AND_STAGE

// XGIT:BEGIN STAGE_FUNC
// 说明：git add -- <file>
func Stage(repo, rel string, logf func(string, ...any)) {
	rel = strings.TrimSpace(rel)
	if rel == "" { return }
	if _, _, err := Shell("git", "-C", repo, "add", "--", rel); err != nil {
		logf("⚠️ 自动加入暂存失败：%s", rel)
	} else {
		logf("🧮 已加入暂存：%s", rel)
	}
}
// XGIT:END STAGE_FUNC

// XGIT:BEGIN BLOCK_APPLY
// 说明：在文件中定位/创建锚点并按模式写入；命中后自动 stage
func ApplyBlock(repo string, blk BlockChunk, logf func(string, ...any)) error {
	file := filepath.Join(repo, blk.Path)
	_ = os.MkdirAll(filepath.Dir(file), 0755)
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		_ = os.WriteFile(file, []byte(""), 0644)
	}

	begin, end := BeginEndMarkers(blk.Path, blk.Anchor)
	data, _ := os.ReadFile(file)
	txt := strings.ReplaceAll(string(data), "\r", "")

	type pair struct{ s, e int }
	pairs := make([]pair, 0)
	var stack []int
	lines := strings.Split(txt, "\n")
	for i, l := range lines {
		if strings.Contains(l, begin) { stack = append(stack, i) }
		if strings.Contains(l, end) && len(stack) > 0 {
			s := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			pairs = append(pairs, pair{s: s, e: i})
		}
	}
	// 简单升序
	for i := 1; i < len(pairs); i++ {
		j := i
		for j > 0 && pairs[j-1].s > pairs[j].s {
			pairs[j-1], pairs[j] = pairs[j], pairs[j-1]
			j--
		}
	}

	body := strings.ReplaceAll(blk.Body, "\r", "")
	if !strings.HasSuffix(body, "\n") { body += "\n" }

	// 命中既有锚点
	if blk.Index >= 1 && blk.Index <= len(pairs) {
		p := pairs[blk.Index-1]
		head := strings.Join(lines[:p.s+1], "\n")
		mid  := strings.Join(lines[p.s+1:p.e], "\n")
		tail := strings.Join(lines[p.e:], "\n")

		switch blk.Mode {
		case "replace":
			mid = body
		case "append", "append_once":
			if blk.Mode == "append_once" && normalizedContains(mid, body) {
				_ = os.WriteFile(file, []byte(strings.Join([]string{head, mid, tail}, "\n")), 0644)
				logf("ℹ️ append_once：内容已存在，跳过（%s #%s @index=%d）", blk.Path, blk.Anchor, blk.Index)
				Stage(repo, blk.Path, logf)
				return nil
			}
			if mid == "" { mid = body } else { mid = mid + "\n" + body }
		case "prepend":
			if mid == "" { mid = body } else { mid = body + "\n" + mid }
		default:
			mid = body
		}

		var out bytes.Buffer
		out.WriteString(head); out.WriteString("\n")
		out.WriteString(mid)
		if !strings.HasSuffix(mid, "\n") { out.WriteString("\n") }
		out.WriteString(tail)
		_ = os.WriteFile(file, out.Bytes(), 0644)
		logf("🧩 命中锚区：%s #%s (mode=%s, @index=%d)", blk.Path, blk.Anchor, blk.Mode, blk.Index)
		Stage(repo, blk.Path, logf)
		return nil
	}

	// 无锚点：尾部直接建立完整锚区（begin/body/end）
	var buf bytes.Buffer
	if len(lines) > 0 {
		buf.WriteString(strings.Join(lines, "\n"))
		if !strings.HasSuffix(buf.String(), "\n") { buf.WriteString("\n") }
	}
	buf.WriteString(begin + "\n")
	buf.WriteString(body)
	buf.WriteString(end + "\n")
	_ = os.WriteFile(file, buf.Bytes(), 0644)
	logf("✅ 新建锚区并写入：%s #%s (mode=%s, @index=%d)", blk.Path, blk.Anchor, blk.Index)
	Stage(repo, blk.Path, logf)
	return nil
}

func normalizedContains(haystack, needle string) bool {
	norm := func(s string) string {
		ss := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
		for i := range ss { ss[i] = strings.TrimRight(ss[i], " \t") }
		return strings.Join(ss, "\n")
	}
	return strings.Contains(norm(haystack), norm(needle))
}
// XGIT:END BLOCK_APPLY
