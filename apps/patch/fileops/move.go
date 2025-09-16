/*
XGIT FileOps: file.move
说明：重命名/移动（支持跨目录）；不存在则忽略
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

// XGIT:BEGIN GO:FUNC_FILE_MOVE
// FileMove 移动/改名 —— 协议: file.move
func FileMove(repo, fromRel, toRel string, logger DualLogger) error {
	from := filepath.Join(repo, fromRel)
	to := filepath.Join(repo, toRel)
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		if logger != nil {
			logger.Log("❌ file.move mkdir 失败：%s -> %s (%v)", fromRel, toRel, err)
		}
		return err
	}
	if err := os.Rename(from, to); err != nil {
		if logger != nil {
			logger.Log("❌ file.move 失败：%s -> %s (%v)", fromRel, toRel, err)
		}
		return err
	}
	if logger != nil {
		logger.Log("🔁 file.move 完成：%s -> %s", fromRel, toRel)
	}
	if err := preflightOne(repo, toRel, logger); err != nil {
		if logger != nil {
			logger.Log("❌ 预检失败：%s (%v)", toRel, err)
		}
		return err
	}
	return nil
}

// XGIT:END GO:FUNC_FILE_MOVE
