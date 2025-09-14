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

// 依赖（在其它文件已提供）：
// - type DualLogger interface{ Log(format string, a ...any) }
// - runGit(repo string, logger DualLogger, args ...string) (string, error)
// - findRejects(repo string) ([]string, error)
//
// 设计（lean，无开关）：
// - 不建影子 worktree，不做语言级预检/修复（gofmt/EOL等）。
// - 最小必要：清洗补丁文本 -> intent add -N -> git apply（多策略）。
// - 新增/删除/重命名：跳过 3-way；纯修改：优先 3-way。
// - 失败时打印 git 输出 + 出错行上下文（±20），并报告 .rej（如有）。

// Diff 应用 diffText 到 repo
func Diff(repo string, diffText string, logger DualLogger) error {
	log := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}
	if strings.TrimSpace(diffText) == "" {
		return errors.New("git.diff: 空 diff")
	}

	// 1) 预处理并基本校验
	diffText = sanitizeDiff(diffText)
	if !looksLikeDiff(diffText) {
		return errors.New("git.diff: 输入不是有效的 diff（缺少 diff 头）")
	}

	// 2) 写临时补丁（不保留、不预览）
	patchPath, cleanup, err := writeTempPatch(repo, diffText)
	if err != nil {
		log("❌ git.diff 临时文件失败：%v", err)
		return err
	}
	defer cleanup()

	// 3) 针对新增/重命名做 intent add -N，提升 --index 命中率
	intentAddFromDiff(repo, diffText, logger)

	// 4) 选择策略并尝试应用（只 apply 一次，不重复）
	strategies := buildStrategiesFromDiff(diffText)

	log("📄 git.diff 正在应用补丁：%s", filepath.Base(patchPath))
	if err := applyWithStrategies(repo, patchPath, strategies, logger); err != nil {
		return wrapPatchErrorWithContext(patchPath, err, logger)
	}

	log("✅ git.diff 完成")
	return nil
}

// ---------- 策略 & 辅助 ----------

// 从完整 diff 文本判断是否包含新增/删除/重命名
func analyzeDiffKinds(s string) (hasAddOrDelete bool, hasRename bool) {
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "new file mode "),
			strings.HasPrefix(t, "deleted file mode "),
			strings.HasPrefix(t, "--- /dev/null"),
			strings.HasPrefix(t, "+++ /dev/null"):
			hasAddOrDelete = true
		case strings.HasPrefix(t, "rename from "),
			strings.HasPrefix(t, "rename to "):
			hasRename = true
		}
	}
	return
}

// 根据 diff 类型选择策略序列
func buildStrategiesFromDiff(s string) [][]string {
	hasAddOrDelete, hasRename := analyzeDiffKinds(s)
	// 重命名/新增/删除：跳过 3-way
	if hasAddOrDelete || hasRename {
		return [][]string{
			{"--whitespace=nowarn"},            // 直贴
			{"--index", "--whitespace=nowarn"}, // 如需更新 index（存在时生效）
		}
	}
	// 纯修改：优先 3way 提高成功率
	return [][]string{
		{"--index", "--3way", "--whitespace=nowarn"},
		{"--3way", "--whitespace=nowarn"},
		{"--index", "--whitespace=nowarn"},
		{"--whitespace=nowarn"},
	}
}

// intentAddFromDiff 对 a/ 和 b/ 路径、以及 rename from/to 的路径做 git add -N
func intentAddFromDiff(repo string, diffText string, logger DualLogger) {
	paths, _, _, _ := parseDiffPaths(diffText)

	addN := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || p == "/dev/null" {
			return
		}
		// 忽略明显目录
	if strings.HasSuffix(p, "/") {
			return
		}
		_, _ = runGit(repo, logger, "add", "-N", p)
	}

	// a/ 与 b/ 路径
	for _, p := range paths.aPaths {
		addN(p)
	}
	for _, p := range paths.bPaths {
		addN(p)
	}
	// rename from/to
	froms, tos := parseRenamePairs(diffText)
	for _, p := range froms {
		addN(p)
	}
	for _, p := range tos {
		addN(p)
	}
}

// 解析 rename from/to
func parseRenamePairs(s string) (froms []string, tos []string) {
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "rename from ") {
			froms = append(froms, strings.TrimSpace(strings.TrimPrefix(t, "rename from ")))
		} else if strings.HasPrefix(t, "rename to ") {
			tos = append(tos, strings.TrimSpace(strings.TrimPrefix(t, "rename to ")))
		}
	}
	return
}

// 执行策略集合（含 .rej 检查与报错上下文）
func applyWithStrategies(repo string, patchPath string, strategies [][]string, logger DualLogger) error {
	var lastOut string
	var lastErr error

	for i, args := range strategies {
		full := append([]string{"apply"}, append(args, patchPath)...)
		out, err := runGit(repo, logger, full...)
		if err != nil {
			lastOut, lastErr = out, err
			if logger != nil {
				logger.Log("⚠️ git %v 失败（策略 #%d）：%v", args, i+1, err)
			}
			// 尝试从错误输出里提取“at line N”，打印上下文
			if line := extractPatchErrorLine(out); line > 0 {
				if ctx := readPatchContext(patchPath, line, 20); ctx != "" && logger != nil {
					logger.Log("🧭 出错行上下文（±20）：\n%s", ctx)
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
		if logger != nil {
			logger.Log("✅ git.diff 完成（策略 #%d）", i+1)
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("%v\n%s", lastErr, lastOut)
	}
	return errors.New("git.diff: git apply 失败（未知原因）")
}

// sanitizeDiff 移除 ```diff / ```patch 围栏，trim 两端空白，并确保末尾有换行
func sanitizeDiff(s string) string {
	s = strings.TrimSpace(s)
	// 剥离三反引号围栏
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
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

// extractPatchErrorLine：尝试从 git 输出中提取 “at line N”
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
		fmt.Fprintf(&b, "%5d| %s\n", i, string(lines[i-1]))
	}
	return b.String()
}

// writeTempPatch 把文本写入 repo 下的临时 .patch 文件，并返回路径和清理函数
func writeTempPatch(repo string, text string) (string, func(), error) {
	dir := repo
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	f, err := os.CreateTemp(dir, ".xgit_*.patch")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()

	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return "", nil, err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		return "", nil, err
	}

	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}

// wrapPatchErrorWithContext：把 git apply 错误输出补充上下文
func wrapPatchErrorWithContext(patchPath string, err error, logger DualLogger) error {
	out := fmt.Sprintf("%v", err)
	var tail strings.Builder
	if line := extractPatchErrorLine(out); line > 0 {
		if ctx := readPatchContext(patchPath, line, 20); ctx != "" {
			tail.WriteString("\n🧭 出错行上下文（±20）：\n")
			tail.WriteString(ctx)
		}
	}
	return fmt.Errorf("%v%s", err, tail.String())
}