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
// 2) 依次尝试：
//    (a) git apply --index --3way --reject --whitespace=nowarn
//    (b) git apply        --3way --reject --whitespace=nowarn
// 3) 成功则记录日志并返回 nil；失败会收集 git 输出与 .rej 线索返回 error
//
// 说明：
// - 使用 --3way 可在存在轻微偏移或上下文变化时更稳；
// - 使用 --reject 避免“全盘失败”，若出现 .rej 代表部份 hunk 无法应用；我们会将此视为失败并返回可读错误；
// - 调用方（ApplyOnce）处在事务里，失败将回滚。
func FileDiff(repo string, diffText string, logger *DualLogger) error {
	log := func(format string, a ...any) {
		if logger != nil {
			(*logger).Log(format, a...)
		}
	}

	if strings.TrimSpace(diffText) == "" {
		return errors.New("file.diff: 空 diff")
	}

	// 写入临时补丁文件（放在 repo 内，避免路径问题）
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

	// 尝试序列：优先带 --index，然后不带 --index
	attempts := [][]string{
		{"apply", "--index", "--3way", "--reject", "--whitespace=nowarn", tmp},
		{"apply", "--3way", "--reject", "--whitespace=nowarn", tmp},
	}

	var firstErr error
	for i, args := range attempts {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Run(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("git %s 失败：%v\n%s", strings.Join(args, " "), err, out.String())
			}
			log("⚠️ git %s 失败（尝试 #%d）：%v", strings.Join(args, " "), i+1, err)
			continue
		}

		// 成功
		log("✅ file.diff 已应用（尝试 #%d 成功）", i+1)

		// 检查是否产生 .rej（有 .rej 说明存在未能自动合入的 hunk）
		rejs, _ := findRejects(repo)
		if len(rejs) > 0 {
			var sb strings.Builder
			for _, r := range rejs {
				sb.WriteString(" - ")
				sb.WriteString(r)
				sb.WriteString("\n")
			}
			return fmt.Errorf("file.diff: 存在未能应用的 hunk（生成 .rej）：\n%s", sb.String())
		}
		return nil
	}

	// 两次尝试都失败，补充 .rej 线索（如果有）
	rejs, _ := findRejects(repo)
	if len(rejs) > 0 {
		var sb strings.Builder
		for _, r := range rejs {
			sb.WriteString(" - ")
			sb.WriteString(r)
			sb.WriteString("\n")
		}
		return fmt.Errorf("%v\nfile.diff: 同时检测到 .rej 文件（可能是上下文不匹配）：\n%s", firstErr, sb.String())
	}
	if firstErr != nil {
		return firstErr
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
			// 适度跳过 .git
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
