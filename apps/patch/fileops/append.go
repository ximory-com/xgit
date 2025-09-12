/*
XGIT FileOps: file.append
说明：在目标文件末尾追加内容；若不存在则创建
*/
// XGIT:BEGIN GO:PACKAGE
package fileops
// XGIT:END GO:PACKAGE

// XGIT:BEGIN GO:IMPORTS
import (
	"os"
	"path/filepath"
)
// XGIT:END GO:IMPORTS

// XGIT:BEGIN GO:FUNC_FILE_APPEND
// FileAppend 末尾追加 —— 协议: file.append
func FileAppend(repo, rel string, data []byte, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		if logger != nil { logger.Log("❌ file.append mkdir 失败：%s (%v)", rel, err) }
		return err
	}
	b := []byte(normalizeLF(string(data)))
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		if logger != nil { logger.Log("❌ file.append 打开失败：%s (%v)", rel, err) }
		return err
	}
	defer f.Close()
	if len(b) > 0 && b[len(b)-1] != '\n' { b = append(b, '\n') }
	if _, err := f.Write(b); err != nil {
		if logger != nil { logger.Log("❌ file.append 写入失败：%s (%v)", rel, err) }
		return err
	}
	if logger != nil { logger.Log("✅ file.append 完成：%s", rel) }
	return nil
}
// XGIT:END GO:FUNC_FILE_APPEND
