package patch

// 体检器（文件级）
// - 文本/二进制判定（启发式）
// - 扩展名语义校验（文本后缀禁止写二进制，反之亦然）
// - 基础魔数识别（PNG/JPEG/GIF/PDF/ZIP 等）
// - EOL 规范化工具（供后续 file.eol/ensure_nl 接线使用）

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
)

// -------------------------
// 判定：是否“像二进制”
// 规则：含 NUL(0x00) 或 非打印控制字符占比过高（粗略）
// -------------------------
func IsBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	var ctrl, nul int
	for _, b := range data {
		if b == 0x00 {
			nul++
			if nul > 0 {
				return true
			}
		}
		// 允许 \t(9) \n(10) \r(13)
		if b < 0x09 || (b > 0x0D && b < 0x20) {
			ctrl++
		}
	}
	return float64(ctrl)/float64(len(data)) > 0.05
}

// -------------------------
// 文本类扩展名集合（可按需扩充）
// -------------------------
var textExt = map[string]bool{
	".txt": true, ".md": true, ".markdown": true,
	".go": true, ".js": true, ".ts": true,
	".json": true, ".toml": true, ".ini": true, ".conf": true,
	".yaml": true, ".yml": true,
	".css": true, ".scss": true,
	".html": true, ".htm": true, ".xml": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	".makefile": true, "makefile": true, // 特殊名
}

// -------------------------
// 二进制常见扩展名（非强制，只用于提示/宽松校验）
// -------------------------
var binaryExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true,
	".pdf": true, ".zip": true, ".gz": true, ".tar": true, ".rar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true,
}

// -------------------------
// 基础魔数匹配（见 magic.go）
// -------------------------
func sniffKind(data []byte) string {
	if kind, ok := matchMagic(data); ok {
		return kind
	}
	if IsBinary(data) {
		return "binary"
	}
	return "text"
}

// -------------------------
// 规范化换行（lf / crlf），并可确保文末换行
// -------------------------
func NormalizeEOL(data []byte, eol string, ensureNL bool) []byte {
	// 先统一为 LF
	s := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	s = bytes.ReplaceAll(s, []byte("\r"), []byte("\n"))

	if ensureNL && (len(s) == 0 || s[len(s)-1] != '\n') {
		s = append(s, '\n')
	}
	switch strings.ToLower(eol) {
	case "crlf", "dos":
		s = bytes.ReplaceAll(s, []byte("\n"), []byte("\r\n"))
	default: // "lf" 或空
		// 保持 LF
	}
	return s
}

// -------------------------
// 校验：文本写入（禁止二进制）
// 用于 file.write/append/prepend 等文本路径
// -------------------------
func ValidateTextWrite(path string, data []byte) error {
	ext := strings.ToLower(filepath.Ext(path))
	kind := sniffKind(data)

	// 强规则：文本类后缀必须是文本内容
	if textExt[ext] || ext == "" || strings.EqualFold(filepath.Base(path), "Makefile") {
		if kind != "text" {
			return errors.New("内容看起来像二进制，禁止写入到文本文件")
		}
		return nil
	}

	// 非文本后缀：放宽，但若明显是二进制也建议你用 file.binary
	if kind == "binary" && !binaryExt[ext] {
		// 只给出温和提醒：由上层决定拦截与否
		return nil
	}
	return nil
}

// -------------------------
// 校验：二进制写入（禁止纯文本）
// 用于 file.binary
// -------------------------
func ValidateBinaryWrite(path string, data []byte) error {
	ext := strings.ToLower(filepath.Ext(path))
	kind := sniffKind(data)

	// 若显著是文本，阻止按二进制写入
	if kind == "text" && !binaryExt[ext] {
		return errors.New("内容看起来是纯文本，不适合用 file.binary")
	}
	return nil
}

// -------------------------
// 轻量 MIME / 类型猜测（可用于日志）
// -------------------------
func GuessKindByExt(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if k, ok := matchMagic(data); ok {
		return k
	}
	if textExt[ext] {
		return "text"
	}
	if binaryExt[ext] {
		return "binary"
	}
	if IsBinary(data) {
		return "binary"
	}
	return "text"
}
