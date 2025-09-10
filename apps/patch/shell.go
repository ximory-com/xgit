package main

// XGIT:BEGIN IMPORTS
// 说明：轻量 shell 包装
import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)
// XGIT:END IMPORTS

// XGIT:BEGIN SHELL
// 说明：执行命令，返回 (stdout, stderr, error)
func Shell(parts ...string) (string, string, error) {
	if len(parts) == 0 {
		return "", "", errors.New("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	var out, er bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &er
	err := cmd.Run()
	return strings.TrimRight(out.String(), "\n"), strings.TrimRight(er.String(), "\n"), err
}
// XGIT:END SHELL
