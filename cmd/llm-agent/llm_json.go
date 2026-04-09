package main

import "strings"

// cleanLLMJSON 清理 LLM 返回的 JSON 响应，移除 think 标签和 markdown 代码块标记。
func cleanLLMJSON(s string) string {
	s = strings.TrimSpace(s)

	if idx := strings.Index(s, "<think>"); idx >= 0 {
		if endIdx := strings.Index(s[idx:], "</think>"); endIdx >= 0 {
			s = s[:idx] + s[idx+endIdx+8:]
		}
	}
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		} else {
			s = strings.TrimPrefix(s, "```json")
			s = strings.TrimPrefix(s, "```JSON")
			s = strings.TrimPrefix(s, "```")
		}
	}

	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	s = strings.TrimSpace(s)

	if len(s) > 0 && s[0] != '{' {
		start := strings.Index(s, "{")
		end := strings.LastIndex(s, "}")
		if start >= 0 && end > start {
			s = s[start : end+1]
		}
	}

	return strings.TrimSpace(s)
}
