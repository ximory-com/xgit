package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xgit/apps/patch/preflight"
)

// ApplyOnce：增加 patchFile 参数用于从文件头读取 repo: 兜底（拿不到可传 ""）
func ApplyOnce(logger *DualLogger, repo string, patch *Patch, patchFile string) {
	// 0) 解析真实仓库路径（优先 Patch.Repo，其次补丁头 repo:，最后 .repos 的 default）
	patchDir := "."
	if strings.TrimSpace(patchFile) != "" {
		patchDir = filepath.Dir(patchFile)
	}
	selectedRepo, err := resolveRepoFromPatch(patchDir, patch, patchFile)
	if err != nil {
		if logger != nil {
			logger.Log("❌ 仓库解析失败：%v", err)
		}
		return
	}
	repo = selectedRepo

	// 1) 打开/截断 patch.log
	logPath := filepath.Join(repo, "patch.log")
	f, ferr := os.Create(logPath) // 截断旧内容
	if ferr != nil && logger != nil {
		logger.Log("⚠️ 无法写入 patch.log：%v（将仅输出到控制台）", ferr)
	}
	writeFile := func(s string) {
		if f != nil {
			_, _ = f.WriteString(s)
			if !strings.HasSuffix(s, "\n") {
				_, _ = f.WriteString("\n")
			}
		}
	}
	log := func(format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		if logger != nil {
			logger.Log("%s", msg)
		}
		writeFile(msg)
	}
	logf := func(format string, a ...any) { log(format, a...) }
	defer func() { if f != nil { _ = f.Close() } }()

	// 2) 事务阶段
	err = WithGitTxn(repo, logf, func() error {
		// 先应用所有指令
		for i, op := range patch.Ops {
			tag := fmt.Sprintf("%s #%d", op.Cmd, i+1)
			if e := applyOp(repo, op, logger); e != nil {
				log("❌ %s 失败：%v", tag, e)
				return e
			}
			log("✅ %s 成功", tag)
		}

		// 再收集本次改动并在“真实仓库”跑预检（失败则回滚）
		changed, _ := collectChangedFiles(repo)
		if len(changed) > 0 {
			// 过滤“新增文件”，避免预检的兜底模板覆盖新文件真实内容
			changedForPreflight := filterOutNewFiles(repo, changed)

			if len(changedForPreflight) == 0 {
				logf("ℹ️ 预检：有文件变更，但全是新增文件（跳过预检写回，仅后续提交）。")
				return nil
			}

			logf("🧪 预检（真实仓库）：%d 个文件", len(changedForPreflight))
			if err := preflightRun(repo, changedForPreflight, logger); err != nil {
				logf("❌ 预检失败：%v", err)
				return err
			}
			logf("✅ 预检通过")
		} else {
			logf("ℹ️ 预检：无文件变更")
		}

		return nil
	})
	if err != nil {
		return // 事务内部已回滚并记录日志
	}

	// 3) 统一 stage/commit/push
	_ = runCmd("git", "-C", repo, "add", "-A")

	names, _ := runCmdOut("git", "-C", repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(names) == "" {
		log("ℹ️ 无改动需要提交。")
		log("✅ 本次补丁完成")
		return
	}

	commit := strings.TrimSpace(patch.CommitMsg)
	if commit == "" {
		commit = "chore: apply patch"
	}
	author := strings.TrimSpace(patch.Author)
	if author == "" {
		author = "XGit Bot <bot@xgit.local>"
	}

	log("ℹ️ 提交说明：%s", commit)
	log("ℹ️ 提交作者：%s", author)

	_ = runCmd("git", "-C", repo, "commit", "--author", author, "-m", commit)
	log("✅ 已提交：%s", commit)

	log("🚀 正在推送（origin HEAD）…")
	if _, err := runCmdOut("git", "-C", repo, "push", "origin", "HEAD"); err != nil {
		log("❌ 推送失败：%v", err)
	} else {
		log("🚀 推送完成")
	}
	log("✅ 本次补丁完成")
}

// 过滤掉“新增（A/??）文件”用于预检：避免预检写回覆盖新文件真实内容
func filterOutNewFiles(repo string, files []string) []string {
	out := make([]string, 0, len(files))
	for _, rel := range files {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		if isAddedInRepo(repo, rel) {
			// 新增文件：不进入预检写回流程（仅后续统一提交）
			continue
		}
		out = append(out, rel)
	}
	return out
}

// isAddedInRepo 返回该路径在 git status 中是否为新增（A/??）
func isAddedInRepo(repo, rel string) bool {
	line, _ := runCmdOut("git", "-C", repo, "status", "--porcelain", "--", rel)
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	// Porcelain: XY<space>path
	// 新增常见： "A  "（索引新增） 或 "?? "（未跟踪）
	if len(line) >= 2 {
		x, y := line[0], line[1]
		if x == 'A' || x == '?' || y == 'A' || y == '?' {
			return true
		}
	}
	// 兼容多行输出时的第一行判断
	if strings.HasPrefix(line, "A ") || strings.HasPrefix(line, "?? ") {
		return true
	}
	return false
}

// resolveRepoFromPatch 依据补丁头与 .repos 解析真实仓库路径。
// 规则：
//   - 若 header/patch.Repo 指定了 repo 逻辑名 → 在 .repos 中查同名 key，取其 value 作为真实路径
//   - 若未指定 → 使用 .repos 中 default= 的值作为“逻辑名”再去查路径
//   - 不接受绝对路径（header 里写绝对路径直接报错）
//
// patchDir 传补丁文件所在目录（通常是工作目录）；
// patchFile 若你拿得到补丁文件路径可传入，用来兜底读取 `repo:` 行，没有就传 ""。
func resolveRepoFromPatch(patchDir string, patch *Patch, patchFile string) (string, error) {
	m, defName := LoadRepos(patchDir)

	// 1) 取逻辑名优先级：Patch.Repo > HeaderRepoName(patchFile) > defaultName
	var logic string
	if patch != nil && strings.TrimSpace(patch.Repo) != "" {
		logic = strings.TrimSpace(patch.Repo)
	} else if patchFile != "" {
		if n := strings.TrimSpace(HeaderRepoName(patchFile)); n != "" {
			logic = n
		}
	}
	if logic == "" {
		logic = defName // defName 是“默认逻辑名”，不是路径
	}
	if logic == "" {
		return "", fmt.Errorf("未指定 repo，且 .repos 中没有 default= 设置")
	}

	// 2) 拒绝绝对路径（只接受逻辑名）
	if filepath.IsAbs(logic) || strings.Contains(logic, string(os.PathSeparator)) {
		return "", fmt.Errorf("repo 只接受逻辑名，禁止绝对/相对路径：%q", logic)
	}

	// 3) 逻辑名 → 真实路径
	path, ok := m[logic]
	if !ok {
		return "", fmt.Errorf(".repos 未找到逻辑名 %q 的映射", logic)
	}
	// 归一为绝对路径，避免后续 git -C 相对上下文混乱
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("解析仓库路径失败：%w", err)
		}
		path = abs
	}
	return path, nil
}

// 仅为编译引用，确保预检包被链接（如你已在别处用到可删）
var _ = preflight.Register