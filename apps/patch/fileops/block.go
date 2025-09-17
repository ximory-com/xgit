package fileops

import (
	"fmt"
	"path/filepath"
)

// block.delete —— 删除一个作用域（start-keys / end-keys）内的整段
//   - 无 start-keys：全文
//   - 无 end-keys：到 EOF
//   - start 多处 → 用 nthb 选择；end 多处 → 取第一处
func BlockDelete(repo, rel string, args map[string]string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	lines, err := readLines(abs)
	if err != nil {
		return err
	}
	sc, err := resolveScope(lines, args)
	if err != nil {
		return fmt.Errorf("block.delete: %w", err)
	}
	if sc.start < 1 || sc.end < sc.start || sc.end > len(lines) {
		return fmt.Errorf("block.delete: 非法范围 [%d..%d]", sc.start, sc.end)
	}
	delN := sc.end - sc.start + 1
	lines = splice(lines, sc.start-1, delN, nil)
	lines = ensureTrailingNL(lines)
	if err := writeLines(abs, lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("🗑️ block.delete  %s:[%d..%d] (-%d)", rel, sc.start, sc.end, delN)
	}
	return stageAndPreflight(repo, rel, logger)
}

// block.replace —— 用正文替换一个作用域内的整段
func BlockReplace(repo, rel, body string, args map[string]string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	lines, err := readLines(abs)
	if err != nil {
		return err
	}
	sc, err := resolveScope(lines, args)
	if err != nil {
		return fmt.Errorf("block.replace: %w", err)
	}
	if sc.start < 1 || sc.end < sc.start || sc.end > len(lines) {
		return fmt.Errorf("block.replace: 非法范围 [%d..%d]", sc.start, sc.end)
	}
	newLines := splitPayload(body)
	delN := sc.end - sc.start + 1
	lines = splice(lines, sc.start-1, delN, newLines)
	lines = ensureTrailingNL(lines)
	if err := writeLines(abs, lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("✏️ block.replace %s:[%d..%d] (%d→%d)", rel, sc.start, sc.end, delN, len(newLines))
	}
	return stageAndPreflight(repo, rel, logger)
}
