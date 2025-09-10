package main

// XGIT:BEGIN IMPORTS
// 说明：工具函数（路径规范 + 进程/锁等）
import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)
// XGIT:END IMPORTS

// XGIT:BEGIN NORM_PATH
// 说明：*.md 或无扩展 => 文件名大写；其余 => 文件名小写；扩展小写；去首尾空白
func Lower(s string) string { return strings.ToLower(s) }
func Upper(s string) string { return strings.ToUpper(s) }
func NormPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.ReplaceAll(p, "//", "/")
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	name, ext := base, ""
	if i := strings.LastIndex(base, "."); i >= 0 {
		name, ext = base[:i], base[i+1:]
	}
	extL := Lower(ext)
	if ext == "" || extL == "md" {
		name = Upper(name)
	} else {
		name = Lower(name)
	}
	if extL != "" {
		base = name + "." + extL
	} else {
		base = name
	}
	if dir == "." {
		return base
	}
	return filepath.Join(dir, base)
}
// XGIT:END NORM_PATH

// XGIT:BEGIN PROC_LOCK
// 说明：进程/锁
func writePID(lock string) error {
	if _, err := os.Stat(lock); err == nil {
		// 已存在则认为忙
		return io.EOF
	}
	return os.WriteFile(lock, []byte(os.GetpidString()), 0644)
}
func readPID(lock string) int {
	b, err := os.ReadFile(lock)
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(b))
	var pid int
	_, _ = fmtSscanf(s, "%d", &pid)
	return pid
}
func processAlive(pid int) bool {
	// 最简单可移植的探测：发 0 信号不具备；这里只判断>0
	return pid > 0
}
func fmtSscanf(s, f string, a ...any) (int, error) { return fmt.Sscanf(s, f, a...) }
// XGIT:END PROC_LOCK

// XGIT:BEGIN IO_HELPERS
// 说明：文件 md5 / 读取最后一行（非空）
func FileMD5(path string) (string, error) {
	all, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := md5.Sum(all)
	return hex.EncodeToString(h[:]), nil
}
func LastMeaningfulLine(r io.Reader) string {
	sc := bufio.NewScanner(r)
	last := ""
	for sc.Scan() {
		t := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(t) != "" {
			last = t
		}
	}
	return last
}
// XGIT:END IO_HELPERS
