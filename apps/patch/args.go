package main

import (
	"strconv"
	"strings"
)

func argBool(m map[string]string, key string, def bool) bool {
	if v, ok := m[key]; ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return def
}
func argInt(m map[string]string, key string, def int) int {
	if v, ok := m[key]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}
func argStr(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}