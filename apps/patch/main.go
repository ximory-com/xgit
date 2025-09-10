// XGIT:BEGIN PACKAGE
package main
// XGIT:END PACKAGE

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

// XGIT:BEGIN MAIN
// CLI: xgit_patchd [start|stop|status]
// - start  : 常驻监听 patch/文本.txt，补丁稳定且 EOF 正确则应用
// - stop   : 结束当前守护进程（通过 PID 文件）
// - status : 显示是否运行
func main() {
	baseDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	patchFile := filepath.Join(baseDir, "文本.txt")
	patchDir := baseDir
	eof := "=== PATCH EOF ==="

	// 解析命令
	cmd := "start"
	if len(os.Args) > 1 {
		cmd = strings.ToLower(os.Args[1])
	}

	pidFile := filepath.Join(patchDir, ".xgit_patchd.pid")

	switch cmd {
	case "stop":
		if pid, ok := readPID(pidFile); ok {
			if err := killProcess(pid); err != nil {
				fmt.Printf("⚠️ 尝试结束进程失败(pid=%d)：%v\n", pid, err)
			} else {
				fmt.Printf("🛑 已结束进程(pid=%d)\n", pid)
			}
		} else {
			fmt.Println("ℹ️ 未发现运行中的进程。")
		}
		_ = os.Remove(pidFile)
		return

	case "status":
		if pid, ok := readPID(pidFile); ok && processAlive(pid) {
			fmt.Printf("✅ 运行中（pid=%d）\n", pid)
		} else {
			fmt.Println("⛔ 未运行")
		}
		return
	}

	// start
	logger, err := NewDualLogger(patchDir)
	if err != nil {
		fmt.Println("logger 初始化失败:", err)
		return
	}
	defer logger.Close()

	// 写入 PID（失败不致命）
	_ = writePID(pidFile, os.Getpid())
	logger.Log("▶ xgit_patchd 启动，监听：%s", patchFile)

	// 加载仓库映射
	repos, def := loadRepos(patchDir)

	// watcher
	w := &Watcher{PatchFile: patchFile, EOFMark: eof, logger: logger}
	var lastHash string

	for {
		ok, size, h8 := w.stableAndEOF()
		if ok && h8 != "" && h8 != lastHash {
			logger.Log("📦 补丁稳定（size=%d md5=%s）→ 准备执行", size, h8)

			// 读取补丁并解析
			all, err := os.ReadFile(patchFile)
			if err != nil {
				logger.Log("❌ 读取补丁失败：%v", err)
				lastHash = h8
				time.Sleep(700 * time.Millisecond)
				continue
			}
			pt, err := ParsePatch(all, eof)
			if err != nil {
				logger.Log("❌ 解析失败：%v", err)
				lastHash = h8
				time.Sleep(700 * time.Millisecond)
				continue
			}

			// repo 选择：优先补丁头 repo: <name or abs>
			targetName := def
			if name := headerRepoName(patchFile); name != "" {
				targetName = name
			}
			repoPath := repos[targetName]
			if repoPath == "" && strings.HasPrefix(targetName, "/") {
				repoPath = targetName
			}
			if repoPath == "" {
				logger.Log("❌ 无法解析仓库（.repos 或 repo: 头字段）。")
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
// XGIT:END MAIN
