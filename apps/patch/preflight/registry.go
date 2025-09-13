package preflight

import (
	"path/filepath"
	"strings"
)

type Logf func(string, ...any)

// Runner 预检器：对指定文件进行“自动修复或报错”。
// 约定：若修改了文件，返回 (changed=true, err=nil)
type Runner interface {
	Name() string
	Match(path string) bool
	Run(repo, rel string, logf Logf) (changed bool, err error)
}

// 内置 runner 列表（按顺序匹配）
var runners []Runner

func Register(r Runner) { runners = append(runners, r) }

func RunAll(repo string, files []string, logf Logf) (bool, error) {
	anyChanged := false
	for _, f := range files {
		rel := strings.TrimSpace(f)
		if rel == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(rel))
		_ = ext // 备用：必要时可按扩展提前快速过滤

		var matched bool
		for _, r := range runners {
			if r.Match(rel) {
				matched = true
				changed, err := r.Run(repo, rel, logf)
				if err != nil {
					return false, err
				}
				if changed {
					anyChanged = true
				}
				break
			}
		}
		if !matched {
			// 没有适配器就跳过：保持宽容
			continue
		}
	}
	return anyChanged, nil
}
// Lookup 根据文件路径找到第一个匹配的 Runner
func Lookup(path string) Runner {
	for _, r := range runners {
		if r.Match(path) {
			return r
		}
	}
	return nil
}

