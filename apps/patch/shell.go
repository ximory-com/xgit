// XGIT:BEGIN PACKAGE
package main
// XGIT:END PACKAGE

// XGIT:BEGIN IMPORTS
import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)
// XGIT:END IMPORTS

// XGIT:BEGIN SHELL
// Shell：执行外部命令，返回 stdout/stderr
func Shell(parts ...string) (string, string, error) {
	if len(parts) == 0 {
		return "", "", errors.New("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	var out, er bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &er
	err := cmd.Run()
	return strings.TrimRight(out.String(), "\n"),
		strings.TrimRight(er.String(), "\n"), err
}

// 兼容小写
func shell(parts ...string) (string, string, error) { return Shell(parts...) }
// XGIT:END SHELL
