/*
XGIT FileOps: file.image
说明：写入图片（Base64），和 binary 类似，仅语义区分（后续可做额外校验）
*/
// XGIT:BEGIN GO:PACKAGE
package fileops

// XGIT:END GO:PACKAGE

// XGIT:BEGIN GO:IMPORTS
import (
	"encoding/base64"
	"os"
	"path/filepath"
)

// XGIT:END GO:IMPORTS

// XGIT:BEGIN GO:FUNC_FILE_IMAGE
// FileImage 写入图片 —— 协议: file.image
func FileImage(repo, rel, base64Data string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(abs, raw, 0o644); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("🖼️ file.image 完成：%s (size=%d)", rel, len(raw))
	}
	if err := preflightOne(repo, rel, logger); err != nil {
		if logger != nil {
			logger.Log("❌ 预检失败：%s (%v)", rel, err)
		}
		return err
	}
	return nil
}

// XGIT:END GO:FUNC_FILE_IMAGE
