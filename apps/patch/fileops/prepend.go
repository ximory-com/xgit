/*
XGIT FileOps: file.prepend
说明：在目标文件开头插入内容；若不存在则创建（等价于 write）
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

// XGIT:BEGIN GO:FUNC_FILE_PREPEND
// FilePrepend 开头插入 —— 协议: file.prepend
func FilePrepend(repo, rel string, data []byte, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	_ = os.MkdirAll(filepath.Dir(abs), 0o755)
	old, _ := os.ReadFile(abs)
	newContent := []byte(normalizeLF(string(data)))
	if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
		newContent = append(newContent, '\n')
	}
	newContent = append(newContent, old...)
	if err := os.WriteFile(abs, newContent, 0o644); err != nil {
		if logger != nil {
			logger.Log("❌ file.prepend 写入失败：%s (%v)", rel, err)
		}
		return err
	}
	if logger != nil {
		logger.Log("✅ file.prepend 完成：%s", rel)
	}
	if err := preflightOne(repo, rel, logger); err != nil {
		if logger != nil {
			logger.Log("❌ 预检失败：%s (%v)", rel, err)
		}
		return err
	}
	return nil
}

// XGIT:END GO:FUNC_FILE_PREPEND
