// apps/patch/gitops/diff.go
package gitops

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// 依赖：
// - DualLogger 接口（在 gitops/common.go 已声明）
// - runGit(repo, logger, args...) (string, error)：封装 git 命令执行并返回合并输出
// - findRejects(repo) ([]string, error)：扫描 .rej 文件（可放在 gitops/common.go）
//
// 说明：Diff v2 做了这些增强：
// 1) 预处理 diff 文本：剥掉 ```diff/```patch 围栏，trim，保证末尾换行；
// 2) 校验是有效 diff（包含 "diff --git" 或者 '---'/'+++' 头）；
// 3) 解析受影响路径：识别新增/删除（/dev/null / new file mode / deleted file mode），
//    对潜在新增文件先 git add -N，以提升 --index 应用成功率；
// 4) 智能策略：若包含新增/删除则跳过 3-way；否则按优先级尝试：
//       (a) --index --3way
//       (b) --3way
//       (c) --index
//       (d) 直贴
// 5) 失败时给出 git 输出，并从 .patch 文件中打印报错行上下文（±20 行），同时列出 .rej；
// 6) 环境变量：
//    - XGIT_KEEP_PATCH=1    留存临时补丁文件（默认删除）
//    - XGIT_SHOW_PATCH=1    控制台打印补丁预览（最多 200 行，避免爆屏）
func Diff(repo string, diffText string, logger DualLogger) error {
	log := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}
	if strings.TrimSpace(diffText) == "" {
		return errors.New("git.diff: 空 diff")
	}

	// 1) 预处理
	diffText = sanitizeDiff(diffText)

	// 2) 检查是否像个 diff
	if !looksLikeDiff(diffText) {
		return errors.New("git.diff: 输入不是有效的 diff（缺少 diff 头）")
	}

	// 3) 解析路径与新增/删除特征
	paths, hasDevNull, hasNewFileMode, hasDeletedMode := parseDiffPaths(diffText)
	containsAddOrDelete := hasDevNull || hasNewFileMode || hasDeletedMode

	// 对疑似新增（b/ 路径）做意向添加，让 --index 能找到 blob
	if len(paths.bPaths) > 0 {
		for _, p := range paths.bPaths {
			if p == "/dev/null" {
				continue
			}
			_, _ = runGit(repo, logger, "add", "-N", p)
		}
	}

	// 4) 临时补丁文件
	keep := os.Getenv("XGIT_KEEP_PATCH") == "1"
	show := os.Getenv("XGIT_SHOW_PATCH") == "1"

	dir := repo
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	tmpf, err := os.CreateTemp(dir, ".xgit_*.patch")
	if err != nil {
		log("❌ git.diff 临时文件失败：%v", err)
		return err
	}
	tmp := tmpf.Name()
	if !keep {
		defer os.Remove(tmp)
	}
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

	log("📄 git.diff 正在应用补丁：%s", filepath.Base(tmp))
	if show {
		log("📄 补丁预览（最多 200 行）：\n%s", previewLines(diffText, 200))
	}

	// 5) 决策策略集
	strategies := buildStrategies(!containsAddOrDelete)

	// 6) 逐策略尝试
	var lastOut string
	var lastErr error
	for i, args := range strategies {
		full := append([]string{"apply"}, append(args, tmp)...)
		out, err := runGit(repo, logger, full...)
		if err != nil {
			lastOut, lastErr = out, err
			log("⚠️ git %v 失败（策略 #%d）：%v", args, i+1, err)

			// 尝试从错误输出里提取“at line N”，打印上下文
			if line := extractPatchErrorLine(out); line > 0 {
				ctx := readPatchContext(tmp, line, 20)
				if ctx != "" {
					log("🧭 出错行上下文（±20）：\n%s", ctx)
				}
			}
			continue
		}

		// 成功后检查 .rej
		if rejs, _ := findRejects(repo); len(rejs) > 0 {
			var b strings.Builder
			for _, r := range rejs {
				b.WriteString(" - ")
				b.WriteString(r)
				b.WriteString("\n")
			}
			return fmt.Errorf("git.diff: 存在未能应用的 hunk（生成 .rej）：\n%s", b.String())
		}

		log("✅ git.diff 完成（策略 #%d）", i+1)
		return nil
	}

	// 7) 全部失败，补充 .rej 与最后错误
	if rejs, _ := findRejects(repo); len(rejs) > 0 {
		var b strings.Builder
		for _, r := range rejs {
			b.WriteString(" - ")
			b.WriteString(r)
			b.WriteString("\n")
		}
		return fmt.Errorf("%v\n%s\ngit.diff: 同时检测到 .rej 文件：\n%s", lastErr, lastOut, b.String())
	}
	if lastErr != nil {
		return fmt.Errorf("%v\n%s", lastErr, lastOut)
	}
	return errors.New("git.diff: git apply 失败（未知原因）")
}

// ========== 辅助实现 ==========

// sanitizeDiff 移除 ```diff / ```patch 围栏，trim 两端空白，并确保末尾有换行
func sanitizeDiff(s string) string {
	s = strings.TrimSpace(s)
	// 剥离三反引号围栏
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 && strings.HasPrefix(lines[0], "```") {
			// 找到最后一行可能的 ```
			if strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[1 : len(lines)-1]
				s = strings.Join(lines, "\n")
			}
		}
	}
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

// looksLikeDiff 粗略判断是否是有效 diff
func looksLikeDiff(s string) bool {
	// 支持 git diff（带 diff --git 头）或统一 diff（--- / +++）
	return strings.Contains(s, "diff --git ") ||
		(strings.Contains(s, "\n--- ") && strings.Contains(s, "\n+++ "))
}

type parsedPaths struct {
	aPaths []string // from "--- a/..."
	bPaths []string // from "+++ b/..."
}

// parseDiffPaths 解析出 a/ 和 b/ 路径，及新增/删除迹象
func parseDiffPaths(s string) (paths parsedPaths, hasDevNull bool, hasNewFileMode bool, hasDeletedMode bool) {
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		// 头部特征
		if strings.HasPrefix(t, "new file mode ") {
			hasNewFileMode = true
		}
		if strings.HasPrefix(t, "deleted file mode ") {
			hasDeletedMode = true
		}
		// 路径行
		if strings.HasPrefix(t, "--- ") {
			r := strings.TrimSpace(strings.TrimPrefix(t, "--- "))
			if r == "/dev/null" {
				hasDevNull = true
				paths.aPaths = append(paths.aPaths, r)
			} else if strings.HasPrefix(r, "a/") {
				paths.aPaths = append(paths.aPaths, r[2:])
			}
		}
		if strings.HasPrefix(t, "+++ ") {
			r := strings.TrimSpace(strings.TrimPrefix(t, "+++ "))
			if r == "/dev/null" {
				hasDevNull = true
				paths.bPaths = append(paths.bPaths, r)
			} else if strings.HasPrefix(r, "b/") {
				pp := r[2:]
				paths.bPaths = append(paths.bPaths, pp)
			}
		}
	}
	return
}
// buildStrategies 根据是否允许 3-way 返回尝试序列参数（不含 "apply" 与补丁路径）
func buildStrategies(allow3Way bool) [][]string {
	if allow3Way {
		return [][]string{
			{"--index", "--3way", "--whitespace=nowarn"},
			{"--3way", "--whitespace=nowarn"},
			{"--index", "--whitespace=nowarn"},
			{"--whitespace=nowarn"},
		}
	}
	// 新增/删除文件的场景：跳过 3-way
	return [][]string{
		{"--index", "--whitespace=nowarn"},
		{"--whitespace=nowarn"},
	}
}

// extractPatchErrorLine 尝试从 git 输出中提取 “at line N”
func extractPatchErrorLine(out string) int {
	re := regexp.MustCompile(`(?i)\bat line\s+(\d+)\b`)
	if m := re.FindStringSubmatch(out); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	// 兼容 “corrupt patch at line 40”
	re2 := regexp.MustCompile(`(?i)at line\s+(\d+)`)
	if m := re2.FindStringSubmatch(out); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return 0
}

// readPatchContext 读取补丁文件第 line 行附近的上下文（±around 行）
func readPatchContext(path string, line, around int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := bytes.Split(data, []byte("\n"))
	if line < 1 {
		line = 1
	}
	start := line - around
	if start < 1 {
		start = 1
	}
	end := line + around
	if end > len(lines) {
		end = len(lines)
	}
	var b bytes.Buffer
	for i := start; i <= end; i++ {
		// 行号对齐输出
		fmt.Fprintf(&b, "%5d| %s\n", i, string(lines[i-1]))
	}
	return b.String()
}

// previewLines 打印前 n 行（避免日志爆屏）
func previewLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}