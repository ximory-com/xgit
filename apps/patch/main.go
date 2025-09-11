package main

// XGIT:BEGIN FILE-HEADER
// main.go — 入口与 CLI（start/stop/status）
// 依赖：DualLogger、LoadRepos、Watcher(StableAndEOF)、ParsePatch([]byte,eof)、ApplyOnce、PID 工具
// XGIT:END FILE-HEADER

// XGIT:BEGIN IMPORTS
import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)
// XGIT:END IMPORTS

// XGIT:BEGIN CONSTANTS
const (
	eofMark   = "=== PATCH EOF ==="
	pidName   = ".xgit_patchd.pid"
	patchName = "文本.txt"
)
// XGIT:END CONSTANTS

// XGIT:BEGIN HELP
func usage() {
	fmt.Println("用法: xgit_patchd [start|stop|status]")
}
// XGIT:END HELP

// XGIT:BEGIN MAIN
// CLI: xgit_patchd [start|stop|status]
func main() {
	baseDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	patchFile := filepath.Join(baseDir, patchName)
	pidFile := filepath.Join(baseDir, pidName)

	if len(os.Args) < 2 {
		usage()
		return
	}
	cmd := os.Args[1]

	switch cmd {
	case "start":
		// 已有实例？
		if pid, ok := readPID(pidFile); ok && processAlive(pid) {
			fmt.Printf("已在运行 (pid=%d)\n", pid)
			return
		}
		// logger
		logger, err := NewDualLogger(baseDir)
		if err != nil {
			fmt.Println("logger 初始化失败:", err)
			return
		}
		defer logger.Close()

		logger.Log("▶ xgit_patchd 启动，监听：%s", patchFile)

		// 写 PID
		if err := writePID(pidFile, os.Getpid()); err != nil {
			logger.Log("⚠️ 写 PID 失败：%v", err)
		}
		defer func() { _ = os.Remove(pidFile) }()

		// 加载 .repos
		repos, def := LoadRepos(baseDir)

		// watcher
		w := NewWatcher(patchFile, eofMark, logger)

		var lastHash string
		for {
			ok, size, h8 := w.StableAndEOF()
			if ok && h8 != "" && h8 != lastHash {
				logger.Log("📦 补丁稳定（size=%d md5=%s）→ 准备执行", size, h8)

				eof := w.EOFMark
				pt, err := ParsePatch(patchFile, eof)
				if err != nil {
					logger.Log("❌ 解析失败：%v", err)
					lastHash = h8
					time.Sleep(700 * time.Millisecond)
					continue
				}

				// repo 选择：优先补丁头 repo: <name|/abs/path>，否则 .repos 的 default
				target := headerRepoName(patchFile)
				if target == "" {
					target = def
				}
				repoPath := ""
				if strings.HasPrefix(target, "/") {
					repoPath = target
				} else {
					repoPath = repos[target]
				}
				if strings.TrimSpace(repoPath) == "" {
					logger.Log("❌ 无法解析仓库（补丁头 repo: 或 .repos/default）")
					lastHash = h8
					time.Sleep(700 * time.Millisecond)
					continue
				}

				ApplyOnce(logger, repoPath, pt)
				lastHash = h8
			}
			time.Sleep(250 * time.Millisecond)
		}

	case "stop":
		if pid, ok := readPID(pidFile); ok && processAlive(pid) {
			_ = killProcess(pid)
			_ = os.Remove(pidFile)
			fmt.Printf("已发送停止信号 (pid=%d)\n", pid)
			return
		}
		fmt.Println("未发现运行中的进程")
	case "status":
		if pid, ok := readPID(pidFile); ok && processAlive(pid) {
			fmt.Printf("运行中 (pid=%d)\n", pid)
		} else {
			fmt.Println("未运行")
		}
	default:
		usage()
	}
}
// XGIT:END MAIN

// XGIT:BEGIN HEADER-REPO
// 从补丁头部读取 repo: 字段（直到遇到第一个 "===" 为止）
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
// XGIT:END HEADER-REPO
