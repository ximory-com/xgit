/*
XGIT FileOps: file.delete
说明：删除目标文件/目录（递归）；不存在则忽略；额外清理自底向上的空父目录
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
		if logger != nil {
			logger.Log("❌ file.delete 失败：%s (%v)", rel, err)
		}
		return err
	}
	if logger != nil {
		logger.Log("🗑️ file.delete 完成：%s", rel)
	}

	// 额外：自底向上清理空父目录（不触及仓库根）
	pruneEmptyParents(repo, rel, logger)
	return nil
}

// 清理空父目录（一路向上，直到遇到非空目录或仓库根）
func pruneEmptyParents(repo, rel string, logger DualLogger) {
	dir := filepath.Dir(rel)
	repoAbs, _ := filepath.Abs(repo)

	for {
		if dir == "." || dir == "/" {
			return
		}
		abs := filepath.Join(repo, dir)
		absClean, _ := filepath.Abs(abs)
		if absClean == repoAbs {
			return // 不删仓库根
		}

		ents, err := os.ReadDir(abs)
		if err != nil {
			return
		}
		if len(ents) == 0 {
			if err := os.Remove(abs); err == nil {
				if logger != nil {
					logger.Log("🧹 已清理空目录：%s", dir)
				}
				dir = filepath.Dir(dir)
				continue
			}
		}
		return
	}
}
// XGIT:END GO:FUNC_FILE_DELETE
