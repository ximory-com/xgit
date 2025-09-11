/*
XGIT FileOps: file.binary
说明：写入二进制（Base64）；常用于非图片资源
*/
// XGIT:BEGIN GO:PACKAGE
package main
// XGIT:END GO:PACKAGE

// XGIT:BEGIN GO:IMPORTS
import (
	"os"
	"path/filepath"
	"encoding/base64"
)
// XGIT:END GO:IMPORTS

// XGIT:BEGIN GO:FUNC_FILE_BINARY
// FileBinary 写入二进制 —— 协议: file.binary
func FileBinary(repo, rel, base64Data string, logger *DualLogger) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil { return err }
	raw, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil { return err }
	if err := os.WriteFile(abs, raw, 0o644); err != nil { return err }
	if logger != nil { logger.Log("✅ file.binary 完成：%s (size=%d)", rel, len(raw)) }
	return nil
}
// XGIT:END GO:FUNC_FILE_BINARY
