package patch

// 文件稳定检测 + 严格 EOF 去抖
// 导出：NewWatcher, (*Watcher).StableAndEOF

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"time"
)

type Watcher struct {
	PatchFile string
	EOFMark   string
	logger    *DualLogger
	eofWarned bool
}

// NewWatcher 构造
func NewWatcher(patchFile, eof string, logger *DualLogger) *Watcher {
	return &Watcher{PatchFile: patchFile, EOFMark: eof, logger: logger}
}

// StableAndEOF：文件大小连续稳定 + 末行等于 EOF 标记 => ok，并返回 size 与 md5 前8位
func (w *Watcher) StableAndEOF() (ok bool, size int, hash8 string) {
	fi, err := os.Stat(w.PatchFile)
	if err != nil || fi.Size() <= 0 {
		return false, 0, ""
	}
	size1 := fi.Size()
	time.Sleep(300 * time.Millisecond) // 简易稳定检测
	fi2, err2 := os.Stat(w.PatchFile)
	if err2 != nil || fi2.Size() != size1 {
		return false, 0, ""
	}

	// 严格 EOF
	f, _ := os.Open(w.PatchFile)
	defer f.Close()
	last := lastMeaningfulLine(f)
	if last != w.EOFMark {
		if !w.eofWarned {
			w.logger.Log("⏳ 等待严格 EOF 标记“%s”", w.EOFMark)
			w.eofWarned = true
		}
		return false, 0, ""
	}
	w.eofWarned = false

	all, _ := os.ReadFile(w.PatchFile)
	sum := md5.Sum(all)
	return true, int(size1), hex.EncodeToString(sum[:])[:8]
}

func lastMeaningfulLine(r io.Reader) string {
	sc := bufio.NewScanner(r)
	last := ""
	for sc.Scan() {
		s := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(s) != "" {
			last = s
		}
	}
	return last
}
