package main

// 兜底实现：LoadRepos / NewWatcher / ApplyOnce
// 目的：修复 main.go 未定义符号，先保证能编译运行；
// 后续若已有同名正式实现，可删除本文件。

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------- LoadRepos ----------
func LoadRepos(patchDir string) (map[string]string, string) {
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
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			path := strings.Join(parts[1:], " ")
			m[name] = path
		}
	}
	return m, def
}

// ---------- Watcher ----------
type Watcher struct {
	PatchFile string
	EOFMark   string
	logger    *DualLogger
	eofWarned bool
}

func NewWatcher(patchFile, eofMark string, logger *DualLogger) *Watcher {
	return &Watcher{PatchFile: patchFile, EOFMark: eofMark, logger: logger}
}

// StableAndEOF：大小 300ms 内稳定，且最后非空行 == EOFMark；返回 ok/size/md5(前8位)
func (w *Watcher) StableAndEOF() (bool, int, string) {
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

	f, _ := os.Open(w.PatchFile)
	defer f.Close()
	if last := _lastLine(f); last != w.EOFMark {
		if !w.eofWarned {
			w.logger.Log("⏳ 等待严格 EOF 标记“%s”", w.EOFMark)
			w.eofWarned = true
		}
		return false, 0, ""
	}
	w.eofWarned = false

	all, _ := os.ReadFile(w.PatchFile)
	sum := md5.Sum(all)
	return true, int(size1), hex.EncodeToString(sum[:])[:8]
}

func _lastLine(r io.Reader) string {
	sc := bufio.NewScanner(r)
	last := ""
	for sc.Scan() {
		s := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(s) != "" {
			last = s
		}
	}
	return last
}

// ---------- ApplyOnce ----------
// 依赖外部已存在：WriteFile、ApplyBlock、Shell、DualLogger.Log、Patch
func ApplyOnce(logger *DualLogger, repo string, p *Patch) {
	logger.Log("▶ 开始执行补丁：%s", time.Now().Format("2006-01-02 15:04:05"))
	logger.Log("ℹ️ 仓库：%s", repo)

	// 清理工作区（auto）
	logger.Log("ℹ️ 自动清理工作区：reset --hard / clean -fd")
	_, _, _ = Shell("git", "-C", repo, "reset", "--hard")
	_, _, _ = Shell("git", "-C", repo, "clean", "-fd")

	// 写文件
	for _, f := range p.Files {
		if err := WriteFile(repo, f.Path, f.Content, logger.Log); err != nil {
			logger.Log("❌ 写入失败：%s (%v)", f.Path, err)
			return
		}
	}

	// 区块
	for _, b := range p.Blocks {
		if err := ApplyBlock(repo, b, logger.Log); err != nil {
			logger.Log("❌ 区块失败：%s #%s (%v)", b.Path, b.Anchor, err)
			return
		}
	}

	// 是否有暂存改动
	names, _, _ := Shell("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		logger.Log("ℹ️ 无改动需要提交。")
		logger.Log("✅ 本次补丁完成")
		return
	}

	// 提交 & 推送
	commit := strings.TrimSpace(p.Commit)
	if commit == "" {
		commit = "chore: apply patch"
	}
	author := strings.TrimSpace(p.Author)
	if author == "" {
		author = "XGit Bot <bot@xgit.local>"
	}
	logger.Log("ℹ️ 提交说明：%s", commit)
	logger.Log("ℹ️ 提交作者：%s", author)

	_, _, _ = Shell("git", "-C", repo, "commit", "--author", author, "-m", commit)
	logger.Log("✅ 已提交：%s", commit)

	logger.Log("🚀 正在推送（origin HEAD）…")
	if _, er, err := Shell("git", "-C", repo, "push", "origin", "HEAD"); err != nil {
		logger.Log("❌ 推送失败：%s", er)
	} else {
		logger.Log("🚀 推送完成")
	}
	logger.Log("✅ 本次补丁完成")
}
