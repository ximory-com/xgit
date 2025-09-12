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

	// 统一为 LF 处理
	in := normalizeLF(orig)
	formatted, err := format.Source(in)
	if err != nil {
		// 语法不合法，直接失败
		logf("❌ preflight(go): %s 语法/格式化失败：%v", rel, err)
		return false, err
	}

	// 恢复原行尾风格
	out := formatted
	if isCRLF {
		out = toCRLF(out)
	}
	// 保证 EOF 换行
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	// 若无变化则跳过
	if bytes.Equal(orig, out) {
		logf("🧪 preflight(go): %s 已符合 go fmt", rel)
		return false, nil
	}

	if err := atomicWrite(abs, out, mode, mtime); err != nil {
		return false, err
	}
	logf("🛠️ preflight(go): 已格式化 %s", rel)
	return true, nil
}

func init() { Register(goFmtRunner{}) }
