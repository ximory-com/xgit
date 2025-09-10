package main

// XGIT:BEGIN IMPORTS
// 说明：watcher（稳定判断 + 严格 EOF 去抖）
import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"time"
)

// XGIT:END IMPORTS

// XGIT:BEGIN WATCH
// 说明：稳定文件 + 严格 EOF；并打印一次性等待提示（去抖）
type Watcher struct {
	PatchFile string
	EOFMark   string
	Logger    *DualLogger
	eofWarned bool
}

func (w *Watcher) StableAndEOF() (ok bool, size int, hash8 string) {
	fi, err := os.Stat(w.PatchFile)
	if err != nil || fi.Size() <= 0 { return false, 0, "" }
	size1 := fi.Size()
	time.Sleep(300 * time.Millisecond)
	fi2, err2 := os.Stat(w.PatchFile)
	if err2 != nil || fi2.Size() != size1 { return false, 0, "" }

	f, _ := os.Open(w.PatchFile); defer f.Close()
	line := lastLine(f)
	if line != w.EOFMark {
		if !w.eofWarned {
			w.Logger.Log("⏳ 等待严格 EOF 标记“%s”", w.EOFMark)
			w.eofWarned = true
		}
		return false, 0, ""
	}
	w.eofWarned = false

	all, _ := os.ReadFile(w.PatchFile)
	h := md5.Sum(all)
	return true, int(size1), hex.EncodeToString(h[:])[:8]
}

func lastLine(r io.Reader) string {
	sc := bufio.NewScanner(r)
	last := ""
	for sc.Scan() {
		t := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(t) != "" { last = t }
	}
	return last
}
// XGIT:END WATCH
