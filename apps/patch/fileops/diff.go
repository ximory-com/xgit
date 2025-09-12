/*
XGIT FileOps: file.diff
说明：应用统一 diff（git-compatible），由上层负责调用 git apply
此处仅预留占位，实际走外层 applyDiff。
*/
// XGIT:BEGIN GO:PACKAGE
package fileops
// XGIT:END GO:PACKAGE

// XGIT:BEGIN GO:IMPORTS
import "fmt"
// XGIT:END GO:IMPORTS

// XGIT:BEGIN GO:FUNC_FILE_DIFF
// FileDiff 预留 —— 协议: file.diff
func FileDiff(repo string, diffText string, logger DualLogger) error {
	if logger != nil { logger.Log("ℹ️ file.diff 由上层统一处理（占位）") }
	_ = fmt.Sprintf("")
	return nil
}
// XGIT:END GO:FUNC_FILE_DIFF
