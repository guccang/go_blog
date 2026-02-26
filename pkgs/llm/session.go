package llm

import (
	"context"
	"control"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"strings"
	"time"
)

// ============================================================================
// 会话记忆持久化 - 参考 Anthropic 长期运行 Agent 的 progress file 方案
// 每轮对话结束后保存结构化摘要，下次对话时注入到 System Prompt
// ============================================================================

// SessionProgress 结构化会话进度（类似 claude-progress.txt）
type SessionProgress struct {
	Timestamp   string   `json:"timestamp"`
	Query       string   `json:"query"`
	Summary     string   `json:"summary"`
	ToolsCalled []string `json:"tools_called"`
	NextSteps   string   `json:"next_steps,omitempty"`
}

// SessionProgressFile 每日会话进度文件
type SessionProgressFile struct {
	Date     string            `json:"date"`
	Sessions []SessionProgress `json:"sessions"`
}

const sessionProgressPrefix = "session_progress_"

// SaveSessionProgress 保存会话进度
// 在每轮对话结束后调用，通过 LLM 生成摘要并追加到当日会话进度博客
func SaveSessionProgress(account, query, response string, toolsCalled []string) {
	if query == "" || response == "" {
		return
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	blogTitle := fmt.Sprintf("%s%s", sessionProgressPrefix, dateStr)

	// 用 LLM 生成简短摘要（避免保存完整对话）
	summary := generateSessionSummary(account, query, response)

	progress := SessionProgress{
		Timestamp:   now.Format("15:04:05"),
		Query:       truncateForSession(query, 100),
		Summary:     summary,
		ToolsCalled: toolsCalled,
	}

	// 加载已有进度
	progressFile := loadSessionProgressFile(account, blogTitle)
	progressFile.Date = dateStr
	progressFile.Sessions = append(progressFile.Sessions, progress)

	// 保存到博客
	saveSessionProgressFile(account, blogTitle, progressFile)
	log.InfoF(log.ModuleLLM, "Session progress saved: %s (%d sessions)", blogTitle, len(progressFile.Sessions))
}

// LoadRecentSessions 加载最近的会话进度
// 用于注入到 System Prompt 中，让 AI 了解之前的对话内容
func LoadRecentSessions(account string, maxSessions int) []SessionProgress {
	today := time.Now()
	var allSessions []SessionProgress

	// 尝试加载今天和昨天的会话进度
	for i := 0; i < 2; i++ {
		date := today.AddDate(0, 0, -i).Format("2006-01-02")
		blogTitle := fmt.Sprintf("%s%s", sessionProgressPrefix, date)
		progressFile := loadSessionProgressFile(account, blogTitle)
		allSessions = append(allSessions, progressFile.Sessions...)
	}

	// 取最近 N 条
	if len(allSessions) > maxSessions {
		allSessions = allSessions[len(allSessions)-maxSessions:]
	}

	return allSessions
}

// FormatSessionHistory 格式化会话历史为可读文本
func FormatSessionHistory(sessions []SessionProgress) string {
	if len(sessions) == 0 {
		return ""
	}

	var parts []string
	for _, s := range sessions {
		entry := fmt.Sprintf("[%s] 问:%s → %s", s.Timestamp, s.Query, s.Summary)
		if len(s.ToolsCalled) > 0 {
			entry += fmt.Sprintf(" (工具:%s)", strings.Join(s.ToolsCalled, ","))
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, "\n")
}

// ============================================================================
// 内部实现
// ============================================================================

// generateSessionSummary 用 LLM 生成会话摘要
func generateSessionSummary(account, query, response string) string {
	summaryPrompt := fmt.Sprintf(`请用一句话(不超过50字)总结这轮对话的核心内容,只返回总结,不要其他文字:
用户问: %s
AI答: %s`, truncateForSession(query, 200), truncateForSession(response, 500))

	messages := []Message{
		{Role: "user", Content: summaryPrompt},
	}

	summary, err := SendSyncLLMRequestNoTools(context.Background(), messages, account)
	if err != nil {
		log.WarnF(log.ModuleLLM, "Failed to generate session summary: %v", err)
		// fallback: 截取 response 前 50 字
		return truncateForSession(response, 50)
	}

	return strings.TrimSpace(summary)
}

// loadSessionProgressFile 从博客加载会话进度文件
func loadSessionProgressFile(account, blogTitle string) SessionProgressFile {
	var progressFile SessionProgressFile

	blog := control.GetBlog(account, blogTitle)
	if blog == nil {
		return progressFile
	}

	// 尝试从博客内容解析 JSON
	content := strings.TrimSpace(blog.Content)
	if err := json.Unmarshal([]byte(content), &progressFile); err != nil {
		log.WarnF(log.ModuleLLM, "Failed to parse session progress: %v", err)
	}

	return progressFile
}

// saveSessionProgressFile 保存会话进度文件到博客
func saveSessionProgressFile(account, blogTitle string, progressFile SessionProgressFile) {
	jsonBytes, err := json.MarshalIndent(progressFile, "", "  ")
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Failed to marshal session progress: %v", err)
		return
	}

	content := string(jsonBytes)

	existingBlog := control.GetBlog(account, blogTitle)
	if existingBlog != nil {
		blogData := &module.UploadedBlogData{
			Title:    blogTitle,
			Content:  content,
			Tags:     existingBlog.Tags,
			AuthType: existingBlog.AuthType,
			Encrypt:  existingBlog.Encrypt,
		}
		control.ModifyBlog(account, blogData)
	} else {
		blogData := &module.UploadedBlogData{
			Title:    blogTitle,
			Content:  content,
			Tags:     "AI会话|自动生成",
			AuthType: module.EAuthType_diary,
		}
		control.AddBlog(account, blogData)
	}
}

// truncateForSession 截断文本（用于会话进度保存）
func truncateForSession(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
