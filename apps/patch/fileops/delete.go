/*
XGIT FileOps: file.delete
说明：删除目标文件/目录（递归）；不存在则忽略
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

// XGIT:BEGIN GO:FUNC_FILE_DELETE
// FileDelete 删除 —— 协议: file.delete
func FileDelete(repo, rel string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	if err := os.RemoveAll(abs); err != nil {
		if logger != nil { logger.Log("❌ file.delete 失败：%s (%v)", rel, err) }
		return err
	}
	if logger != nil { logger.Log("🗑️ file.delete 完成：%s", rel) }
	return nil
}
// XGIT:END GO:FUNC_FILE_DELETE
