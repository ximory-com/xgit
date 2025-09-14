// run_preflight.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xgit/apps/patch/preflight"
)

// 预检：对 files 中的每个文件选择合适的 Runner 并执行
func preflightRun(repo string, files []string, logger *DualLogger) error {
	logf := func(format string, a ...any) {
		if logger != nil {
			logger.Log(format, a...)
		}
	}
	for _, f := range files {
		rel := strings.TrimSpace(f)
		if rel == "" {
			continue
		}
		abs := filepath.Join(repo, rel)

		// 删除后的文件不做预检（影子里不存在即判为删除）
		if _, err := os.Stat(abs); err != nil && os.IsNotExist(err) {
			logf("ℹ️ 跳过预检（文件已删除）：%s", rel)
			continue
		}

		lang := preflight.DetectLangByExt(rel)
		if lang == "" {
			lang = "unknown"
		}
		logf("🧪 预检 %s (%s)", rel, lang)

		if r := preflight.Lookup(rel); r != nil {
			changed, err := r.Run(repo, rel, logf)
			if err != nil {
				return fmt.Errorf("预检失败 %s: %w", rel, err)
			}
			if changed {
				logf("🛠️ 预检已修改 %s", rel)
			} else {
				logf("✔ 预检通过，无需修改：%s", rel)
			}
		} else {
			logf("ℹ️ 无匹配的预检器：%s", rel)
		}
	}
	return nil
}