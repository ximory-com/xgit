package gitops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// XGIT:BEGIN GITOPS COMMON
// DualLogger 与主工程保持相同签名，便于注入
type DualLogger interface {
	Log(format string, a ...any)
}

// runGit 执行 git 命令（自动 -C repo），返回合并输出（stdout+stderr）
func runGit(repo string, logger DualLogger, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("git %s 失败：%v\n%s", strings.Join(args, " "), err, out.String())
	}
	return out.String(), nil
}

// runGitQuiet 同 runGit，但仅关心错误
func runGitQuiet(repo string, logger DualLogger, args ...string) error {
	_, err := runGit(repo, logger, args...)
	return err
}

// findRejects 扫描 repo 下的 .rej 文件；仅用于提示
func findRejects(repo string) ([]string, error) {
	var out []string
	_ = filepath.WalkDir(repo, func(p string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if filepath.Base(p) == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".rej") {
			rel, _ := filepath.Rel(repo, p)
			out = append(out, rel)
		}
		return nil
	})
	return out, nil
}
// XGIT:END GITOPS COMMON
