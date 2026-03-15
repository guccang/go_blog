package main

import (
	"log"
	"os"
	"path/filepath"
)

const maxPromptFileChars = 20000

// loadWorkspaceFile 从工作区目录加载提示文件，文件不存在则返回 fallback（向后兼容）
func loadWorkspaceFile(workspaceDir, filename, fallback string) string {
	path := filepath.Join(workspaceDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}

	content := string(data)
	if len(content) > maxPromptFileChars {
		log.Printf("[Workspace] 警告: %s 超过 %d 字符，已截断 (原始 %d)", filename, maxPromptFileChars, len(content))
		content = content[:maxPromptFileChars]
	} else {
		log.Printf("[Workspace] 加载 %s (%d 字符)", filename, len(content))
	}

	return content
}
