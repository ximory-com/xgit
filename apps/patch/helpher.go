package main

import (
	"fmt"
	"os/exec"
	"os"
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

// 收集变更文件，过滤目录/重命名 old 端
func collectChangedFiles(repo string) ([]string, error) {
	out, err := runCmdOut("git", "-C", repo, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, raw := range strings.Split(strings.TrimSpace(out), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || len(line) <= 3 {
			continue
		}
		payload := strings.TrimSpace(line[3:])

		// R old -> new 只取 new
		if idx := strings.Index(payload, "->"); idx >= 0 {
			payload = strings.TrimSpace(payload[idx+2:])
		}

		// 跳过明显目录（?? dir/）
		if strings.HasSuffix(payload, "/") {
			continue
		}
		// 文件系统再判一次，目录跳过
		full := filepath.Join(repo, payload)
		if fi, err := os.Stat(full); err == nil && fi.IsDir() {
			continue
		}
		changed = append(changed, payload)
	}
	return changed, nil
}