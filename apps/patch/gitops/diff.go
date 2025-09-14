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

	"xgit/apps/patch/preflight"
)

// 依赖（已在 gitops/common.go 等处提供）：
// - type DualLogger interface{ Log(format string, a ...any) }
// - runGit(repo string, logger DualLogger, args ...string) (string, error)
// - findRejects(repo string) ([]string, error)
//
// v3 变更：在真实仓库应用前，先在“影子 worktree”应用原始补丁 -> 影子里跑预检（含 gofmt 与末尾仅一换行修复）
// -> 用影子导出“规范化补丁” -> 再按策略集应用到真实仓库。
// 好处：统一把 go 文件末尾换行/gofmt 问题在影子阶段修好，最大化降低 corrupt/预检不通过。

// Diff 应用 diffText 到 repo。
func Diff(repo string, diffText string, logger DualLogger) error {
	log := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}
	if strings.TrimSpace(diffText) == "" {
		return errors.New("git.diff: 空 diff")
	}

	// 1) 预处理并校验 diff
	diffText = sanitizeDiff(diffText)
	if !looksLikeDiff(diffText) {
		return errors.New("git.diff: 输入不是有效的 diff（缺少 diff 头）")
	}
	// 👇👇 新增：校验每个 hunk 头是否带 -n,m +n,m
	if err := validateHunkHeaders(diffText); err != nil {
		return fmt.Errorf("git.diff: 无效 hunk 头：%w", err)
	}

	// 路径/新增删除特征（决定策略）
	_, hasDevNull, hasNewMode, hasDelMode := parseDiffPaths(diffText)
	allow3 := !(hasDevNull || hasNewMode || hasDelMode)

	keep := os.Getenv("XGIT_KEEP_PATCH") == "1"
	show := os.Getenv("XGIT_SHOW_PATCH") == "1"

	// 2) 先把“原始补丁”写入临时文件（用于影子仓库尝试）
	rawPatch, err := writeTempPatch(repo, diffText, keep)
	if err != nil {
		log("❌ git.diff 临时文件失败：%v", err)
		return err
	}
	if show {
		log("📄 补丁预览（最多 200 行）：\n%s", previewLines(diffText, 200))
	}

	// 3) 影子 worktree：应用原始补丁 -> 预检 -> 导出规范化补丁
	shadow, cleanupShadow, err := addShadowWorktree(repo, logger)
	if err != nil {
		return err
	}
	defer cleanupShadow()

	// 影子里先做意向 add（提升 --index 命中率）
	intentAddFromDiff(shadow, diffText, logger)

	log("📄 [影子] 正在应用原始补丁：%s", filepath.Base(rawPatch))
	if err := applyWithStrategies(shadow, rawPatch, allow3, logger); err != nil {
		return wrapPatchErrorWithContext(rawPatch, err, logger)
	}

	// 影子里收集变更并预检（含 go fmt/末尾换行统一）
	changed, _ := collectChangedFiles(shadow, logger)
	if len(changed) > 0 {
		log("🧪 [影子] 预检：%d 个文件", len(changed))
		if err := runPreflights(shadow, changed, diffText, logger); err != nil {
			log("❌ [影子] 预检失败：%v", err)
			return err
		}
	} else {
		log("ℹ️ [影子] 无文件变更")
	}

	// 用影子导出“规范化补丁”
	normText, err := exportNormalizedPatch(shadow, logger)
	if err != nil {
		return err
	}
	if strings.TrimSpace(normText) == "" {
		log("ℹ️ [影子] 规范化后无改动需要应用。")
		return nil
	}
	// 规范化补丁也要校验/决定策略
	_, nHasDevNull, nHasNewMode, nHasDelMode := parseDiffPaths(normText)
	nAllow3 := !(nHasDevNull || nHasNewMode || nHasDelMode)

	normPatch, err := writeTempPatch(repo, normText, keep)
	if err != nil {
		return err
	}

	// 4) 在真实仓库应用“规范化补丁”
	log("📄 git.diff 正在应用补丁：%s", filepath.Base(normPatch))
	if err := applyWithStrategies(repo, normPatch, nAllow3, logger); err != nil {
		// 打印错误上下文 & .rej
		return wrapPatchErrorWithContext(normPatch, err, logger)
	}
	// 成功检查 .rej
	if rejs, _ := findRejects(repo); len(rejs) > 0 {
		var b strings.Builder
		for _, r := range rejs {
			b.WriteString(" - ")
			b.WriteString(r)
			b.WriteString("\n")
		}
		return fmt.Errorf("git.diff: 存在未能应用的 hunk（生成 .rej）：\n%s", b.String())
	}

	log("✅ git.diff 完成（规范化补丁）")
	return nil
}

// ---------- 影子阶段 & 预检 & 规范化导出 ----------

// addShadowWorktree 新建影子工作区
func addShadowWorktree(repo string, logger DualLogger) (shadow string, cleanup func(), err error) {
	shadow, err = os.MkdirTemp("", "xgit_shadow_*")
	if err != nil {
		return "", nil, fmt.Errorf("创建影子工作区失败：%w", err)
	}
	if _, e := runGit(repo, logger, "worktree", "add", "--detach", shadow, "HEAD"); e != nil {
		os.RemoveAll(shadow)
		return "", nil, fmt.Errorf("git worktree add 失败：%w", e)
	}
	cleanup = func() {
		_, _ = runGit(repo, logger, "worktree", "remove", "--force", shadow)
		_ = os.RemoveAll(shadow)
	}
	return shadow, cleanup, nil
}

// intentAddFromDiff 对 b/ 路径做 git add -N
func intentAddFromDiff(repo string, diffText string, logger DualLogger) {
	paths, _, _, _ := parseDiffPaths(diffText)
	for _, p := range paths.bPaths {
		if p == "/dev/null" {
			continue
		}
		_, _ = runGit(repo, logger, "add", "-N", p)
	}
}

// collectChangedFiles 用 git status --porcelain 收集变更路径
func collectChangedFiles(repo string, logger DualLogger) ([]string, error) {
	out, err := runGit(repo, logger, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 3 {
			changed = append(changed, strings.TrimSpace(line[3:]))
		}
	}
	return changed, nil
}

// runPreflights 跑注册的预检器（含 goFmtRunner -> 末尾仅一换行 + gofmt）
func runPreflights(repo string, files []string, diffText string, logger DualLogger) error {
	log := func(f string, a ...any) { if logger != nil { logger.Log(f, a...) } }
	for _, rel := range files {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		// 跳过影子中已删除的文件
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil && os.IsNotExist(err) {
			continue
		}
		// 🔑 新增：删除补丁的 go 文件跳过预检
		if strings.HasSuffix(rel, ".go") && shouldSkipGoPreflight(rel, diffText) {
			log("🗑️ 跳过 go 预检（删除文件）：%s", rel)
			continue
		}

		lang := preflight.DetectLangByExt(rel)
		if lang == "" {
			lang = "unknown"
		}
		log("🧪 预检 %s (%s)", rel, lang)

		if r := preflight.Lookup(rel); r != nil {
			changed, err := r.Run(repo, rel, func(fmt string, a ...any) {
				if logger != nil {
					logger.Log(fmt, a...)
				}
			})
			if err != nil {
				return fmt.Errorf("预检失败 %s: %w", rel, err)
			}
			if changed {
				log("🛠️ 预检已修改 %s", rel)
			} else {
				log("✔ 预检通过，无需修改：%s", rel)
			}
		} else {
			log("ℹ️ 无匹配的预检器：%s", rel)
		}
	}
	return nil
}

// exportNormalizedPatch 把影子中的变更导出为“规范化补丁”（git diff）
func exportNormalizedPatch(shadow string, logger DualLogger) (string, error) {
	// 全量 add、导出 diff（不带颜色，含二进制）
	_, _ = runGit(shadow, logger, "add", "-A")
	out, err := runGit(shadow, logger, "diff", "--no-color", "--binary")
	if err != nil {
		return "", fmt.Errorf("导出规范化补丁失败：%w", err)
	}
	return out, nil
}

// applyWithStrategies 依次尝试策略集（允许/禁止 3-way）
func applyWithStrategies(repo string, patchPath string, allow3 bool, logger DualLogger) error {
	strategies := buildStrategies(allow3)
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

// ---------- 通用小工具 ----------

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

// writeTempPatch 把文本写入 repo 下的临时 .patch 文件
func writeTempPatch(repo string, text string, keep bool) (string, error) {
	dir := repo
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	f, err := os.CreateTemp(dir, ".xgit_*.patch")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if !keep {
		defer func() {
			// 删除动作由调用者在 apply 成功/失败后统一处理更安全；
			// 这里不 defer remove，避免提前删。调用者可设置 XGIT_KEEP_PATCH 控制保留。
		}()
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, nil
}

// wrapPatchErrorWithContext 把 git apply 错误输出补充上下文与 .rej 列表
func wrapPatchErrorWithContext(patchPath string, err error, logger DualLogger) error {
	out := fmt.Sprintf("%v", err)
	var tail strings.Builder
	if line := extractPatchErrorLine(out); line > 0 {
		if ctx := readPatchContext(patchPath, line, 20); ctx != "" {
			tail.WriteString("\n🧭 出错行上下文（±20）：\n")
			tail.WriteString(ctx)
		}
	}
	// .rej 信息在调用层已有检查；这里仅返回拼接后的错误
	return fmt.Errorf("%v%s", err, tail.String())
}
// ================= 新增：放在文件中工具函数区域 =================

// shouldSkipGoPreflight 判断某文件在 diff 中是否纯删除，供 runPreflights 跳过 go 预检
func shouldSkipGoPreflight(rel string, diffText string) bool {
	lines := strings.Split(diffText, "\n")
	inFile := false
	onlyMinus := true
	seenAny := false

	for _, l := range lines {
		// 进入对应文件块
		if strings.HasPrefix(l, "--- a/") {
			path := strings.TrimPrefix(strings.TrimSpace(l), "--- a/")
			inFile = (path == rel)
			onlyMinus = true
			seenAny = false
			continue
		}
		if !inFile {
			continue
		}
		// 退出文件块
		if strings.HasPrefix(l, "diff --git ") {
			break
		}
		// hunk 行
		if strings.HasPrefix(l, "@@") {
			continue
		}
		if strings.HasPrefix(l, "+") {
			onlyMinus = false
			seenAny = true
		}
		if strings.HasPrefix(l, "-") || strings.HasPrefix(l, " ") {
			seenAny = true
		}
	}
	return inFile && seenAny && onlyMinus
}

// validateHunkHeaders 确保每个 @@ hunk 头都包含行号/行数区间：@@ -n[,m] +n[,m] @@
func validateHunkHeaders(s string) error {
	// 允许的最小形式：@@ -12 +34 @@（count 可省略），或 @@ -12,3 +34,5 @@
	reOK := regexp.MustCompile(`^@@\s+-\d+(?:,\d+)?\s+\+\d+(?:,\d+)?\s+@@(?:\s.*)?$`)

	var bad []string
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		// 只检查以 @@ 开头的行
		if !strings.HasPrefix(l, "@@") {
			continue
		}
		if reOK.MatchString(l) {
			continue
		}
		// 记录出问题的行（1-based 行号）
		bad = append(bad, fmt.Sprintf("%d: %s", i+1, l))
	}

	if len(bad) == 0 {
		return nil
	}

	// 给出修复提示
	var b strings.Builder
	b.WriteString("以下 hunk 头缺少行号区间（示例应为：@@ -1,3 +1,4 @@）：\n")
	for _, x := range bad {
		b.WriteString(" - ")
		b.WriteString(x)
		b.WriteString("\n")
	}
	b.WriteString("请在生成或手写补丁时，保证每个 hunk 头都有 -n[,m] 和 +n[,m]。建议用 `git diff --no-color --binary` 导出补丁以避免该问题。")
	return errors.New(b.String())
}
