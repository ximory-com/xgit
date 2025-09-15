// apps/patch/gitops/diff.go
package gitops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
// - 清洗补丁文本 -> intent add -N -> 预检(--check --recount) -> git apply（按 diff 类型选择策略）。
// - 新增/删除/重命名：跳过 3-way；纯修改：优先 3-way；所有策略统一 --recount。
// - 失败时打印 git 输出与出错行上下文（±20），并报告 .rej（如有）。
// - 成功后逐个文件打印：新建/删除/修改/改名 <路径/对>；
//   对“新建文件”执行：补丁 + 行数 == 工作区实际行数 的强校验。

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

	// 0) 处理前统计
	countLF := func(s string) int { return strings.Count(s, "\n") }
	countCR := func(s string) int { return strings.Count(s, "\r") }
	hasFence := func(s string) (lead, tail bool) {
		lines := strings.Split(s, "\n")
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
			lead = true
		}
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			tail = true
		}
		return
	}
	sha8 := func(s string) string {
		h := sha256.Sum256([]byte(s))
		return hex.EncodeToString(h[:])[:8]
	}

	orig := diffText
	origLF, origCR := countLF(orig), countCR(orig)
	leadFence, tailFence := hasFence(orig)
	origHash := sha8(orig)
	log("📝 处理前 diff: %d 字节, %d 行(\\n), %d 个\\r, fence[首=%v,尾=%v], hash=%s",
		len(orig), origLF, origCR, leadFence, tailFence, origHash)

	// 1) 预处理并基本校验
	diffText = sanitizeDiff(diffText)

	newLF, newCR := countLF(diffText), countCR(diffText)
	newHash := sha8(diffText)
	log("📝 处理后 diff: %d 字节, %d 行(\\n), %d 个\\r, hash=%s",
		len(diffText), newLF, newCR, newHash)

	// 清洗后仍需“像 diff”
	if !looksLikeDiff(diffText) {
		if looksLikeDiff(orig) {
			return fmt.Errorf("git.diff: 清洗后不再像有效 diff（前后hash=%s→%s）", origHash, newHash)
		}
		return errors.New("git.diff: 输入不是有效的 diff（缺少 diff 头）")
	}

	// 围栏行数的合理性（只允许去掉 0/1/2 行围栏）
	if delta := origLF - newLF; delta < 0 || delta > 2 {
		log("⚠️ 清洗后行数变化异常：origLF=%d, newLF=%d（可能非围栏导致的行丢失）", origLF, newLF)
	}

	// 2) 写临时补丁（写后回读校验：hash+行数）
	patchPath, cleanup, err := writeTempPatch(repo, diffText)
	if err != nil {
		log("❌ git.diff 临时文件失败：%v", err)
		return err
	}
	defer cleanup()
	log("📄 git.diff 正在应用补丁：%s", filepath.Base(patchPath))

	// 3) 针对新增/重命名做 intent add -N
	intentAddFromDiff(repo, diffText, logger)

	// 3.2) 文件系统预检：新增/修改/删除/改名的存在性约束
	if err := fsPreflight(repo, diffText, logger); err != nil {
		return err
	}

	// 3.5) 预检：在正式 apply 前先 --check --recount
	if err := preflightCheck(repo, patchPath, logger); err != nil {
		// 若能解析出报错行，打印上下文
		if line := extractPatchErrorLine(err.Error()); line > 0 {
			if ctx := readPatchContext(patchPath, line, 20); ctx != "" {
				log("🧭 预检失败，出错行上下文（±20）：\n%s", ctx)
			}
		}
		return err
	}

	// 4) 选择策略并尝试应用（统一 --recount）
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

		// 4.5) 新建文件强校验：补丁“+行数”应等于工作区实际行数
		for _, p := range adds {
			expect := countPlusLinesForFile(diffText, p)
			if expect <= 0 {
				// 未能统计出 “+” 行数，给出提示但不中断（视为免核对）
				log("ℹ️ 新建 %s：跳过行数校验（未找到 '+' 行）", p)
				continue
			}
			if err := ensureWorktreeLines(repo, p, expect); err != nil {
				return fmt.Errorf("git.diff: 新建文件内容校验失败 %s：%w", p, err)
			}
			log("🔎 校验通过：%s 行数=%d", p, expect)
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

// 根据 diff 类型选择策略序列（统一加 --recount）
func buildStrategiesFromDiff(s string) [][]string {
	hasAddOrDelete, hasRename := analyzeDiffKinds(s)
	// 重命名/新增/删除：跳过 3-way
	if hasAddOrDelete || hasRename {
		return [][]string{
			{"--recount", "--whitespace=nowarn"},            // 直贴
			{"--index", "--recount", "--whitespace=nowarn"}, // 如需更新 index（存在时生效）
		}
	}
	// 纯修改：优先 3way 提高成功率
	return [][]string{
		{"--index", "--3way", "--recount", "--whitespace=nowarn"},
		{"--3way", "--recount", "--whitespace=nowarn"},
		{"--index", "--recount", "--whitespace=nowarn"},
		{"--recount", "--whitespace=nowarn"},
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
// 1) 剥掉首尾 ```...``` 围栏行（支持 ```diff / ```patch）
// 2) 归一化换行: \r\n / \r -> \n
// 3) 确保末尾有且仅有一个 '\n'
// 绝不 TrimSpace、绝不改动任何以 '+', '-', ' ' 开头的 hunk 行
func sanitizeDiff(s string) string {
	// 剥离 Markdown 围栏（保留正文原样）
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		// 去掉首行围栏（``` 或 ```diff/patch）
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
			lines = lines[1:]
		}
		// 若最后一行是围栏，也去掉
		if len(lines) > 0 {
			last := strings.TrimSpace(lines[len(lines)-1])
			if strings.HasPrefix(last, "```") && last == "```" {
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
// 写入后会回读校验 hash + 行数，防止写盘污染导致内容被截断
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

	// 预计算 hash 与行数
	wantHash, wantLines := hashAndNLines(text)

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

	// 回读校验
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	gotHash, gotLines := hashAndNLines(string(data))
	if gotHash != wantHash || gotLines != wantLines {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("patch 回读校验失败：hash %s→%s, 行数 %d→%d", wantHash[:8], gotHash[:8], wantLines, gotLines)
	}

	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}

// NEW: 计算 sha256 与 \n 行数（便于写盘后回读比对）
func hashAndNLines(s string) (sum string, n int) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:]), n
}

// NEW: 预检 – 在正式 apply 前先 --check --recount
func preflightCheck(repo, patchPath string, logger DualLogger) error {
	_, err := runGit(repo, logger, "apply", "--check", "--recount", "--verbose", patchPath)
	if err != nil {
		return fmt.Errorf("git apply --check 失败：%w", err)
	}
	return nil
}

// NEW: 统计某个 b/<path> 文件在补丁中的 '+' 行数（不含 '+++')
func countPlusLinesForFile(diffText, repoRelPath string) int {
	var inTarget, inHunk bool
	lines := strings.Split(diffText, "\n")
	plus := 0
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			inTarget, inHunk = false, false
			// diff --git a/xxx b/xxx
			if strings.Contains(ln, " b/"+repoRelPath) {
				inTarget = true
			}
			continue
		}
		if !inTarget {
			continue
		}
		if strings.HasPrefix(ln, "@@ ") {
			inHunk = true
			continue
		}
		if strings.HasPrefix(ln, "diff --git ") {
			inHunk = false
			continue
		}
		if !inHunk {
			continue
		}
		if strings.HasPrefix(ln, "+") && !strings.HasPrefix(ln, "+++") {
			plus++
		}
	}
	return plus
}

// NEW: 读取工作区文件行数并与期望对比；允许“末行无换行”边界
func ensureWorktreeLines(repo, repoRelPath string, expect int) error {
	data, err := os.ReadFile(filepath.Join(repo, repoRelPath))
	if err != nil {
		return err
	}
	got := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		got++
	}
	if got != expect {
		return fmt.Errorf("行数不一致：期望 %d，实际 %d", expect, got)
	}
	return nil
}

// fsPreflight：对补丁涉及的目标做本地文件系统存在性校验
// 规则：
//  A(新增)  -> 目标文件若存在 => FAIL（不执行）
//  M(修改)  -> 目标文件若不存在 => FAIL（不执行）
//  D(删除)  -> 目标文件若不存在 => FAIL（不执行）
//  R(改名)  -> from 不存在 => FAIL；to 若已存在 => FAIL（避免覆盖）
//
// 注意：这里基于 summarizeDiffFiles(diffText) 的结果；
//       若你的 diff 里有同一文件同补丁先 A 再 M 之类复杂操作，建议改为按块解析。
//       常规新建/修改/删除/改名场景，这个足够稳。
func fsPreflight(repo, diffText string, logger DualLogger) error {
    log := func(format string, a ...any) {
        if logger != nil {
            logger.Log(format, a...)
        }
    }

    adds, dels, mods, renames := summarizeDiffFiles(diffText)

    type viol struct{ kind, path, more string }
    var conflicts []viol

    exists := func(p string) bool {
        st, err := os.Stat(filepath.Join(repo, p))
        return err == nil && !st.IsDir()
    }

    // 新增：目标不得已存在
    for _, p := range adds {
        if exists(p) {
            conflicts = append(conflicts, viol{"A", p, "目标已存在"})
        }
    }

    // 修改：目标必须已存在
    for _, p := range mods {
        if !exists(p) {
            conflicts = append(conflicts, viol{"M", p, "目标不存在"})
        }
    }

    // 删除：目标必须已存在
    for _, p := range dels {
        if !exists(p) {
            conflicts = append(conflicts, viol{"D", p, "目标不存在"})
        }
    }

    // 改名：from 必须存在；to 不得存在（避免覆盖）
    for _, pr := range renames {
        from, to := pr[0], pr[1]
        if !exists(from) {
            conflicts = append(conflicts, viol{"R", from, "rename from 不存在"})
        }
        if exists(to) {
            conflicts = append(conflicts, viol{"R", to, "rename to 已存在"})
        }
    }

    if len(conflicts) == 0 {
        log("🔒 预检：文件存在性通过（A/M/D/R）")
        return nil
    }

    // 打印冲突清单并中止
    var b strings.Builder
    b.WriteString("git.diff: 文件存在性预检失败：\n")
    for _, c := range conflicts {
        fmt.Fprintf(&b, " - [%s] %s：%s\n", c.kind, c.path, c.more)
    }
    log("❌ %s", b.String())
    return errors.New(b.String())
}