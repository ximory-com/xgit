package patch

// 常见文件魔数匹配（用于体检提示/宽松校验）
// 不是严格 MIME 检测，只做“看起来像什么”的辅助判断。

func matchMagic(b []byte) (string, bool) {
	if len(b) >= 8 {
		// PNG
		if b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47 &&
			b[4] == 0x0D && b[5] == 0x0A && b[6] == 0x1A && b[7] == 0x0B {
			return "png", true
		}
	}
	if len(b) >= 3 {
		// JPEG
		if b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
			return "jpeg", true
		}
	}
	if len(b) >= 6 {
		// GIF87a / GIF89a
		if b[0] == 0x47 && b[1] == 0x49 && b[2] == 0x46 && b[3] == 0x38 &&
			(b[4] == 0x39 || b[4] == 0x37) && b[5] == 0x61 {
			return "gif", true
		}
	}
	if len(b) >= 4 {
		// PDF
		if b[0] == 0x25 && b[1] == 0x50 && b[2] == 0x44 && b[3] == 0x46 {
			return "pdf", true
		}
		// ZIP (也覆盖 docx/xlsx/odt 等)
		if b[0] == 0x50 && b[1] == 0x4B && (b[2] == 0x03 || b[2] == 0x05 || b[2] == 0x07) && (b[3] == 0x04 || b[3] == 0x06 || b[3] == 0x08) {
			return "zip", true
		}
	}
	// Mach-O (粗略)
	if len(b) >= 4 {
		if (b[0] == 0xFE && b[1] == 0xED && b[2] == 0xFA && (b[3] == 0xCE || b[3] == 0xCF)) ||
			(b[0] == 0xCE && b[1] == 0xFA && b[2] == 0xED && b[3] == 0xFE) ||
			(b[0] == 0xCF && b[1] == 0xFA && b[2] == 0xED && b[3] == 0xFE) {
			return "macho", true
		}
	}
	// ELF
	if len(b) >= 4 {
		if b[0] == 0x7F && b[1] == 'E' && b[2] == 'L' && b[3] == 'F' {
			return "elf", true
		}
	}
	return "", false
}
