package main

// XGIT:BEGIN FILE-HEADER
// main.go — 入口与 CLI（start/stop/status）
// 依赖：DualLogger、LoadRepos、Watcher(StableAndEOF)、ParsePatch(text,eofMark)、ApplyOnce、PID 工具
// XGIT:END FILE-HEADER

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	eofMark   = "=== PATCH EOF ==="
	pidName   = ".xgit_patchd.pid"
	patchName = "文本.txt"
)

func usage() {
	fmt.Println("用法: xgit_patchd [start|stop|status]")
}

// CLI: xgit_patchd [start|stop|status]
func main() {
	baseDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	pidFile := filepath.Join(baseDir, pidName)
	patchFile := filepath.Join(baseDir, patchName)

	if len(os.Args) < 2 {
		usage()
		return
	}
	switch strings.ToLower(os.Args[1]) {
	case "start":
		if pid, ok := readPID(pidFile); ok && processAlive(pid) {
			logger, _ := NewDualLogger(baseDir)
			defer logger.Close()
			logger.Log("已在运行 (pid=%d)", pid)
			return
		}
		logger, err := NewDualLogger(baseDir)
		if err != nil {
			logger, _ = NewDualLogger(baseDir)
			defer logger.Close()
			logger.Log("logger 初始化失败: %v", err)
			return
		}
		defer logger.Close()

		logger.Log("▶ xgit_patchd 启动，监听：%s", patchFile)

		if err := writePID(pidFile, os.Getpid()); err != nil {
			logger.Log("⚠️ 写 PID 失败：%v", err)
		}
		defer func() { _ = os.Remove(pidFile) }()

		w := NewWatcher(patchFile, eofMark, logger)

		lastHash := loadLastHash(baseDir)
		for {
			ok, size, h8 := w.StableAndEOF()
			if ok && h8 != "" && h8 != lastHash {
				logger.Log("📦 补丁稳定（size=%d md5=%s）→ 准备执行", size, h8)

				// ↓↓↓ 修正：不要把 ReadFile 的 err 当成 eof 传；把 []byte 转成 string
				data, rerr := os.ReadFile(patchFile)
				if rerr != nil {
					logger.Log("❌ 读取补丁失败：%v", rerr)
					lastHash = h8
					saveLastHash(baseDir, h8)
					continue
				}
				patch, perr := ParsePatch(string(data), eofMark) // 期望签名：ParsePatch(text string, eof string)
				if perr != nil {
					logger.Log("❌ 解析补丁失败：%v", perr)
					lastHash = h8
					saveLastHash(baseDir, h8)
					continue
				}

				ApplyOnce(logger, "", patch, patchFile)

				lastHash = h8
				saveLastHash(baseDir, h8)
			}
			time.Sleep(500 * time.Millisecond)
		}
	case "stop":
		if pid, ok := readPID(pidFile); ok && processAlive(pid) {
			_ = killProcess(pid)
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
	case "clearhash":
		clearHash(baseDir)
	default:
		usage()
	}
}

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

func clearHash(baseDir string) {
	hashFile := filepath.Join(baseDir, ".lastpatch")
	if err := os.Remove(hashFile); err == nil {
		fmt.Printf("🧹 已清除补丁 hash 记录：%s", hashFile)
	} else if os.IsNotExist(err) {
		fmt.Printf("ℹ️ 未找到补丁 hash 文件，无需清理")
	} else {
		fmt.Printf("❌ 清理补丁 hash 失败：%v", err)
	}
}
