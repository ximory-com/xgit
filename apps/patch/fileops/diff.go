// apps/patch/fileops/diff.go
package fileops

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileDiff 使用 `git apply` 在 repo 上应用 unified diff / git diff 格式的补丁文本。
// 行为：
// 1) 将传入 diff 文本写入 repo 目录下的临时 .patch 文件
// 2) 依次尝试（注意：绝不把 --reject 与 --3way 同时使用）：
//    (a) git -C <repo> apply --index --3way --whitespace=nowarn <patch>
//    (b) git -C <repo> apply        --3way --whitespace=nowarn <patch>
//    (c) git -C <repo> apply --reject        --whitespace=nowarn <patch>
//    (d) git -C <repo> apply                     --whitespace=nowarn <patch>
// 3) 任一步成功即返回；若产生 .rej 文件则视为失败并回滚（由外层事务处理）。
//
// 说明：
// - --3way 能在存在上下文偏移时更稳；
// - --reject 可保留无法合并的 hunk 为 .rej，便于人工处理；但与 --3way 互斥；
// - 调用方（ApplyOnce）处在事务里，失败将回滚。
func FileDiff(repo string, diffText string, logger DualLogger) error {
	log := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}

	if strings.TrimSpace(diffText) == "" {
		return errors.New("file.diff: 空 diff")
	}

	// 写入临时补丁文件（放在 repo 内，避免相对路径问题）
	dir := repo
	if dir == "" {
		dir = "."
	}
	tmpf, err := os.CreateTemp(dir, ".xgit_*.patch")
	if err != nil {
		log("❌ file.diff 临时文件失败：%v", err)
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

	log("📄 file.diff 正在应用补丁：%s", filepath.Base(tmp))

	try := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", repo, "apply"}, args...)...)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git apply %v 失败：%w\n%s", args, err, buf.String())
		}
		return nil
	}

	// 依次降级尝试（不混用 --reject 与 --3way）
	steps := [][]string{
		{"--index", "--3way", "--whitespace=nowarn", tmp},
		{"--3way", "--whitespace=nowarn", tmp},
		{"--reject", "--whitespace=nowarn", tmp}, // 与 --3way 互斥
		{"--whitespace=nowarn", tmp},
	}

	var lastErr error
	for i, s := range steps {
		if err := try(s...); err != nil {
			lastErr = err
			log("⚠️ %s", err.Error())
			continue
		}
		// 成功；若出现 .rej 仍视为失败（需要人工处理）
		if rejs, _ := findRejects(repo); len(rejs) > 0 {
			var sb strings.Builder
			for _, r := range rejs {
				sb.WriteString(" - ")
				sb.WriteString(r)
				sb.WriteString("\n")
			}
			return fmt.Errorf("file.diff: 存在未能应用的 hunk（生成 .rej）：\n%s", sb.String())
		}
		log("✏️ file.diff 完成（策略 #%d）", i+1)
		return nil
	}

	// 全部失败，补充 .rej 线索（如果有）
	if rejs, _ := findRejects(repo); len(rejs) > 0 {
		var sb strings.Builder
		for _, r := range rejs {
			sb.WriteString(" - ")
			sb.WriteString(r)
			sb.WriteString("\n")
		}
		return fmt.Errorf("file.diff: 所有策略失败；检测到 .rej：\n%s\n最后错误：%v", sb.String(), lastErr)
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("file.diff: git apply 失败（未知原因）")
}

// findRejects 简单扫描 repo 下的 .rej 文件；仅做提示用途
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
