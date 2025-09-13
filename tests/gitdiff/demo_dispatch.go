package main

import (
	"errors"
	"fmt"
)

// demo_dispatch.go：最小可用雏形，用于增量补丁验证
type FileOp struct {
	Cmd  string
	Path string
	Body string
	Args map[string]string
}

type DualLogger interface{ Log(format string, a ...any) }

func applyOp(repo string, op *FileOp, logger DualLogger) error {
	switch op.Cmd {
	case "noop":
		if logger != nil {
			logger.Log("noop on %s", op.Path)
		}
		_ = fmt.Sprintf // keep imported
		return nil
	default:
		return errors.New("未知指令: " + op.Cmd)
	}
}
