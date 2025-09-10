// xgit_patchd.go
// 功能：监听补丁文件 -> 解析 -> 以事务方式对目标仓库执行 file/mv/delete/block -> 提交推送
// 依赖：标准库，无第三方；macOS/Linux 通用（需安装 git）
// 版本：v0.9.0 (single-file anchor edition)

package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	PatchFile   string
	ReposFile   string
	LogDir      string
	EOFMark     string
	IntervalMS  int
	StableTries int
	DebounceMS  int
	LockPath    string
	PatchLog    string
	WatcherLog  string
	CleanMode   string // auto|strict|ignore
	Push        bool
}

type Patch struct {
	CommitMsg  string
	Author     string
	RepoAlias  string
	Files      []FileWrite
	Deletes    []string
	Moves      []Move
	Blocks     []Block
	// Diff: 先留接口，后续可加
}

type FileWrite struct {
	Path    string
	Content []byte
}

type Move struct {
	From string
	To   string
}

type Block struct {
	Path   string
	Anchor string
	Mode   string // replace|append|prepend|append_once
	Index  int    // 1-based
	Body   []byte
}

// ------------------------------
// XGIT:BEGIN LOGGER
// ------------------------------
type MultiLogger struct {
	w io.Writer // 控制台 + 文件
}

func NewMultiLogger(patchLogPath string) (*MultiLogger, func(), error) {
	// 每次覆盖 patch.log
	f, err := os.Create(patchLogPath)
	if err != nil {
		return nil, nil, err
	}
	w := io.MultiWriter(os.Stdout, f)
	cleanup := func() { _ = f.Close() }
	return &MultiLogger{w: w}, cleanup, nil
}

func (l *MultiLogger) Infof(format string, args ...any)  { fmt.Fprintf(l.w, "ℹ️ "+format+"\n", args...) }
func (l *MultiLogger) Okf(format string, args ...any)    { fmt.Fprintf(l.w, "✅ "+format+"\n", args...) }
func (l *MultiLogger) Warnf(format string, args ...any)  { fmt.Fprintf(l.w, "⚠️ "+format+"\n", args...) }
func (l *MultiLogger) Errf(format string, args ...any)   { fmt.Fprintf(l.w, "❌ "+format+"\n", args...) }
func (l *MultiLogger) Pushf(format string, args ...any)  { fmt.Fprintf(l.w, "🚀 "+format+"\n", args...) }
func (l *MultiLogger) Matchf(format string, args ...any) { fmt.Fprintf(l.w, "🧩 "+format+"\n", args...) }
func (l *MultiLogger) Rollf(format string, args ...any)  { fmt.Fprintf(l.w, "↩️ "+format+"\n", args...) }
func (l *MultiLogger) Beginf(format string, args ...any) { fmt.Fprintf(l.w, "▶ "+format+"\n", args...) }
func (l *MultiLogger) Waitf(format string, args ...any)  { fmt.Fprintf(l.w, "⏳ "+format+"\n", args...) }
func (l *MultiLogger) Deletef(format string, args ...any){ fmt.Fprintf(l.w, "🗑️ "+format+"\n", args...) }
func (l *MultiLogger) Renamef(format string, args ...any){ fmt.Fprintf(l.w, "🔁 "+format+"\n", args...) }
// ------------------------------
// XGIT:END LOGGER
// ------------------------------

// ------------------------------
// XGIT:BEGIN FLAGS_AND_DEFAULTS
// ------------------------------
func defaultConfig() Config {
	return Config{
		PatchFile:   "./文本.txt",
		ReposFile:   "./.repos",
		LogDir:      ".",
		EOFMark:     "=== PATCH EOF ===",
		IntervalMS:  500,
		StableTries: 6,
		DebounceMS:  600,
		CleanMode:   "auto",
		Push:        true,
	}
}

func parseFlags(cfg *Config) (cmd string) {
	flag.StringVar(&cfg.PatchFile, "patch", cfg.PatchFile, "补丁文件路径")
	flag.StringVar(&cfg.ReposFile, "repos", cfg.ReposFile, ".repos 映射文件路径")
	flag.StringVar(&cfg.LogDir, "logdir", cfg.LogDir, "日志与锁目录")
	flag.StringVar(&cfg.EOFMark, "eof", cfg.EOFMark, "严格 EOF 标记")
	flag.IntVar(&cfg.IntervalMS, "interval", cfg.IntervalMS, "轮询间隔(毫秒)")
	flag.IntVar(&cfg.StableTries, "stable", cfg.StableTries, "稳定判定次数")
	flag.IntVar(&cfg.DebounceMS, "debounce", cfg.DebounceMS, "去抖等待(毫秒)")
	flag.StringVar(&cfg.CleanMode, "clean", cfg.CleanMode, "clean 策略: auto|strict|ignore")
	flag.BoolVar(&cfg.Push, "push", cfg.Push, "是否推送到远程")

	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Println("用法: xgit_patchd [start|stop|status] [flags...]")
		os.Exit(2)
	}
	cmd = flag.Arg(0)
	cfg.LockPath = filepath.Join(cfg.LogDir, ".xgit_patchd.lock")
	cfg.PatchLog = filepath.Join(cfg.LogDir, "patch.log")
	cfg.WatcherLog = filepath.Join(cfg.LogDir, "watch.log") // 目前仅预留
	return
}
// ------------------------------
// XGIT:END FLAGS_AND_DEFAULTS
// ------------------------------

// ------------------------------
// XGIT:BEGIN UTILS
// ------------------------------
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func fileSize(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func lastLine(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	i := bytes.LastIndexByte(b, '\n')
	var line []byte
	if i >= 0 && i+1 < len(b) {
		line = b[i+1:]
	} else {
		line = b
	}
	return strings.TrimRight(strings.ReplaceAll(string(line), "\r", ""), "\n"), nil
}

func md5sumFile(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	_, _ = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func msSleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func run(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func trimSpaceCR(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "\r")
	return s
}

func toAbs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	q, _ := filepath.Abs(p)
	return q
}

func normalizeLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte{'\r', '\n'}, []byte{'\n'})
	b = bytes.ReplaceAll(b, []byte{'\r'}, []byte{'\n'})
	if len(b) == 0 || b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return b
}
// ------------------------------
// XGIT:END UTILS
// ------------------------------

// ------------------------------
// XGIT:BEGIN REPOS_MAPPING
// ------------------------------
func parseReposMap(path string) (map[string]string, string, error) {
	// 返回：alias -> absPath；defaultAlias
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	m := map[string]string{}
	def := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexAny(line, "#;"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 形态1: "default = alias"
		if strings.Contains(line, "default") && strings.Contains(line, "=") {
			kv := strings.SplitN(line, "=", 2)
			if len(kv) == 2 && strings.TrimSpace(kv[0]) == "default" {
				def = strings.TrimSpace(kv[1])
				continue
			}
		}
		// 形态2: "alias /abs/path"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			alias := parts[0]
			path := strings.Join(parts[1:], " ")
			path = strings.TrimSpace(path)
			path = strings.Trim(path, `"`)
			path = toAbs(path)
			m[alias] = path
			continue
		}
		// 形态3: "alias = /abs/path"
		if strings.Contains(line, "=") {
			kv := strings.SplitN(line, "=", 2)
			alias := strings.TrimSpace(kv[0])
			path := strings.TrimSpace(kv[1])
			path = strings.Trim(path, `"`)
			path = toAbs(path)
			m[alias] = path
			continue
		}
	}
	return m, def, sc.Err()
}
// ------------------------------
// XGIT:END REPOS_MAPPING
// ------------------------------

// ------------------------------
// XGIT:BEGIN PATCH_PARSER
// ------------------------------
func parsePatch(patchPath, eofMark string) (*Patch, error) {
	b, err := os.ReadFile(patchPath)
	if err != nil {
		return nil, err
	}
	// 严格 EOF：最后一个非空行必须是 eofMark
	lastMeaningful := ""
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		s := strings.TrimSpace(strings.TrimSuffix(sc.Text(), "\r"))
		if s != "" {
			lastMeaningful = s
		}
	}
	if lastMeaningful != eofMark {
		return nil, fmt.Errorf("严格 EOF 校验失败：期望『%s』，实得『%s』", eofMark, lastMeaningful)
	}

	p := &Patch{}
	lines := bufio.NewScanner(bytes.NewReader(b))
	type section int
	const (
		none section = iota
		fileSec
		blockSec
	)
	var cur section
	var curPath, curAnchor, curMode string
	curIndex := 1
	var body bytes.Buffer

	headerDone := false

	for lines.Scan() {
		raw := strings.TrimSuffix(lines.Text(), "\r")
		line := raw

		// EOF -> 停止
		if strings.TrimSpace(line) == eofMark {
			break
		}

		if !headerDone {
			if strings.HasPrefix(line, "commitmsg:") {
				p.CommitMsg = trimSpaceCR(strings.TrimPrefix(line, "commitmsg:"))
				continue
			}
			if strings.HasPrefix(line, "author:") {
				p.Author = trimSpaceCR(strings.TrimPrefix(line, "author:"))
				continue
			}
			if strings.HasPrefix(line, "repo:") {
				p.RepoAlias = trimSpaceCR(strings.TrimPrefix(line, "repo:"))
				continue
			}
		}

		// 进入某块后，视为 header 结束
		if strings.HasPrefix(line, "===") {
			headerDone = true
		}

		// file
		if cur == none && strings.HasPrefix(line, "=== file:") && strings.HasSuffix(line, "===") {
			curPath = strings.TrimSpace(line[len("=== file:") : len(line)-len("===")])
			curPath = strings.TrimSpace(curPath)
			body.Reset()
			cur = fileSec
			continue
		}
		// delete
		if cur == none && strings.HasPrefix(line, "=== delete:") && strings.HasSuffix(line, "===") {
			path := strings.TrimSpace(line[len("=== delete:") : len(line)-len("===")])
			p.Deletes = append(p.Deletes, path)
			continue
		}
		// mv
		if cur == none && strings.HasPrefix(line, "=== mv:") && strings.HasSuffix(line, "===") {
			m := strings.TrimSpace(line[len("=== mv:") : len(line)-len("===")])
			// "old => new"
			parts := strings.Split(m, "=>")
			if len(parts) == 2 {
				from := strings.TrimSpace(parts[0])
				to := strings.TrimSpace(parts[1])
				p.Moves = append(p.Moves, Move{From: from, To: to})
			}
			continue
		}
		// block
		if cur == none && strings.HasPrefix(line, "=== block:") && strings.HasSuffix(line, "===") {
			spec := strings.TrimSpace(line[len("=== block:") : len(line)-len("===")])
			// e.g. ".gitignore#XGIT_IGNORE mode=append_once @index=2"
			// path#anchor [mode=xxx] [@index=N]
			curPath, curAnchor, curMode, curIndex = parseBlockSpec(spec)
			body.Reset()
			cur = blockSec
			continue
		}
		// end
		if strings.TrimSpace(line) == "=== end ===" {
			switch cur {
			case fileSec:
				p.Files = append(p.Files, FileWrite{Path: curPath, Content: normalizeLF(body.Bytes())})
			case blockSec:
				p.Blocks = append(p.Blocks, Block{
					Path: curPath, Anchor: curAnchor, Mode: curMode, Index: curIndex, Body: normalizeLF(body.Bytes()),
				})
			}
			cur = none
			body.Reset()
			continue
		}

		// 收集正文
		if cur == fileSec || cur == blockSec {
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}

	return p, nil
}

func parseBlockSpec(spec string) (path, anchor, mode string, index int) {
	index = 1
	mode = "replace"
	// path#anchor ...
	parts := strings.Fields(spec)
	if len(parts) == 0 {
		return
	}
	pa := parts[0]
	if i := strings.Index(pa, "#"); i >= 0 {
		path = strings.TrimSpace(pa[:i])
		anchor = strings.TrimSpace(pa[i+1:])
	} else {
		path = strings.TrimSpace(pa)
	}
	// parse rest
	for _, t := range parts[1:] {
		if strings.HasPrefix(t, "mode=") {
			mode = strings.TrimSpace(strings.TrimPrefix(t, "mode="))
		} else if strings.HasPrefix(t, "@index=") {
			n := strings.TrimPrefix(t, "@index=")
			if v, err := strconv.Atoi(n); err == nil && v > 0 {
				index = v
			}
		}
	}
	return
}
// ------------------------------
// XGIT:END PATCH_PARSER
// ------------------------------

// ------------------------------
// XGIT:BEGIN GIT_TX
// ------------------------------
type Tx struct {
	repo   string
	start  string
	logger *MultiLogger
}

func NewTx(repo string, logger *MultiLogger) (*Tx, error) {
	out, _ := run(repo, "git", "rev-parse", "--verify", "HEAD")
	start := strings.TrimSpace(out)
	return &Tx{repo: repo, start: start, logger: logger}, nil
}

func (t *Tx) Clean(mode string) error {
	switch mode {
	case "auto":
		t.logger.Infof("自动清理工作区：reset --hard / clean -fd")
		if _, err := run(t.repo, "git", "reset", "--hard"); err != nil {
			return err
		}
		if _, err := run(t.repo, "git", "clean", "-fd"); err != nil {
			return err
		}
	case "strict":
		_, err1 := run(t.repo, "git", "diff", "--quiet")
		_, err2 := run(t.repo, "git", "diff", "--cached", "--quiet")
		if err1 != nil || err2 != nil {
			return errors.New("工作区不干净，已中止（可设置 -clean auto）")
		}
	case "ignore":
		// pass
	default:
		return fmt.Errorf("未知 clean 模式：%s", mode)
	}
	return nil
}

func (t *Tx) Rollback() {
	if t.start == "" {
		return
	}
	_, _ = run(t.repo, "git", "reset", "--hard", t.start)
	_, _ = run(t.repo, "git", "clean", "-fd")
	t.logger.Rollf("已回滚至 %s", t.start)
}
// ------------------------------
// XGIT:END GIT_TX
// ------------------------------

// ------------------------------
// XGIT:BEGIN BLOCK_ENGINE
// ------------------------------

// Block 查找支持嵌套（BEGIN/END 成对，栈式匹配），大小写不敏感。
// BEGIN/END 前后的注释符不强制（尽量宽松：只要同一行出现“XGIT:BEGIN name”/“XGIT:END name”）
// 这样就能同时兼容 HTML/JS/CSS/YAML/INI 等文件。
var (
	reBegin = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)\bXGIT:\s*BEGIN\s+` + regexp.QuoteMeta(name) + `\b`)
	}
	reEnd = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)\bXGIT:\s*END\s+` + regexp.QuoteMeta(name) + `\b`)
	}
)

type blockPair struct{ Start, End int }

func findBlockPairs(lines []string, name string) []blockPair {
	rb := reBegin(name)
	re := reEnd(name)
	stack := []int{}
	pairs := []blockPair{}
	for i, ln := range lines {
		if rb.FindStringIndex(ln) != nil {
			stack = append(stack, i)
		}
		if re.FindStringIndex(ln) != nil && len(stack) > 0 {
			s := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			pairs = append(pairs, blockPair{Start: s, End: i})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Start < pairs[j].Start })
	return pairs
}

func applyBlock(repo string, logger *MultiLogger, b Block) error {
	abs := filepath.Join(repo, b.Path)
	_ = os.MkdirAll(filepath.Dir(abs), 0o755)
	_ = touch(abs)

	content, _ := os.ReadFile(abs)
	base := string(bytes.ReplaceAll(content, []byte{'\r'}, []byte{}))
	lines := strings.Split(base, "\n")

	pairs := findBlockPairs(lines, b.Anchor)
	if len(pairs) == 0 {
		// 自动引导：直接末尾追加一个完整锚区（begin+body+end）
		body := string(normalizeLF(b.Body))
		beginLine := fmt.Sprintf("# XGIT:BEGIN %s", b.Anchor)
		endLine := fmt.Sprintf("# XGIT:END %s", b.Anchor)
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, beginLine)
		if body != "" {
			bodyLines := strings.Split(strings.TrimRight(body, "\n"), "\n")
			lines = append(lines, bodyLines...)
		}
		lines = append(lines, endLine)
		logger.Infof("自动引导空锚点：%s #%s（@index=%d）", b.Path, b.Anchor, b.Index)
		logger.Okf("新建锚区并写入：%s #%s (mode=%s, @index=%d)", b.Path, b.Anchor, b.Mode, b.Index)
		return os.WriteFile(abs, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}

	// index 越界
	if b.Index <= 0 || b.Index > len(pairs) {
		return fmt.Errorf("未找到锚区或 index 越界：%s #%s（@index=%d）", b.Path, b.Anchor, b.Index)
	}

	// 命中区块
	p := pairs[b.Index-1]
	logger.Matchf("命中锚区：%s #%s", b.Path, b.Anchor)

	// 取出三段：头、体、尾
	head := strings.Join(lines[:p.Start+1], "\n")
	body := strings.Join(lines[p.Start+1:p.End], "\n")
	tail := strings.Join(lines[p.End:], "\n")

	// 归一化新正文
	newBody := strings.TrimRight(string(normalizeLF(b.Body)), "\n")

	switch b.Mode {
	case "replace":
		body = newBody
	case "append":
		if body == "" {
			body = newBody
		} else if newBody != "" {
			body = body + "\n" + newBody
		}
	case "prepend":
		if body == "" {
			body = newBody
		} else if newBody != "" {
			body = newBody + "\n" + body
		}
	case "append_once":
		// 以行尾去空格的方式做“等价判断”
		norm := func(s string) string {
			var out []string
			for _, l := range strings.Split(s, "\n") {
				out = append(out, strings.TrimRight(l, " \t"))
			}
			return strings.Join(out, "\n")
		}
		if strings.Contains(norm(body), norm(newBody)) {
			logger.Infof("append_once：内容已存在，跳过（%s #%s @index=%d）", b.Path, b.Anchor, b.Index)
		} else {
			if body == "" {
				body = newBody
			} else if newBody != "" {
				body = body + "\n" + newBody
			}
		}
	default:
		body = newBody
	}

	var out string
	if head != "" && head[len(head)-1] != '\n' {
		head += "\n"
	}
	out = head + body
	if tail != "" && tail[0] != '\n' {
		out += "\n"
	}
	out += tail

	logger.Okf("区块：%s #%s（%s @index=%d）", b.Path, b.Anchor, b.Mode, b.Index)
	return os.WriteFile(abs, []byte(out), 0o644)
}

func touch(p string) error {
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
// ------------------------------
// XGIT:END BLOCK_ENGINE
// ------------------------------

// ------------------------------
// XGIT:BEGIN FILE_OPS
// ------------------------------
func applyFileWrite(repo string, logger *MultiLogger, fw FileWrite) error {
	abs := filepath.Join(repo, fw.Path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(abs, normalizeLF(fw.Content), 0o644); err != nil {
		return err
	}
	logger.Okf("写入文件：%s", fw.Path)
	_, err := run(repo, "git", "add", "--", fw.Path)
	return err
}

func applyDelete(repo string, logger *MultiLogger, path string) error {
	abs := filepath.Join(repo, path)
	if fileExists(abs) {
		// 优先 git rm；否则物理删除
		if out, err := run(repo, "git", "rm", "-rf", "--", path); err != nil {
			_ = os.RemoveAll(abs)
			logger.Deletef("删除（物理）：%s", path)
			_ = out
		} else {
			logger.Deletef("删除：%s", path)
		}
	} else {
		logger.Infof("跳过删除（不存在）：%s", path)
	}
	return nil
}

func applyMove(repo string, logger *MultiLogger, mv Move) error {
	// 先确保目标目录存在
	absTo := filepath.Join(repo, mv.To)
	_ = os.MkdirAll(filepath.Dir(absTo), 0o755)
	if _, err := run(repo, "git", "mv", "-f", "--", mv.From, mv.To); err != nil {
		// fallback：物理 mv + add
		absFrom := filepath.Join(repo, mv.From)
		if !fileExists(absFrom) {
			logger.Infof("跳过改名（不存在）：%s", mv.From)
			return nil
		}
		if err := os.Rename(absFrom, absTo); err != nil {
			return err
		}
		if _, err := run(repo, "git", "add", "--", mv.To); err != nil {
			return err
		}
	}
	logger.Renamef("改名：%s → %s", mv.From, mv.To)
	return nil
}
// ------------------------------
// XGIT:END FILE_OPS
// ------------------------------

// ------------------------------
// XGIT:BEGIN APPLY_PATCH
// ------------------------------
func applyPatch(cfg Config, logger *MultiLogger, p *Patch, repoMap map[string]string, defAlias string) error {
	alias := strings.TrimSpace(p.RepoAlias)
	if alias == "" {
		if defAlias != "" {
			alias = defAlias
		}
	}
	repoPath := repoMap[alias]
	if repoPath == "" {
		return fmt.Errorf("无效仓库标识：“%s”", alias)
	}
	logger.Infof("仓库：%s", repoPath)

	tx, err := NewTx(repoPath, logger)
	if err != nil {
		return err
	}

	if err := tx.Clean(cfg.CleanMode); err != nil {
		return err
	}

	// 执行 mv/delete/file/block
	for _, m := range p.Moves {
		if err := applyMove(repoPath, logger, m); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, d := range p.Deletes {
		if err := applyDelete(repoPath, logger, d); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, fw := range p.Files {
		if err := applyFileWrite(repoPath, logger, fw); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, b := range p.Blocks {
		if err := applyBlock(repoPath, logger, b); err != nil {
			logger.Errf("%v", err)
			tx.Rollback()
			return err
		}
	}

	// 是否有变更
	if _, err := run(repoPath, "git", "diff", "--cached", "--quiet"); err == nil {
		logger.Infof("无改动需要提交。")
		return nil
	}

	commitMsg := strings.TrimSpace(p.CommitMsg)
	if commitMsg == "" {
		commitMsg = "chore: apply patch"
	}
	author := strings.TrimSpace(p.Author)
	if author == "" {
		author = "XGit Bot <bot@xgit.local>"
	}

	logger.Infof("提交说明：%s", commitMsg)
	logger.Infof("提交作者：%s", author)
	if out, err := run(repoPath, "git", "commit", "--author", author, "-m", commitMsg); err != nil {
		tx.Rollback()
		return fmt.Errorf("提交失败：%v\n%s", err, out)
	} else {
		logger.Okf("已提交：%s", commitMsg)
	}

	if cfg.Push {
		logger.Pushf("正在推送（origin HEAD）…")
		if out, err := run(repoPath, "git", "push", "origin", "HEAD"); err != nil {
			tx.Rollback()
			return fmt.Errorf("推送失败：%v\n%s", err, out)
		}
		logger.Pushf("推送完成")
	} else {
		logger.Infof("已禁用推送（-push=false）")
	}
	return nil
}
// ------------------------------
// XGIT:END APPLY_PATCH
// ------------------------------

// ------------------------------
// XGIT:BEGIN WATCH_LOOP
// ------------------------------
func watchLoop(cfg Config) {
	// 单实例锁
	if _, err := os.Stat(cfg.LockPath); err == nil {
		fmt.Println("❌ 已在运行（锁被占用），退出")
		return
	}
	if err := os.WriteFile(cfg.LockPath, []byte(fmt.Sprintln(os.Getpid())), 0o644); err != nil {
		fmt.Println("❌ 无法创建锁文件：", err)
		return
	}
	defer os.Remove(cfg.LockPath)

	// 每次执行覆盖 patch.log
	logger, closeLog, err := NewMultiLogger(cfg.PatchLog)
	if err != nil {
		fmt.Println("❌ 打开 patch.log 失败：", err)
		return
	}
	defer closeLog()

	logger.Beginf("监听启动：%s", cfg.PatchFile)

	// 预载 .repos
	repoMap, defAlias, err := parseReposMap(cfg.ReposFile)
	if err != nil {
		logger.Errf("解析 .repos 失败：%v", err)
	}

	lastMD5 := ""
	lastSize := int64(0)
	stableCnt := 0

	for {
		if fileExists(cfg.PatchFile) {
			curSize := fileSize(cfg.PatchFile)
			if curSize > 0 && curSize == lastSize {
				stableCnt++
			} else {
				stableCnt = 0
				lastSize = curSize
			}

			if stableCnt >= cfg.StableTries {
				// 去抖
				msSleep(cfg.DebounceMS)
				// 再验尺寸
				curSize2 := fileSize(cfg.PatchFile)
				if curSize2 != curSize {
					stableCnt = 0
					lastSize = curSize2
					continue
				}
				// 严格 EOF
				ll, _ := lastLine(cfg.PatchFile)
				if ll != cfg.EOFMark {
					logger.Waitf("等待严格 EOF 标记“%s”", cfg.EOFMark)
					stableCnt = 0
					continue
				}
				curMD5 := md5sumFile(cfg.PatchFile)
				logger.Infof("补丁稳定（size=%d md5=%s）→ 准备执行", curSize2, curMD5[:8])
				if curMD5 != "" && curMD5 != lastMD5 {
					// 每次执行覆盖 patch.log（重新打开）
					closeLog()
					logger, closeLog, _ = NewMultiLogger(cfg.PatchLog)
					logger.Beginf("开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))

					patch, err := parsePatch(cfg.PatchFile, cfg.EOFMark)
					if err != nil {
						logger.Errf("%v", err)
					} else {
						if err := applyPatch(cfg, logger, patch, repoMap, defAlias); err != nil {
							logger.Errf("%v", err)
						} else {
							lastMD5 = curMD5
						}
					}
					logger.Okf("本次补丁完成")
				}
				stableCnt = 0
			}
		}
		msSleep(cfg.IntervalMS)
	}
}
// ------------------------------
// XGIT:END WATCH_LOOP
// ------------------------------

// ------------------------------
// XGIT:BEGIN MAIN_CLI
// ------------------------------
func doStop(lock string) {
	if !fileExists(lock) {
		fmt.Println("✅ 已停止（无锁）")
		return
	}
	// 尝试读取 pid 并杀掉
	b, _ := os.ReadFile(lock)
	pidStr := strings.TrimSpace(string(b))
	_ = os.Remove(lock)
	if pidStr == "" {
		fmt.Println("✅ 已停止（清理锁）")
		return
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if _, err := run("", "kill", "-TERM", pidStr); err == nil {
			fmt.Println("✅ 已停止（进程已终止）")
			return
		}
	}
	fmt.Println("✅ 已停止（清理锁）")
}

func doStatus(lock string) {
	if !fileExists(lock) {
		fmt.Println("ℹ️ 状态：未运行")
		return
	}
	b, _ := os.ReadFile(lock)
	fmt.Printf("ℹ️ 状态：运行中（pid=%s）\n", strings.TrimSpace(string(b)))
}

func main() {
	cfg := defaultConfig()
	cmd := parseFlags(&cfg)
	switch cmd {
	case "start":
		watchLoop(cfg)
	case "stop":
		doStop(cfg.LockPath)
	case "status":
		doStatus(cfg.LockPath)
	default:
		fmt.Println("未知命令：", cmd)
		os.Exit(2)
	}
}
// ------------------------------
// XGIT:END MAIN_CLI
// ------------------------------
