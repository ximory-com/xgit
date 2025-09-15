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
// 设计（lean，无影子、无语言预检）：
// - 清洗补丁文本 -> intent add -N -> git apply（按 diff 类型选择策略）。
// - 新增/删除/重命名：跳过 3-way；纯修改：优先 3-way。
// - 失败时打印 git 输出与出错行上下文（±20），并报告 .rej（如有）。
// - 成功后逐个文件打印：新建/删除/修改/改名 <路径/对>。

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

	log("📄 git.diff 正在应用补丁：%s", filepath.Base(patchPath))

	// 3) 针对新增/重命名做 intent add -N，提升 --index 命中率（即使策略里先直贴，也不冲突）
	intentAddFromDiff(repo, diffText, logger)

	// 4) 选择策略并尝试应用
	strategies := buildStrategiesFromDiff(diffText)
	var lastOut string
	var lastErr error
	for i, args := range strategies {
		full := append([]string{"apply"}, append(args, patchPath)...)
		out, err := runGit(repo, logger, full...)
		if err != nil {
			lastOut, lastErr = out, err
			log("⚠️ git %v 失败（策略 #%d）：%v", args, i+1, err)
			if line := extractPatchErrorLine(out); line > 0 {
				if ctx := readPatchContext(patchPath, line, 20); ctx != "" {
					log("🧭 出错行上下文（±20）：\n%s", ctx)
				}
			}
			continue
		}
		// 成功后检查是否生成 .rej
		if rejs, _ := findRejects(repo); len(rejs) > 0 {
			var b strings.Builder
			for _, r := range rejs {
				b.WriteString(" - ")
				b.WriteString(r)
				b.WriteString("\n")
			}
			return fmt.Errorf("git.diff: 存在未能应用的 hunk（生成 .rej）：\n%s", b.String())
		}

		// ✨ 成功：解析文件清单并逐条输出（对齐：新建/删除/修改/改名）
		adds, dels, mods, renames := summarizeDiffFiles(diffText)
		printed := false
		if len(adds) > 0 {
			for _, f := range adds {
				log("✅ git.diff 完成（策略 #%d）新建  %s", i+1, f)
				printed = true
			}
		}
		if len(dels) > 0 {
			for _, f := range dels {
				log("✅ git.diff 完成（策略 #%d）删除  %s", i+1, f)
				printed = true
			}
		}
		if len(mods) > 0 {
			for _, f := range mods {
				log("✅ git.diff 完成（策略 #%d）修改  %s", i+1, f)
				printed = true
			}
		}
		if len(renames) > 0 {
			for _, pair := range renames {
				log("✅ git.diff 完成（策略 #%d）改名  %s → %s", i+1, pair[0], pair[1])
				printed = true
			}
		}
		if !printed {
			log("✅ git.diff 完成（策略 #%d）", i+1)
		}
		return nil
	}

	// 5) 全部失败
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

// ---------- 策略 & 辅助 ----------

// analyzeDiffKinds：从完整 diff 文本判断是否包含新增/删除/重命名
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

// sanitizeDiff 只做最小化处理：
// 1) 可选：剥掉首尾 ```...``` 围栏行（不动中间内容）
// 2) 归一化换行: \r\n / \r -> \n
// 3) 确保末尾有且仅有一个 '\n'
// 绝不 TrimSpace、绝不改动任何以 '+', '-', ' ' 开头的 hunk 行
func sanitizeDiff(s string) string {
	// 不改动原始空白，只处理围栏
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		// 去掉首行围栏
		if len(lines) > 0 && strings.HasPrefix(lines[0], "```") {
			lines = lines[1:]
		}
		// 若最后一行是围栏，也去掉
		if len(lines) > 0 {
			last := lines[len(lines)-1]
			if strings.HasPrefix(strings.TrimSpace(last), "```") && strings.TrimSpace(last) == "```" {
				lines = lines[:len(lines)-1]
			}
		}
		s = strings.Join(lines, "\n")
	}

	// 归一化换行（不动行内空白）
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// 确保末尾恰好一个换行：先去掉所有末尾的 \n，再补一个
	s = strings.TrimRight(s, "\n") + "\n"
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

// summarizeDiffFiles：粗略解析文件粒度的新建/删除/修改/改名
func summarizeDiffFiles(s string) (adds, dels, mods []string, renames [][2]string) {
	lines := strings.Split(s, "\n")
	var lastNewFile, lastDeletedFile bool
	var renameFrom, renameTo string

	for _, raw := range lines {
		t := strings.TrimSpace(raw)

		// 标志位：进入某个文件块后，new/deleted 的下一条路径行生效
		if strings.HasPrefix(t, "new file mode ") {
			lastNewFile = true
			lastDeletedFile = false
			continue
		}
		if strings.HasPrefix(t, "deleted file mode ") {
			lastDeletedFile = true
			lastNewFile = false
			continue
		}

		// 路径行
		if strings.HasPrefix(t, "+++ ") {
			path := strings.TrimPrefix(t, "+++ ")
			if path == "/dev/null" {
				// b 为 /dev/null，不是新增
			} else if strings.HasPrefix(path, "b/") {
				p := strings.TrimPrefix(path, "b/")
				if lastNewFile {
					adds = append(adds, p)
					lastNewFile = false
				}
			}
			continue
		}
		if strings.HasPrefix(t, "--- ") {
			path := strings.TrimPrefix(t, "--- ")
			if path == "/dev/null" {
				// a 为 /dev/null，不是删除
			} else if strings.HasPrefix(path, "a/") {
				p := strings.TrimPrefix(path, "a/")
				if lastDeletedFile {
					dels = append(dels, p)
					lastDeletedFile = false
				}
			}
			continue
		}

		// 普通修改：diff --git a/x b/x 且没有 new/deleted/rename 情况
		if strings.HasPrefix(t, "diff --git a/") && strings.Contains(t, " b/") {
			fields := strings.Fields(t)
			if len(fields) >= 3 {
				ap := strings.TrimPrefix(fields[1], "a/")
				bp := strings.TrimPrefix(fields[2], "b/")
				if ap == bp && ap != "/dev/null" {
					mods = append(mods, ap)
				}
			}
			continue
		}

		// 重命名
		if strings.HasPrefix(t, "rename from ") {
			renameFrom = strings.TrimSpace(strings.TrimPrefix(t, "rename from "))
			continue
		}
		if strings.HasPrefix(t, "rename to ") {
			renameTo = strings.TrimSpace(strings.TrimPrefix(t, "rename to "))
			if renameFrom != "" && renameTo != "" {
				renames = append(renames, [2]string{renameFrom, renameTo})
				renameFrom, renameTo = "", ""
			}
			continue
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