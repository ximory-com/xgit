package main

// XGIT:BEGIN FILE-HEADER
// pidutil.go — 进程 PID 文件以及存活检测
// XGIT:END FILE-HEADER

// XGIT:BEGIN IMPORTS
import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)
// XGIT:END IMPORTS

// XGIT:BEGIN PID-IMPL
func writePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

func readPID(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func processAlive(pid int) bool {
	p := exec.Command("ps", "-p", strconv.Itoa(pid))
	if err := p.Run(); err != nil {
		return false
	}
	return true
}

func killProcess(pid int) error {
	// 尝试优雅停止
	if err := exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run(); err != nil {
		// 强制
		_ = exec.Command("kill", "-KILL", strconv.Itoa(pid)).Run()
	}
	return nil
}
// XGIT:END PID-IMPL
