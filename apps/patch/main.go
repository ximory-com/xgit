package main

// XGIT:BEGIN FILE-HEADER
// main.go — 入口与 CLI（start/stop/status）
// 依赖：DualLogger、LoadRepos、Watcher(StableAndEOF)、ParsePatch([]byte,eof)、ApplyOnce、PID 工具
// XGIT:END FILE-HEADER

// XGIT:BEGIN IMPORTS
import (
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

		// watcher
		w := NewWatcher(patchFile, eofMark, logger)

		lastHash := loadLastHash(baseDir)
		for {
			ok, size, h8 := w.StableAndEOF()
			if ok && h8 != "" && h8 != lastHash {
				logger.Log("📦 补丁稳定（size=%d md5=%s）→ 准备执行", size, h8)

				data, err := os.ReadFile(patchFile)
				if err != nil {
					logger.Log("❌ 读取补丁失败：%v", err)
					lastHash = h8
					time.Sleep(700 * time.Millisecond)
					continue
				}

				eof := w.EOFMark
				pt, err := ParsePatch(string(data), eof)
				if err != nil {
					logger.Log("❌ 解析失败：%v", err)
					lastHash = h8
					time.Sleep(700 * time.Millisecond)
					continue
				}

				// repo 选择：优先补丁头 repo: <name|/abs/path>，否则 .repos 的 default
				repoPath, err := resolveRepo(baseDir, patchFile)
				if err != nil {
					logger.Log("❌ %v", err)
					lastHash = h8
					time.Sleep(700 * time.Millisecond)
					continue
				}

				ApplyOnce(logger, repoPath, pt, patchFile)
				lastHash = h8
				saveLastHash(baseDir, lastHash)
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

// XGIT:BEGIN RESOLVE-REPO
// 统一仓库解析逻辑
func resolveRepo(baseDir, patchFile string) (string, error) {
	// 1) 读 .repos
	repos, def := LoadRepos(baseDir)

	// 2) 读补丁头 repo:
	target := HeaderRepoName(patchFile)
	if strings.TrimSpace(target) == "" {
		target = def
	}
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("无法解析仓库：既无补丁头 repo:，也无 .repos default")
	}

	// 3) 按规则返回（禁止绝对路径）
	if strings.HasPrefix(target, "/") {
		return "", fmt.Errorf("不支持绝对路径仓库：%s（请在 .repos 里用名字映射）", target)
	}
	repoPath := strings.TrimSpace(repos[target])
	if repoPath == "" {
		return "", fmt.Errorf("在 .repos 中未找到仓库名映射：%s", target)
	}
	return repoPath, nil
}
// XGIT:END RESOLVE-REPO

// XGIT:BEGIN LAST-HASH
func saveLastHash(baseDir, hash string) {
	_ = os.WriteFile(filepath.Join(baseDir, ".lastpatch"), []byte(hash), 0644)
}

func loadLastHash(baseDir string) string {
	data, err := os.ReadFile(filepath.Join(baseDir, ".lastpatch"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
// XGIT:END LAST-HASH