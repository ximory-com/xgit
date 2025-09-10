package main

// XGIT:BEGIN IMPORTS
// 说明：日志模块 import（可按需追加）
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)
// 可追加日志相关 import，例如："runtime/debug"

// XGIT:END IMPORTS

// XGIT:BEGIN LOGGING
// 说明：控制台 + patch.log 双路输出；每次补丁触发前截断 patch.log
type DualLogger struct {
	Console io.Writer
	File    *os.File
	w       io.Writer
}

func NewDualLogger(patchDir string) (*DualLogger, error) {
	logPath := filepath.Join(patchDir, "patch.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	d := &DualLogger{Console: os.Stdout, File: f}
	d.w = io.MultiWriter(d.Console, d.File)
	return d, nil
}
func (d *DualLogger) Close() { if d != nil && d.File != nil { _ = d.File.Close() } }
func (d *DualLogger) Log(format string, a ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(d.w, "%s %s\n", ts, fmt.Sprintf(format, a...))
}
// XGIT:END LOGGING
// XGIT:BEGIN LOGGING_HEADER
package main

import (
	"io"
	"os"
	"path/filepath"
	"time"
)
// XGIT:END LOGGING_HEADER
