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
// v3 流程：在真实仓库应用前，先在“影子 worktree”应用原始补丁 -> 影子里跑预检（含 gofmt 与末尾仅一换行修复）
// -> 用影子导出“规范化补丁” -> 再按策略集应用到真实仓库。
// 这样把 go 文件末尾换行/gofmt 问题在影子阶段一次性修好，降低 corrupt/预检不通过。

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
	// 校验每个 hunk 头是否带 -n,m +n,m，避免 “patch with only garbage”
	if err := validateHunkHeaders(diffText); err != nil {
		return fmt.Errorf("git.diff: 无效 hunk 头：%w", err)
	}

	// 路径/新增删除特征（决定是否启用 3-way）
	paths, hasDevNull, hasNewMode, hasDelMode := parseDiffPaths(diffText)

	// 删除校验
	if hasDelMode {
		for _, p := range paths.aPaths {
			if p != "/dev/null" && !isTracked(repo, p) {
				return fmt.Errorf("git.diff: 删除失败，文件 %s 未在 Git 管理范围", p)
			}
		}
	}

	// 改名校验
	rFrom, _ := parseRenamePairs(diffText)
	for _, p := range rFrom {
		if !isTracked(repo, p) {
			return fmt.Errorf("git.diff: 改名失败，源文件 %s 未在 Git 管理范围", p)
		}
	}	
	allow3 := !(hasDevNull || hasNewMode || hasDelMode)
	isDelete := detectDelete(diffText) // 👈 新增：识别是否为“删除文件”场景

	keep := os.Getenv("XGIT_KEEP_PATCH") == "1"
	show := os.Getenv("XGIT_SHOW_PATCH") == "1"

	// 2) 把“原始补丁”写入临时文件（用于影子仓库尝试）
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

	// 影子里先做意向 add（提升 --index 命中率），并确保需要的父目录/前置文件存在
	intentAddFromDiff(shadow, diffText, logger)
	syncPrereqsToShadow(repo, shadow, diffText, logger)

	log("📄 [影子] 正在应用原始补丁：%s", filepath.Base(rawPatch))
	if err := applyWithStrategies(shadow, rawPatch, allow3, logger, isDelete); err != nil {
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
	nIsDelete := detectDelete(normText) 

	normPatch, err := writeTempPatch(repo, normText, keep)
	if err != nil {
		return err
	}

	// 4) 在真实仓库应用“规范化补丁”
	log("📄 git.diff 正在应用补丁：%s", filepath.Base(normPatch))
	if err := applyWithStrategies(repo, normPatch, nAllow3, logger, nIsDelete); err != nil { // 👈 传 isDelete
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

// intentAddFromDiff：只对 b/ 路径和 rename to/from 做 add -N。
// 不再对 a/ 路径 add -N，避免纯删除时把旧路径标成“意向添加”。
func intentAddFromDiff(repo string, diffText string, logger DualLogger) {
	paths, _, _, _ := parseDiffPaths(diffText)

	addN := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || p == "/dev/null" || strings.HasSuffix(p, "/") {
			return
		}
		_, _ = runGit(repo, logger, "add", "-N", p)
	}

	// 去重
	seen := make(map[string]struct{})
	maybeAdd := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		addN(p)
	}

	// 1) 只处理 b/…（新增/修改/重命名后的新路径）
	for _, p := range paths.bPaths {
		maybeAdd(p)
	}

	// 2) 解析 rename from/to 并处理（两端都处理更稳妥）
	rFrom, rTo := parseRenamePairs(diffText) // 确保你已实现它
	for _, p := range rFrom {
		maybeAdd(p)
	}
	for _, p := range rTo {
		maybeAdd(p)
	}
}

// 解析 "rename from ..." / "rename to ..."
func parseRenamePairs(s string) (from []string, to []string) {
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "rename from ") {
			from = append(from, strings.TrimSpace(strings.TrimPrefix(t, "rename from ")))
		} else if strings.HasPrefix(t, "rename to ") {
			to = append(to, strings.TrimSpace(strings.TrimPrefix(t, "rename to ")))
		}
	}
	return
}
// syncPrereqsToShadow：确保影子里存在需要的父目录/前置文件（主要为重命名/修改建立路径）
func syncPrereqsToShadow(realRepo, shadow string, diffText string, logger DualLogger) {
	pp, _, _, _ := parseDiffPaths(diffText)
	mkParents := func(rel string) {
		if rel == "/dev/null" || strings.TrimSpace(rel) == "" {
			return
		}
		_ = os.MkdirAll(filepath.Join(shadow, filepath.Dir(rel)), 0o755)
	}
	for _, p := range pp.aPaths {
		mkParents(p)
	}
	for _, p := range pp.bPaths {
		mkParents(p)
	}

	// 若真实仓库有对应文件而影子缺失，则拷过去（为 rename/modify 提供基线）
	copyIfExists := func(rel string) {
		if rel == "/dev/null" || strings.TrimSpace(rel) == "" {
			return
		}
		src := filepath.Join(realRepo, rel)
		dst := filepath.Join(shadow, rel)
		if _, err := os.Stat(src); err == nil {
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				if data, e := os.ReadFile(src); e == nil {
					_ = os.MkdirAll(filepath.Dir(dst), 0o755)
					_ = os.WriteFile(dst, data, 0o644)
				}
			}
		}
	}
	for _, p := range pp.aPaths {
		copyIfExists(p)
	}
	for _, p := range pp.bPaths {
		copyIfExists(p)
	}
}

// collectChangedFiles 用 git status --porcelain 收集变更路径
func collectChangedFiles(repo string, logger DualLogger) ([]string, error) {
	out, err := runGit(repo, logger, "status", "--porcelain", "-uall")
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, raw := range strings.Split(strings.TrimSpace(out), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || len(line) <= 3 {
			continue
		}
		payload := strings.TrimSpace(line[3:]) // 跳过 XY 和空格

		// 处理 rename： "R  old -> new" / "R100 old -> new"
		if idx := strings.Index(payload, "->"); idx >= 0 {
			payload = strings.TrimSpace(payload[idx+2:])
		}

		// 忽略明显目录标记（porcelain 可能是 "?? dir/"）
		if strings.HasSuffix(payload, "/") {
			continue
		}

		full := filepath.Join(repo, payload)
		if fi, err := os.Stat(full); err == nil && fi.IsDir() {
			continue
		}
		if isTempPatch(payload) {
			continue
		}

		changed = append(changed, payload)
	}
	return changed, nil
}

// runPreflights 跑注册的预检器（含 goFmtRunner -> 末尾仅一换行 + gofmt）
func runPreflights(repo string, files []string, diffText string, logger DualLogger) error {
	log := func(f string, a ...any) {
		if logger != nil {
			logger.Log(f, a...)
		}
	}
	for _, rel := range files {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		// 跳过影子中已删除的文件
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil && os.IsNotExist(err) {
			continue
		}
		// 删除补丁的 go 文件跳过预检（避免对已删除目标做 gofmt）
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
func applyWithStrategies(repo string, patchPath string, allow3 bool, logger DualLogger, isDelete bool) error {
	strategies := buildStrategies(allow3, isDelete)
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

// buildStrategies 根据是否允许 3-way 和是否删除场景 返回尝试序列参数（不含 "apply" 与补丁路径）
func buildStrategies(allow3Way bool, isDelete bool) [][]string {
	// 删除文件：不少仓库 index 里没有该文件，先走“纯 apply”避免 `does not exist in index`
	if isDelete {
		return [][]string{
			{"--whitespace=nowarn"},           // 先不碰 index，最宽松
			{"--index", "--whitespace=nowarn"}, // 需要时再带 index
		}
	}

	// 常规/新增/改名/修改
	if allow3Way {
		return [][]string{
			{"--index", "--3way", "--whitespace=nowarn"},
			{"--3way", "--whitespace=nowarn"},
			{"--index", "--whitespace=nowarn"},
			{"--whitespace=nowarn"},
		}
	}
	// 新增文件（/dev/null 或 new file mode）场景：跳过 3-way
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

// writeTempPatch 把文本写到系统临时目录的 .patch 文件（避免被仓库 diff 捕获）
func writeTempPatch(repo string, text string, keep bool) (string, error) {
	f, err := os.CreateTemp("", ".xgit_*.patch") // 不要放 repo
	if err != nil {
		return "", err
	}
	path := f.Name()
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
	// 是否保留文件由调用方环境变量 XGIT_KEEP_PATCH 控制；这里不删除
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
	return fmt.Errorf("%v%s", err, tail.String())
}

// ================== 影子/预检辅助 ==================

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
	b.WriteString("请在生成或手写补丁时，保证每个 hunk 头都有 -n[,m] 和 +n[,m]。建议用 `git diff --no-color --binary` 导出补丁。")
	return errors.New(b.String())
}

func detectDelete(s string) bool {
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		// 明确的删除标记
		if strings.HasPrefix(t, "deleted file mode ") {
			return true
		}
		// 经典删除形态：+++ /dev/null
		if strings.HasPrefix(t, "+++ ") && strings.HasSuffix(t, "/dev/null") {
			return true
		}
		// 也兼容 --- a/xxx +++ /dev/null 的组合
		if strings.HasPrefix(t, "--- ") && strings.HasSuffix(t, "/dev/null") {
			return true
		}
	}
	return false
}

func isTracked(repo, path string) bool {
    _, err := runGit(repo, nil, "ls-files", "--error-unmatch", path)
    return err == nil
}

func isTempPatch(rel string) bool {
	base := filepath.Base(rel)
	return strings.HasPrefix(base, ".xgit_") && strings.HasSuffix(base, ".patch")
}