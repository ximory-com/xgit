package preflight

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type jsonRunner struct{}

func (jsonRunner) Name() string { return "json-pretty" }
func (jsonRunner) Match(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".json"
}
func (jsonRunner) Run(repo, rel string, logf Logf) (bool, error) {
	abs := filepath.Join(repo, rel)
	orig, err := os.ReadFile(abs)
	if err != nil {
		return false, err
	}
	// 保存权限/mtime 与 EOL
	isCRLF := bytes.Contains(orig, []byte("\r\n"))
	mode := os.FileMode(0o644)
	mtime := time.Time{}
	if fi, _ := os.Stat(abs); fi != nil {
		mode = fi.Mode()
		mtime = fi.ModTime()
	}

	// 解析校验
	var v any
	dec := json.NewDecoder(bytes.NewReader(orig))
	dec.DisallowUnknownFields() // 严一点：多余字段时报错（可按需放宽）
	if err := dec.Decode(&v); err != nil {
		logf("❌ preflight(json): %s 语法错误：%v", rel, err)
		return false, err
	}

	// pretty 输出（2 空格）
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return false, err
	}
	out := buf.Bytes()
	// 恢复 EOL 风格
	if isCRLF {
		out = toCRLF(out)
	}
	// 若无变化则跳过
	if bytes.Equal(orig, out) {
		logf("🧪 preflight(json): %s 已是规范格式", rel)
		return false, nil
	}

	if err := atomicWrite(abs, out, mode, mtime); err != nil {
		return false, err
	}
	logf("🛠️ preflight(json): 已格式化 %s", rel)
	return true, nil
}

func init() { Register(jsonRunner{}) }
