package main

// XGIT:BEGIN IMPORTS
// 说明：import 整块锚点（可整体替换或增删项）
import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)
// 这里可以追加额外 import（append_once 确保不重复），例如："net/http"

// XGIT:END IMPORTS

// XGIT:BEGIN MAIN
// 说明：程序入口 + CLI（start|stop|status），默认 start（前台）
func main() {
	baseDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	patchFile := filepath.Join(baseDir, "文本.txt")
	patchDir := baseDir
	eof := "=== PATCH EOF ==="

	logger, err := NewDualLogger(patchDir)
	if err != nil {
		fmt.Println("logger init 失败:", err)
		return
	}
	defer logger.Close()

	cmd := "start"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	lock := filepath.Join(patchDir, ".xgit_patchd.lock")

	switch cmd {
	case "stop":
		stop(lock, logger); return
	case "status":
		status(lock, logger); return
	case "start":
		// fallthrough
	default:
		// 无参兼容
	}

	// 单实例
	if err := writePID(lock); err != nil {
		logger.Log("❌ 另一个实例正在运行（锁被占用），退出")
		return
	}
	defer os.Remove(lock)

	logger.Log("▶ xgit_patchd 启动，监听：%s", patchFile)

	// 加载 .repos（支持 default = name；name -> 绝对路径）
	repos, def := LoadRepos(patchDir)

	w := &Watcher{PatchFile: patchFile, EOFMark: eof, Logger: logger}
	var lastHash string

	for {
		ok, size, h8 := w.StableAndEOF()
		if ok && h8 != "" && h8 != lastHash {
			logger.Log("📦 补丁稳定（size=%d md5=%s）→ 准备执行", size, h8)

			pt, err := ParsePatch(patchFile, eof)
			if err != nil {
				logger.Log("❌ 解析失败：%v", err)
				lastHash = h8 // 防止同一内容反复解析
				time.Sleep(700 * time.Millisecond)
				continue
			}

			// 解析 repo：优先头字段 repo: <name|/abs>，其次 default
			targetName := def
			if name := headerRepoName(patchFile); name != "" {
				targetName = name
			}
			repoPath := repos[targetName]
			if repoPath == "" && strings.HasPrefix(targetName, "/") {
				repoPath = targetName
			}
			if repoPath == "" {
				logger.Log("❌ 无法解析仓库（.repos 或 repo: 头字段）")
				lastHash = h8
				time.Sleep(700 * time.Millisecond)
				continue
			}

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

func stop(lock string, logger *DualLogger) {
	pid := readPID(lock)
	if pid <= 0 {
		logger.Log("ℹ️ 未发现运行中的实例（无锁文件）")
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		logger.Log("⚠️ 找不到进程（pid=%d），清理锁并返回", pid)
		_ = os.Remove(lock)
		return
	}
	_ = p.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	_ = os.Remove(lock)
	logger.Log("✅ 已请求停止（pid=%d），锁已清理", pid)
}

func status(lock string, logger *DualLogger) {
	pid := readPID(lock)
	if pid <= 0 {
		logger.Log("ℹ️ 未运行")
		return
	}
	if processAlive(pid) {
		logger.Log("ℹ️ 正在运行（pid=%d）", pid)
	} else {
		logger.Log("⚠️ 锁存在但进程不在，建议执行：xgit_patchd stop")
	}
}
// XGIT:END MAIN
