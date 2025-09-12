package preflight

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"time"
)

// normalizeLF 将 \r\n 统一为 \n（不改变 \r 存在的其它位置）
func normalizeLF(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}
func toCRLF(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
}

// 原子写入：tmp -> fsync -> rename，保持权限与 mtime
func atomicWrite(abs string, data []byte, mode os.FileMode, mtime time.Time) error {
	dir := filepath.Dir(abs)
	tmpf, err := os.CreateTemp(dir, ".xgit_preflight_*")
	if err != nil {
		return err
	}
	tmp := tmpf.Name()
	defer os.Remove(tmp)

	if _, err := io.Copy(tmpf, bytes.NewReader(data)); err != nil {
		_ = tmpf.Close()
		return err
	}
	if err := tmpf.Sync(); err != nil {
		_ = tmpf.Close()
		return err
	}
	if err := tmpf.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp, mode)
	if err := os.Rename(tmp, abs); err != nil {
		return err
	}
	if !mtime.IsZero() {
		_ = os.Chtimes(abs, time.Now(), mtime)
	}
	return nil
}
