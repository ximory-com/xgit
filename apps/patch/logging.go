// apps/patch/logging.go
// XGIT:BEGIN PACKAGE
package main
// XGIT:END PACKAGE

// XGIT:BEGIN IMPORTS
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)
// XGIT:END IMPORTS

// XGIT:BEGIN LOGGING
// DualLogger：控制台 + patch.log（每次补丁执行 truncate 重写）
type DualLogger struct {
	Console io.Writer
	File    *os.File
	w       io.Writer
}

func NewDualLogger(patchDir string) (*DualLogger, error) {
	// 确保目录存在
	if err := os.MkdirAll(patchDir, 0755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(patchDir, "patch.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	d := &DualLogger{Console: os.Stdout, File: f}
	d.w = io.MultiWriter(d.Console, d.File)
	return d, nil
}

func (d *DualLogger) Close() {
	if d == nil || d.File == nil {
		return
	}
	_ = d.File.Close()
}

// 导出方法
func (d *DualLogger) Log(format string, a ...any) {
	if d == nil || d.w == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(d.w, "%s %s\n", ts, fmt.Sprintf(format, a...))
}

// 兼容历史小写调用
func (d *DualLogger) log(format string, a ...any) { d.Log(format, a...) }
// XGIT:END LOGGING