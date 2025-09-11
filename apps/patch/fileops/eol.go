/*
XGIT FileOps: file.eol
说明：统一换行风格（lf/crlf），可选确保末行换行
*/
// XGIT:BEGIN GO:PACKAGE
package main
// XGIT:END GO:PACKAGE

// XGIT:BEGIN GO:IMPORTS
import (
	"os"
	"path/filepath"
	"bytes"
)
// XGIT:END GO:IMPORTS

// XGIT:BEGIN GO:FUNC_FILE_EOL
// FileEOL 换行规范化 —— 协议: file.eol
func FileEOL(repo, rel string, style string, ensureNL bool, logger *DualLogger) error {
	abs := filepath.Join(repo, rel)
	b, err := os.ReadFile(abs)
	if err != nil { return err }
	s := string(b)
	switch style {
	case "crlf": s = toCRLF(s)
	default:     s = normalizeLF(s) // lf
	}
	if ensureNL {
		if !bytes.HasSuffix([]byte(s), []byte("\n")) { s += "\n" }
	}
	if err := os.WriteFile(abs, []byte(s), 0o644); err != nil { return err }
	if logger != nil { logger.Log("🧹 file.eol 完成：%s (%s, ensure_nl=%v)", rel, style, ensureNL) }
	return nil
}
// XGIT:END GO:FUNC_FILE_EOL
