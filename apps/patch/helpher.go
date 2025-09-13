package main

import (
	"fmt"
	"os/exec"
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

func collectChangedFiles(repo string) ([]string, error) {
    out, err := runCmdOut("git", "-C", repo, "status", "--porcelain")
    if err != nil {
        return nil, err
    }
    files := make([]string, 0, 32)
    for _, line := range strings.Split(out, "\n") {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        // 格式：XY<space>path（可能包含重命名 -> 取右侧路径）
        if len(line) > 3 {
            p := strings.TrimSpace(line[3:])
            // 处理类似 "R  old -> new"
            if i := strings.Index(p, " -> "); i >= 0 {
                p = strings.TrimSpace(p[i+4:])
            }
            files = append(files, p)
        }
    }
    return files, nil
}