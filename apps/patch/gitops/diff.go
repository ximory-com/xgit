package gitops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// XGIT:BEGIN GITOPS DIFF
// 使用 `git apply` 应用 unified / git diff 补丁文本。
func Diff(repo string, diffText string, logger DualLogger) error {
	log := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}
	if strings.TrimSpace(diffText) == "" {
		return errors.New("git.diff: 空 diff")
	}

	tmpf, err := os.CreateTemp(repo, ".xgit_*.patch")
	if err != nil {
		log("❌ git.diff 临时文件失败：%v", err)
		return err
	}
	tmp := tmpf.Name()
	defer os.Remove(tmp)
	if _, err := tmpf.WriteString(diffText); err != nil {
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

	log("📄 git.diff 正在应用补丁：%s", filepath.Base(tmp))

	// 三段式尝试：优先 --index --3way，然后 --3way，最后直接应用
	attempts := [][]string{
		{"apply", "--index", "--3way", "--whitespace=nowarn", tmp},
		{"apply", "--3way", "--whitespace=nowarn", tmp},
		{"apply", "--whitespace=nowarn", tmp},
	}

	var lastErr error
	for i, args := range attempts {
		if _, err := runGit(repo, logger, args...); err != nil {
			lastErr = err
			log("⚠️ git %v 失败（尝试 #%d）", args, i+1)
			continue
		}
		// 成功后检查是否产生 .rej（有则视为失败，让调用方回滚）
		rejs, _ := findRejects(repo)
		if len(rejs) > 0 {
			var b strings.Builder
			for _, r := range rejs {
				b.WriteString(" - ")
				b.WriteString(r)
				b.WriteString("\n")
			}
			return fmt.Errorf("git.diff: 存在未能应用的 hunk（生成 .rej）：\n%s", b.String())
		}
		log("✅ git.diff 完成（策略 #%d）", i+1)
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("git.diff: git apply 失败（未知原因）")
}
// XGIT:END GITOPS DIFF
