package patch

// 读取补丁目录下 .repos 映射，并解析补丁头里的 repo: 字段。
// 导出：LoadRepos, HeaderRepoName

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadRepos 解析 patchDir/.repos，返回 (name->path 映射, defaultName)
func LoadRepos(patchDir string) (map[string]string, string) {
	m := map[string]string{}
	def := ""
	f, err := os.Open(filepath.Join(patchDir, ".repos"))
	if err != nil {
		return m, def
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 兼容 "default = name" 以及 "name /abs/path"
		if strings.Contains(line, "=") {
			k, v, _ := strings.Cut(line, "=")
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if strings.EqualFold(k, "default") {
				def = v
			} else if v != "" {
				m[k] = v
			}
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			path := strings.Join(parts[1:], " ")
			m[name] = path
		}
	}
	return m, def
}

// HeaderRepoName 扫描补丁文件头部，读取 repo: <name|/abs/path>
func HeaderRepoName(patchFile string) string {
	f, err := os.Open(patchFile)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "repo:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "repo:"))
		}
		if strings.HasPrefix(line, "===") {
			break
		}
	}
	return ""
}
