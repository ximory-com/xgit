/*
XGIT FileOps: file.chmod
说明：调整权限，如 0644 / 0755；不存在则忽略
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

// XGIT:BEGIN GO:FUNC_FILE_CHMOD
// FileChmod 权限变更 —— 协议: file.chmod
func FileChmod(repo, rel string, mode os.FileMode, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	if err := os.Chmod(abs, mode); err != nil {
		if logger != nil {
			logger.Log("❌ file.chmod 失败：%s (%v)", rel, err)
		}
		return err
	}
	if logger != nil {
		logger.Log("🔐 file.chmod 完成：%s -> %04o", rel, mode)
	}
	if err := preflightOne(repo, rel, logger); err != nil {
		if logger != nil {
			logger.Log("❌ 预检失败：%s (%v)", rel, err)
		}
		return err
	}
	return nil
}

// XGIT:END GO:FUNC_FILE_CHMOD
