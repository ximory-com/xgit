package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// DualLogger：控制台 + 补丁目录 patch.log（覆盖写，只保留最近一次）
type DualLogger struct {
	Console io.Writer
	File    *os.File
	w       io.Writer
	path    string
}

// NewDualLogger 在 patchDir 创建/覆盖 patch.log，并把日志同时写入控制台与文件。
// 若文件创建失败，仍返回可用 logger（仅输出到控制台）。
func NewDualLogger(patchDir string) (*DualLogger, error) {
	if patchDir == "" {
		patchDir = "."
	}
	if err := os.MkdirAll(patchDir, 0o755); err != nil {
		// 目录不可用，则仅返回控制台 logger
		l := &DualLogger{Console: os.Stdout, w: os.Stdout}
		return l, err
	}
	logPath := filepath.Join(patchDir, "patch.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	l := &DualLogger{Console: os.Stdout, File: f, path: logPath}
	if err == nil {
		l.w = io.MultiWriter(l.Console, l.File)
	} else {
		l.w = l.Console
	}
	return l, err
}

// Path 返回 patch.log 的绝对路径（若创建失败则为空字符串）
func (d *DualLogger) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

// Close 关闭文件句柄
func (d *DualLogger) Close() error {
	if d != nil && d.File != nil {
		return d.File.Close()
	}
	return nil
}

// Log：带时间戳写入控制台与 patch.log（若存在）
func (d *DualLogger) Log(format string, a ...any) {
	if d == nil || d.w == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(d.w, "%s %s\n", ts, fmt.Sprintf(format, a...))
}

// 兼容历史小写调用
func (d *DualLogger) log(format string, a ...any) { d.Log(format, a...) }
