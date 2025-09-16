package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// 在 main 包里提供一个 runGit 薄封装，避免依赖 gitops 包的未导出函数。
// 统一用 runCmdOut 调 git，并把 repo 变成 -C <repo>
func runGit(repo string, logger *DualLogger, args ...string) (string, error) {
	argv := append([]string{"-C", repo}, args...)
	out, err := runCmdOut("git", argv...)

	if logger != nil {
		joined := strings.Join(args, " ")

		// 只对关键命令打印 stdout，避免噪音
		printable :=
			strings.HasPrefix(joined, "apply ") ||
				strings.HasPrefix(joined, "push ") ||
				strings.HasPrefix(joined, "commit ") ||
				strings.HasPrefix(joined, "status ")

		// 像 "diff --cached --name-only -z" 这种就别打印了
		if printable {
			if s := strings.TrimSpace(out); s != "" {
				logger.Log("%s", s)
			}
		}
	}
	return out, err
}

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
