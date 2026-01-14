package llm

import (
	"control"
	"fmt"
	"module"
	log "mylog"
	"time"
)

// SaveLLMResponseToDiary saves full LLM response to diary
func SaveLLMResponseToDiary(account, userQuery, llmResponse string) {
	if userQuery == "" || llmResponse == "" {
		return
	}

	// Get current date in diary format
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	diaryTitle := fmt.Sprintf("AI_assistant_%s", dateStr)

	log.DebugF(log.ModuleLLM, "准备保存LLM响应到日记: %s", diaryTitle)

	// Build new conversation entry content
	newEntry := fmt.Sprintf(`

### 🤖 AI助手对话 (%s)

**用户问题：**
%s

**AI回复：**
%s

---
`, now.Format("15:04:05"), userQuery, llmResponse)

	// Check if today's diary already exists
	existingBlog := control.GetBlog(account, diaryTitle)
	var finalContent string

	if existingBlog != nil {
		// Append to existing diary
		log.DebugF(log.ModuleLLM, "发现已存在的日记，追加内容")
		finalContent = existingBlog.Content + newEntry

		// Modify existing blog
		blogData := &module.UploadedBlogData{
			Title:    diaryTitle,
			Content:  finalContent,
			Tags:     existingBlog.Tags,
			AuthType: existingBlog.AuthType,
			Encrypt:  existingBlog.Encrypt,
		}
		control.ModifyBlog(account, blogData)
		log.InfoF(log.ModuleLLM, "LLM响应已追加到现有日记: %s", diaryTitle)
	} else {
		// Create new diary
		log.DebugF(log.ModuleLLM, "创建新的日记")
		finalContent = fmt.Sprintf(`# %s 日记

*今日开始记录...*%s`, dateStr, newEntry)

		// Create new blog with diary permissions
		blogData := &module.UploadedBlogData{
			Title:    diaryTitle,
			Content:  finalContent,
			Tags:     "日记|AI助手|自动生成",
			AuthType: module.EAuthType_diary, // Use diary permission
		}
		control.AddBlog(account, blogData)
		log.InfoF(log.ModuleLLM, "LLM响应已保存到新日记: %s", diaryTitle)
	}
}

// SaveConversationToBlog saves user's last message (placeholder for conversation saving)
func SaveConversationToBlog(messages []Message) {
	if len(messages) == 0 {
		return
	}

	// Get the user's last message
	var userMessage string
	for _, msg := range messages {
		if msg.Role == "user" {
			userMessage = msg.Content
		}
	}

	if userMessage == "" {
		return
	}

	log.DebugF(log.ModuleLLM, "保存用户问题到对话记录: %s", userMessage)
	// Here we can pre-save user questions, actual LLM response will be handled by SaveLLMResponseToDiary
}
