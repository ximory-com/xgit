// XGIT:BEGIN PACKAGE
package main
// XGIT:END PACKAGE

// XGIT:BEGIN IMPORTS
import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)
// XGIT:END IMPORTS

// XGIT:BEGIN PIDUTIL
func writePID(path string, pid int) error {
	return os.WriteFile(path, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func readPID(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := string(b)
	pid, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func processAlive(pid int) bool {
	// 在 Unix/macOS 上，向进程发送 0 信号，可用于探测是否存在
	err := syscall.Kill(pid, 0)
	return err == nil
}

func killProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}
// XGIT:END PIDUTIL
