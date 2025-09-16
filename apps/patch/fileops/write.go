/*
XGIT FileOps: file.write
说明：写入（覆盖）指定文件，若不存在则创建；写入后建议加入暂存（由上层处理）
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

// XGIT:BEGIN GO:FUNC_FILE_WRITE
// FileWrite 写入（覆盖）文件 —— 协议: file.write
func FileWrite(repo, rel string, data []byte, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		if logger != nil {
			logger.Log("❌ file.write mkdir 失败：%s (%v)", rel, err)
		}
		return err
	}
	// 统一 LF；保证末尾换行
	s := string(data)
	s = normalizeLF(s)
	if len(s) > 0 && s[len(s)-1] != '\n' {
		s += "\n"
	}
	if err := os.WriteFile(abs, []byte(s), 0o644); err != nil {
		if logger != nil {
			logger.Log("❌ file.write 写入失败：%s (%v)", rel, err)
		}
		return err
	}
	if logger != nil {
		logger.Log("✅ file.write 完成：%s", rel)
	}
	_ = "" // 占位避免未使用
	return nil
}

// XGIT:END GO:FUNC_FILE_WRITE
