// Package patch: utility helpers (path normalize, stage, write)
// XGIT:BEGIN UTIL_HEADER
package patch

import (
	"os"
	"path/filepath"
	"strings"
)
// XGIT:END UTIL_HEADER

// XGIT:BEGIN NORM_PATH
// NormPath: 标准化路径（*.md 或无扩展 => 文件名大写；其他 => 文件名小写；扩展一律小写）
func NormPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.ReplaceAll(p, "//", "/")
	dir := filepath.Dir(p)
	base := filepath.Base(p)

	name, ext := base, ""
	if i := strings.LastIndex(base, "."); i >= 0 {
		name, ext = base[:i], base[i+1:]
	}
	extL := strings.ToLower(ext)
	if ext == "" || extL == "md" {
		name = strings.ToUpper(name)
	} else {
		name = strings.ToLower(name)
	}
	if extL != "" {
		base = name + "." + extL
	} else {
		base = name
	}
	if dir == "." {
		return base
	}
	return filepath.Join(dir, base)
}
// XGIT:END NORM_PATH

// XGIT:BEGIN WRITE_AND_STAGE
// WriteFile: 写入文件并加入暂存；统一 LF 和末尾换行
func WriteFile(repo string, rel string, content string, logf func(string, ...any)) error {
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	content = strings.ReplaceAll(content, "\r", "")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return err
	}
	if logf != nil {
		logf("✅ 写入文件：%s", rel)
	}
	Stage(repo, rel, logf)
	return nil
}

// Stage: 将路径加入暂存
func Stage(repo, rel string, logf func(string, ...any)) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return
	}
	if _, _, err := Shell("git", "-C", repo, "add", "--", rel); err != nil {
		if logf != nil {
			logf("⚠️ 自动加入暂存失败：%s", rel)
		}
	} else {
		if logf != nil {
			logf("🧮 已加入暂存：%s", rel)
		}
	}
}
// XGIT:END WRITE_AND_STAGE
