package main

import (
	"log"
	"os"
	"path/filepath"
)

const maxPromptFileChars = 20000

// GetAccountWorkspace 返回指定账户的 workspace 目录
// 路径: baseDir/users/{account}
func GetAccountWorkspace(baseDir, account string) string {
	return filepath.Join(baseDir, "users", account)
}

// GetSharedSkillsDir 返回共享的 skills 目录
// 路径: baseDir/skills
func GetSharedSkillsDir(baseDir string) string {
	return filepath.Join(baseDir, "skills")
}

// EnsureAccountWorkspace 确保账户 workspace 目录及子目录存在，返回路径
func EnsureAccountWorkspace(baseDir, account string) string {
	wsPath := GetAccountWorkspace(baseDir, account)

	// 创建账户 workspace 根目录
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		log.Printf("[Workspace] 创建账户目录失败: %s - %v", wsPath, err)
	}

	// 创建必要的子目录
	subDirs := []string{"memory", "chat_sessions"}
	for _, subDir := range subDirs {
		subPath := filepath.Join(wsPath, subDir)
		if err := os.MkdirAll(subPath, 0755); err != nil {
			log.Printf("[Workspace] 创建子目录失败: %s - %v", subPath, err)
		}
	}

	return wsPath
}

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

// loadAccountWorkspaceFile 加载账户私有的提示文件，文件不存在则 fallback 到全局 workspace
func loadAccountWorkspaceFile(baseDir, account, filename, fallback string) string {
	accountWS := GetAccountWorkspace(baseDir, account)
	path := filepath.Join(accountWS, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback 到全局 workspace
			globalPath := filepath.Join(baseDir, filename)
			if globalData, globalErr := os.ReadFile(globalPath); globalErr == nil {
				log.Printf("[Workspace] 加载 %s (全局 workspace, 账户 %s 不存在私有文件)", filename, account)
				content := string(globalData)
				if len(content) > maxPromptFileChars {
					content = content[:maxPromptFileChars]
				}
				return content
			}
		}
		// 账户 workspace 不存在或文件不存在，返回 fallback
		return fallback
	}

	content := string(data)
	if len(content) > maxPromptFileChars {
		log.Printf("[Workspace] 警告: %s (账户 %s) 超过 %d 字符，已截断", filename, account, maxPromptFileChars)
		content = content[:maxPromptFileChars]
	} else {
		log.Printf("[Workspace] 加载 %s (账户 %s, %d 字符)", filename, account, len(content))
	}

	return content
}
