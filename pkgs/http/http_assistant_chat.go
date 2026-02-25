package http

import (
	"control"
	"encoding/json"
	"fmt"
	"llm"
	log "mylog"
	h "net/http"
	"strings"
	"time"
)

// HandleAssistantChat handles assistant chat API using llm CallLM
// 智能助手聊天API处理函数 - 使用llm CallLM
func HandleAssistantChat(w h.ResponseWriter, r *h.Request) {
	log.Debug(log.ModuleHandler, "=== Assistant Chat Request Started (MCP Mode) ===")
	LogRemoteAddr("HandleAssistantChat", r)

	if checkLogin(r) != 0 {
		log.WarnF(log.ModuleHandler, "Unauthorized assistant chat request from %s", r.RemoteAddr)
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	llm.ProcessRequest(r, w)
}

// HandleAssistantChatHistory handles loading stored chat messages
// 智能助手聊天历史加载API处理函数
func HandleAssistantChatHistory(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantChatHistory", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// 获取日期参数，默认为今天
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		// 获取账户信息
		account := getAccountFromRequest(r)

		// 加载指定日期的聊天历史
		chatHistory := loadChatHistoryForDate(account, date)

		response := map[string]interface{}{
			"success":     true,
			"date":        date,
			"chatHistory": chatHistory,
			"timestamp":   time.Now().Unix(),
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// ChatMessage represents a chat message in the history
type ChatMessage struct {
	Role      string `json:"role"`      // "user" or "assistant"
	Content   string `json:"content"`   // message content
	Timestamp string `json:"timestamp"` // time when message was sent
}

// loadChatHistoryForDate loads chat history for a specific date
// 加载指定日期的聊天历史
func loadChatHistoryForDate(account, date string) []ChatMessage {
	// 构建AI助手日记标题
	diaryTitle := fmt.Sprintf("AI_assistant_%s", date)

	// 获取博客内容
	blog := control.GetBlog(account, diaryTitle)
	if blog == nil {
		log.DebugF(log.ModuleHandler, "No chat history found for date: %s", date)
		return []ChatMessage{}
	}

	// 解析博客内容，提取聊天记录
	return parseChatHistoryFromContent(blog.Content)
}

// parseChatHistoryFromContent parses chat messages from blog content
// 从博客内容中解析聊天记录
func parseChatHistoryFromContent(content string) []ChatMessage {
	var messages []ChatMessage

	// 按行分割内容
	lines := strings.Split(content, "\n")

	var currentMessage ChatMessage
	var inUserQuestion bool
	var inAIReply bool
	var currentTime string
	var contentBuilder strings.Builder

	for _, line := range lines {
		// 检测新对话开始的标记
		if strings.Contains(line, "### 🤖 AI助手对话") {
			// 提取时间戳
			if strings.Contains(line, "(") && strings.Contains(line, ")") {
				start := strings.Index(line, "(") + 1
				end := strings.Index(line, ")")
				if start < end {
					currentTime = line[start:end]
				}
			}
			continue
		}

		// 检测用户问题开始
		if strings.Contains(line, "**用户问题：**") {
			// 保存之前的AI回复消息（如果有的话）
			if inAIReply && contentBuilder.Len() > 0 {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				if currentMessage.Content != "" {
					messages = append(messages, currentMessage)
				}
				contentBuilder.Reset()
			}

			inUserQuestion = true
			inAIReply = false
			currentMessage = ChatMessage{
				Role:      "user",
				Timestamp: currentTime,
			}
			continue
		}

		// 检测AI回复开始
		if strings.Contains(line, "**AI回复：**") {
			// 保存用户问题消息
			if inUserQuestion && contentBuilder.Len() > 0 {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				if currentMessage.Content != "" {
					messages = append(messages, currentMessage)
				}
				contentBuilder.Reset()
			}

			inUserQuestion = false
			inAIReply = true
			currentMessage = ChatMessage{
				Role:      "assistant",
				Timestamp: currentTime,
			}
			continue
		}

		// 检测分割线，表示一次对话结束
		if strings.Contains(line, "----") && !strings.Contains(line, "|") {
			// 保存当前AI回复消息
			if inAIReply && contentBuilder.Len() > 0 {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				if currentMessage.Content != "" {
					messages = append(messages, currentMessage)
				}
				contentBuilder.Reset()
			}

			inUserQuestion = false
			inAIReply = false
			continue
		}

		// 收集消息内容
		if (inUserQuestion || inAIReply) && line != "" {
			if contentBuilder.Len() > 0 {
				contentBuilder.WriteString("\n")
			}
			contentBuilder.WriteString(line)
		}
	}

	// 处理最后一条消息
	if (inUserQuestion || inAIReply) && contentBuilder.Len() > 0 {
		currentMessage.Content = strings.TrimSpace(contentBuilder.String())
		if currentMessage.Content != "" {
			messages = append(messages, currentMessage)
		}
	}

	log.DebugF(log.ModuleAssistant, "Parsed %d chat messages from content", len(messages))
	return messages
}
