package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return nil
}
func runCmdOut(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
func gitRevParseHEAD(repo string) (string, error) {
	return runCmdOut("git", "-C", repo, "rev-parse", "--verify", "HEAD")
}
func gitResetHard(repo, rev string) error {
	if rev == "" {
		return runCmd("git", "-C", repo, "reset", "--hard")
	}
	return runCmd("git", "-C", repo, "reset", "--hard", rev)
}
func gitCleanFD(repo string) error {
	return runCmd("git", "-C", repo, "clean", "-fd")
}

// WithGitTxn：在 repo 上开启一次 Git 事务：fn() 出错则回滚到补丁前状态。
func WithGitTxn(repo string, logf func(string, ...any), fn func() error) error {
	preHead, _ := gitRevParseHEAD(repo)
	_ = gitResetHard(repo, "")
	_ = gitCleanFD(repo)

	var err error
	defer func() {
		if err != nil {
			if preHead != "" {
				_ = gitResetHard(repo, preHead)
			} else {
				_ = gitResetHard(repo, "")
			}
			_ = gitCleanFD(repo)
			if logf != nil {
				logf("↩️ 回滚到补丁前状态：%s", preHead)
			}
		}
	}()

	if e := fn(); e != nil {
		err = e
		return err
	}
	return nil
}

// collectChangedFiles: 收集已改动文件（新增/修改/删除/重命名后的新路径）
func collectChangedFiles(repo string) ([]string, error) {
	out, err := runCmdOut("git", "-C", repo, "status", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}

	var changed []string
	parts := strings.Split(out, "\x00")
	for _, p := range parts {
		if p == "" {
			continue
		}
		// 如果是 rename，格式为 "R  old -> new"，只取 new
		if strings.Contains(p, "->") {
			fields := strings.Split(p, "->")
			last := strings.TrimSpace(fields[len(fields)-1])
			p = last
		}

		// 最终清理
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// 跳过目录（保险）
		full := filepath.Join(repo, p)
		if fi, err := os.Stat(full); err == nil && fi.IsDir() {
			continue
		}

		changed = append(changed, p)
	}
	return changed, nil
}
