package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// =========================================
// 日志多路输出（控制台 + patch.log，执行一次补丁截断 patch.log）
// XGIT:BEGIN LOGGING
type dualLogger struct {
	Console io.Writer
	File    *os.File
	w       io.Writer
}

func newDualLogger(patchDir string) (*dualLogger, error) {
	logPath := filepath.Join(patchDir, "patch.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	d := &dualLogger{Console: os.Stdout, File: f}
	d.w = io.MultiWriter(d.Console, d.File)
	return d, nil
}
func (d *dualLogger) Close() { if d != nil && d.File != nil { _ = d.File.Close() } }
func (d *dualLogger) log(format string, a ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(d.w, "%s %s\n", ts, fmt.Sprintf(format, a...))
}
// XGIT:END LOGGING

// =========================================
// 轻量 shell 调用
// XGIT:BEGIN SHELL
func shell(parts ...string) (string, string, error) {
	if len(parts) == 0 {
		return "", "", errors.New("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	var out, er bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &er
	err := cmd.Run()
	return strings.TrimRight(out.String(), "\n"), strings.TrimRight(er.String(), "\n"), err
}
// XGIT:END SHELL

// =========================================
// 解析 .repos （name path；允许 'default = name'）
// XGIT:BEGIN REPOS
func loadRepos(patchDir string) (map[string]string, string) {
	m := map[string]string{}
	def := ""
	f, err := os.Open(filepath.Join(patchDir, ".repos"))
	if err != nil {
		return m, def
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "=") {
			// default = xgit
			k, v, _ := strings.Cut(line, "=")
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if strings.EqualFold(k, "default") {
				def = v
			} else if v != "" {
				m[k] = v
			}
			continue
		}
		// name /abs/path
		sp := strings.Fields(line)
		if len(sp) >= 2 {
			name := sp[0]
			path := strings.Join(sp[1:], " ")
			m[name] = path
		}
	}
	return m, def
}
// XGIT:END REPOS

// =========================================
// 解析补丁（commitmsg/author + file/block）
// XGIT:BEGIN PARSER
type patch struct {
	Commit string
	Author string
	Files  []fileChunk
	Blocks []blockChunk
}
type fileChunk struct {
	Path    string
	Content string
}
type blockChunk struct {
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

func parsePatch(patchFile, eof string) (*patch, error) {
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

	// 提取
	p := &patch{}
	// 头字段
	lines := strings.Split(string(b), "\n")
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

	// 块
	in := 0 // 0 无；1 file；2 block
	curPath := ""
	curBody := &strings.Builder{}
	curBlk := blockChunk{Index: 1, Mode: "replace"}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		if in == 0 {
			if m := rFile.FindStringSubmatch(line); len(m) > 0 {
				in = 1
				curPath = normPath(m[1])
				curBody.Reset()
				continue
			}
			if m := rBlock.FindStringSubmatch(line); len(m) > 0 {
				in = 2
				curBlk = blockChunk{
					Path:   normPath(m[1]),
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
				p.Files = append(p.Files, fileChunk{Path: curPath, Content: curBody.String()})
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
		if in != 0 {
			curBody.WriteString(line)
			curBody.WriteByte('\n')
		}
	}
	return p, nil
}
// XGIT:END PARSER

// =========================================
// 路径规范：*.md 或无扩展 => 文件名大写；其余 => 文件名小写；扩展一律小写；去前后空白
// XGIT:BEGIN NORM_PATH
func lower(s string) string { return strings.ToLower(s) }
func upper(s string) string { return strings.ToUpper(s) }
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
	extL := lower(ext)
	if ext == "" || extL == "md" {
		name = upper(name)
	} else {
		name = lower(name)
	}
	if extL != "" {
		base = fmt.Sprintf("%s.%s", name, extL)
	} else {
		base = name
	}
	if dir == "." {
		return base
	}
	return filepath.Join(dir, base)
}
// XGIT:END NORM_PATH

// =========================================
// Go/HTML/CSS/文本 锚点注释风格
// XGIT:BEGIN ANCHOR_STYLE
func beginEndMarkers(path, name string) (string, string) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm", ".jsx", ".tsx":
		return fmt.Sprintf("<!-- XGIT:BEGIN %s -->", name), fmt.Sprintf("<!-- XGIT:END %s -->", name)
	case ".css", ".scss":
		return fmt.Sprintf("/* XGIT:BEGIN %s */", name), fmt.Sprintf("/* XGIT:END %s */", name)
	case ".go":
		return fmt.Sprintf("// XGIT:BEGIN %s", name), fmt.Sprintf("// XGIT:END %s", name)
	default:
		return fmt.Sprintf("# XGIT:BEGIN %s", name), fmt.Sprintf("# XGIT:END %s", name)
	}
}
// XGIT:END ANCHOR_STYLE

// =========================================
// 写文件 + 统一 stage（关键修改 #1）
// XGIT:BEGIN WRITE_AND_STAGE
func writeFile(repo string, rel string, content string, logf func(string, ...any)) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return err
	}
	// 统一 LF；保证末尾换行
	content = strings.ReplaceAll(content, "\r", "")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return err
	}
	logf("✅ 写入文件：%s", rel)
	stage(repo, rel, logf) // <—— 关键：写入后立即加入暂存
	return nil
}
// XGIT:END WRITE_AND_STAGE

// =========================================
// stage 函数（关键新增 #2）
// XGIT:BEGIN STAGE_FUNC
func stage(repo, rel string, logf func(string, ...any)) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return
	}
	if _, _, err := shell("git", "-C", repo, "add", "--", rel); err != nil {
		logf("⚠️ 自动加入暂存失败：%s", rel)
	} else {
		logf("🧮 已加入暂存：%s", rel)
	}
}
// XGIT:END STAGE_FUNC

// =========================================
// 区块：查找/创建锚点 + 四种模式 + append_once 去重
// 命中后自动 stage（关键修改 #3）
// XGIT:BEGIN BLOCK_APPLY
func applyBlock(repo string, blk blockChunk, logf func(string, ...any)) error {
	file := filepath.Join(repo, blk.Path)
	_ = os.MkdirAll(filepath.Dir(file), 0755)
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		_ = os.WriteFile(file, []byte(""), 0644)
	}

	begin, end := beginEndMarkers(blk.Path, blk.Anchor)
	data, _ := os.ReadFile(file)
	txt := strings.ReplaceAll(string(data), "\r", "")

	// 找所有成对锚点（允许嵌套）
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
	// 排序（按开始行）
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

	// 有目标锚点
	if blk.Index >= 1 && blk.Index <= len(pairs) {
		p := pairs[blk.Index-1]
		head := strings.Join(lines[:p.s+1], "\n")
		mid := strings.Join(lines[p.s+1:p.e], "\n")
		tail := strings.Join(lines[p.e:], "\n")

		switch blk.Mode {
		case "replace":
			mid = body
		case "append", "append_once":
			// 去重判定（按去尾空白规范化）
			if blk.Mode == "append_once" {
				if normalizedContains(mid, body) {
					_ = os.WriteFile(file, []byte(strings.Join([]string{head, mid, tail}, "\n")), 0644)
					logf("ℹ️ append_once：内容已存在，跳过（%s #%s @index=%d）", blk.Path, blk.Anchor, blk.Index)
					stage(repo, blk.Path, logf)
					return nil
				}
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
		_ = os.WriteFile(file, []byte(result), 0644)
		logf("🧩 命中锚区：%s #%s (mode=%s, @index=%d)", blk.Path, blk.Anchor, blk.Mode, blk.Index)
		stage(repo, blk.Path, logf)
		return nil
	}

	// 无锚点：尾部新建完整锚区（带 begin/body/end）
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
	_ = os.WriteFile(file, buf.Bytes(), 0644)
	logf("✅ 新建锚区并写入：%s #%s (mode=%s, @index=%d)", blk.Path, blk.Anchor, blk.Mode, blk.Index)
	stage(repo, blk.Path, logf)
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

// =========================================
// watcher：稳定判断 + EOF 去抖（关键修改 #4）
// XGIT:BEGIN WATCH
type watcher struct {
	PatchFile string
	EOFMark   string
	logger    *dualLogger
	eofWarned bool
}

func (w *watcher) stableAndEOF() (ok bool, size int, hash8 string) {
	fi, err := os.Stat(w.PatchFile)
	if err != nil || fi.Size() <= 0 {
		return false, 0, ""
	}
	size1 := fi.Size()
	time.Sleep(300 * time.Millisecond)
	fi2, err2 := os.Stat(w.PatchFile)
	if err2 != nil || fi2.Size() != size1 {
		return false, 0, ""
	}
	// EOF 校验
	f, _ := os.Open(w.PatchFile)
	defer f.Close()
	line := lastLine(f)
	if line != w.EOFMark {
		if !w.eofWarned {
			w.logger.log("⏳ 等待严格 EOF 标记“%s”", w.EOFMark)
			w.eofWarned = true
		}
		return false, 0, ""
	}
	w.eofWarned = false
	// md5
	all, _ := os.ReadFile(w.PatchFile)
	h := md5.Sum(all)
	return true, int(size1), hex.EncodeToString(h[:])[:8]
}

func lastLine(r io.Reader) string {
	sc := bufio.NewScanner(r)
	last := ""
	for sc.Scan() {
		t := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(t) != "" {
			last = t
		}
	}
	return last
}
// XGIT:END WATCH

// =========================================
// 主流程
// XGIT:BEGIN MAIN
func main() {
	// 参数 & 路径
	baseDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	patchFile := filepath.Join(baseDir, "文本.txt") // 与脚本约定一致
	patchDir := baseDir
	eof := "=== PATCH EOF ==="

	// logger
	logger, err := newDualLogger(patchDir)
	if err != nil {
		fmt.Println("logger init 失败:", err)
		return
	}
	defer logger.Close()

	logger.log("▶ xgit_patchd 启动，监听：%s", patchFile)

	// 加载 repos
	repos, def := loadRepos(patchDir)

	// 轮询 watcher
	w := &watcher{PatchFile: patchFile, EOFMark: eof, logger: logger}
	var lastHash string

	for {
		ok, size, h8 := w.stableAndEOF()
		if ok && h8 != "" && h8 != lastHash {
			logger.log("📦 补丁稳定（size=%d md5=%s）→ 准备执行", size, h8)
			// 解析补丁
			pt, err := parsePatch(patchFile, eof)
			if err != nil {
				logger.log("❌ 解析失败：%v", err)
				lastHash = h8 // 防止同一内容反复解析
				time.Sleep(700 * time.Millisecond)
				continue
			}
			// 解析 repo 选择
			targetName := def
			if name := headerRepoName(patchFile); name != "" {
				targetName = name
			}
			repoPath := repos[targetName]
			if repoPath == "" && strings.HasPrefix(targetName, "/") {
				// 允许直接绝对路径
				repoPath = targetName
			}
			if repoPath == "" {
				logger.log("❌ 无法解析仓库（.repos 或 repo: 头字段）。")
				lastHash = h8
				time.Sleep(700 * time.Millisecond)
				continue
			}

			// 执行一次补丁
			applyOnce(logger, repoPath, pt)
			lastHash = h8
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func headerRepoName(patchFile string) string {
	f, err := os.Open(patchFile)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "repo:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "repo:"))
		}
		if strings.HasPrefix(line, "===") {
			break
		}
	}
	return ""
}

func applyOnce(logger *dualLogger, repo string, p *patch) {
	logger.log("▶ 开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))
	logger.log("ℹ️ 仓库：%s", repo)

	// 清理（auto）
	logger.log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = shell("git", "-C", repo, "clean", "-fd")

	// 写文件
	for _, f := range p.Files {
		if err := writeFile(repo, f.Path, f.Content, logger.log); err != nil {
			logger.log("❌ 写入失败：%s (%v)", f.Path, err)
			return
		}
	}

	// 区块
	for _, b := range p.Blocks {
		if err := applyBlock(repo, b, logger.log); err != nil {
			logger.log("❌ 区块失败：%s #%s (%v)", b.Path, b.Anchor, err)
			return
		}
	}

	// 无改动直接返回（先检查缓存区是否有文件名）
	names, _, _ := shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.log("ℹ️ 无改动需要提交。")
		logger.log("✅ 本次补丁完成")
		return
	}

	// 提交 & 推送
	commit := p.Commit
	if strings.TrimSpace(commit) == "" {
		commit = "chore: apply patch"
	}
	author := strings.TrimSpace(p.Author)
	if author == "" {
		author = "XGit Bot <bot@xgit.local>"
	}
	logger.log("ℹ️ 提交说明：%s", commit)
	logger.log("ℹ️ 提交作者：%s", author)
	_, _, _ = shell("git", "-C", repo, "commit", "--author", author, "-m", commit)
	logger.log("✅ 已提交：%s", commit)

	logger.log("🚀 正在推送（origin HEAD）…")
	if _, er, err := shell("git", "-C", repo, "push", "origin", "HEAD"); err != nil {
		logger.log("❌ 推送失败：%s", er)
	} else {
		logger.log("🚀 推送完成")
	}
	logger.log("✅ 本次补丁完成")
}
// XGIT:END MAIN
