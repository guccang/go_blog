package main

import "strings"

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func base64DecodedLen(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	padding := 0
	if strings.HasSuffix(s, "==") {
		padding = 2
	} else if strings.HasSuffix(s, "=") {
		padding = 1
	}
	return len(s)*3/4 - padding
}

func firstStringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
