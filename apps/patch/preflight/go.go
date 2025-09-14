package preflight

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"time"
)

type goFmtRunner struct{}

func (goFmtRunner) Name() string { return "go-fmt" }
func (goFmtRunner) Match(path string) bool {
	return filepath.Ext(path) == ".go"
}
func (goFmtRunner) Run(repo, rel string, logf Logf) (bool, error) {
	abs := filepath.Join(repo, rel)
	orig, err := os.ReadFile(abs)
	if err != nil {
		return false, err
	}

	// 记录 EOL 风格与权限/mtime
	isCRLF := bytes.Contains(orig, []byte("\r\n"))
	mode := os.FileMode(0o644)
	mtime := time.Time{}
	if fi, _ := os.Stat(abs); fi != nil {
		mode = fi.Mode()
		mtime = fi.ModTime()
	}

	// —— 预修复：统一 LF，并把末尾换行规范为“恰好一个” —— //
	in := normalizeLF(orig)

	// 去掉所有尾部 '\n'，再补回一个
	in = bytes.TrimRight(in, "\n")
	in = append(in, '\n')

	// —— gofmt —— //
	formatted, err := format.Source(in)
	if err != nil {
		logf("❌ preflight(go): %s 语法/格式化失败：%v", rel, err)
		return false, err
	}

	// —— 再次规范末尾换行为“恰好一个” —— //
	formatted = bytes.TrimRight(formatted, "\n")
	formatted = append(formatted, '\n')

	// 如原文件是 CRLF，则转换回 CRLF
	out := formatted
	if isCRLF {
		out = toCRLF(out)
	}

	// 若无变化则跳过
	if bytes.Equal(orig, out) {
		logf("🧪 preflight(go): %s 已符合规范与 gofmt", rel)
		return false, nil
	}

	if err := atomicWrite(abs, out, mode, mtime); err != nil {
		return false, err
	}
	logf("🛠️ preflight(go): 规范化末尾换行并格式化 %s", rel)
	return true, nil
}

func init() { Register(goFmtRunner{}) }
