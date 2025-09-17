package fileops

import (
	"fmt"
	"path/filepath"
)

// line.insert  —— 在定位到的“目标行”之前插入（支持多行）
func LineInsert(repo, rel, body string, args map[string]string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	lines, err := readLines(abs)
	if err != nil {
		return err
	}
	sc, err := resolveScope(lines, args) // 无 start-keys → 全文
	if err != nil {
		return fmt.Errorf("line.insert: %w", err)
	}
	loc, err := resolveLineInScope(lines, sc, args)
	if err != nil {
		return fmt.Errorf("line.insert: %w", err)
	}
	insert := splitPayload(body)
	lines = insertAt(lines, loc-1, insert)
	lines = ensureTrailingNL(lines)
	if err := writeLines(abs, lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("➕ line.insert: %s:L%d (+%d)", rel, loc, len(insert))
	}
	return stageAndPreflight(repo, rel, logger)
}

// line.append —— 在定位到的“目标行”之后插入（支持多行）
func LineAppend(repo, rel, body string, args map[string]string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	lines, err := readLines(abs)
	if err != nil {
		return err
	}
	sc, err := resolveScope(lines, args)
	if err != nil {
		return fmt.Errorf("line.append: %w", err)
	}
	loc, err := resolveLineInScope(lines, sc, args)
	if err != nil {
		return fmt.Errorf("line.append: %w", err)
	}
	insert := splitPayload(body)
	lines = insertAt(lines, loc, insert)
	lines = ensureTrailingNL(lines)
	if err := writeLines(abs, lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("➕ line.append: %s:L%d (+%d)", rel, loc, len(insert))
	}
	return stageAndPreflight(repo, rel, logger)
}

// line.replace —— 将“目标行”整行替换为正文（支持多行）
func LineReplace(repo, rel, body string, args map[string]string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	lines, err := readLines(abs)
	if err != nil {
		return err
	}
	sc, err := resolveScope(lines, args)
	if err != nil {
		return fmt.Errorf("line.replace: %w", err)
	}
	loc, err := resolveLineInScope(lines, sc, args)
	if err != nil {
		return fmt.Errorf("line.replace: %w", err)
	}
	newLines := splitPayload(body)
	lines = splice(lines, loc-1, 1, newLines)
	lines = ensureTrailingNL(lines)
	if err := writeLines(abs, lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("✏️ line.replace: %s:L%d (1→%d)", rel, loc, len(newLines))
	}
	return stageAndPreflight(repo, rel, logger)
}

// line.delete —— 删除“目标行”
func LineDelete(repo, rel string, args map[string]string, logger DualLogger) error {
	abs := filepath.Join(repo, rel)
	lines, err := readLines(abs)
	if err != nil {
		return err
	}
	sc, err := resolveScope(lines, args)
	if err != nil {
		return fmt.Errorf("line.delete: %w", err)
	}
	loc, err := resolveLineInScope(lines, sc, args)
	if err != nil {
		return fmt.Errorf("line.delete: %w", err)
	}
	old := lines[loc-1]
	lines = splice(lines, loc-1, 1, nil)
	lines = ensureTrailingNL(lines)
	if err := writeLines(abs, lines); err != nil {
		return err
	}
	if logger != nil {
		logger.Log("🗑️ line.delete: %s:L%d (-1) %q", rel, loc, old)
	}
	return stageAndPreflight(repo, rel, logger)
}
