package main

// XGIT:BEGIN IMPORTS
// 说明：仓库映射 .repos 解析（支持 default = name）
import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)
// XGIT:END IMPORTS

// XGIT:BEGIN REPOS
// 说明：读取 patchDir/.repos -> (name->absPath, defaultName)
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
		// name /abs/path（或带空格的路径）
		sp := strings.Fields(line)
		if len(sp) >= 2 {
			name := sp[0]
			path := strings.Join(sp[1:], " ")
			m[name] = path
		}
	}
	return m, def
}
// XGIT:END REPOS
// XGIT:BEGIN REPOS_HEADER
package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)
// XGIT:END REPOS_HEADER
