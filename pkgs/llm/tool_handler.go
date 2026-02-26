package llm

import (
	"config"
	"context"
	"encoding/json"
	"fmt"
	"mcp"
	log "mylog"
	"net/http"
	"net/url"
	"strings"
)

// ToolExecutor handles tool call execution loop
type ToolExecutor struct {
	account        string
	maxCalls       int
	availableTools []mcp.LLMTool
	writer         http.ResponseWriter
	flusher        http.Flusher
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(account string, tools []mcp.LLMTool, w http.ResponseWriter, flusher http.Flusher) *ToolExecutor {
	return &ToolExecutor{
		account:        account,
		maxCalls:       25,
		availableTools: tools,
		writer:         w,
		flusher:        flusher,
	}
}

// ExecuteToolLoop executes the main tool calling loop
func (te *ToolExecutor) ExecuteToolLoop(query string, selectedTools []string) error {
	log.DebugF(log.ModuleLLM, "=== Streaming LLM Processing Started with Tool Support ===")
	log.DebugF(log.ModuleLLM, "Query: account=%s %s", te.account, query)
	log.DebugF(log.ModuleLLM, "Selected tools: %v", selectedTools)

	maxSelected := GetMaxSelectedTools()
	if len(selectedTools) > maxSelected {
		log.WarnF(log.ModuleLLM, "Selected tools count is too large, max is %d", maxSelected)
		selectedTools = selectedTools[:maxSelected]
	}

	sysPrompt := BuildEnhancedSystemPrompt(te.account)

	// Initialize messages
	messages := []Message{
		{
			Role:    "system",
			Content: sysPrompt,
		},
		{
			Role:    "user",
			Content: query,
		},
	}

	// Get available tools
	availableTools := mcp.GetAvailableLLMTools(selectedTools)
	log.DebugF(log.ModuleLLM, "Available LLM tools: %d", len(availableTools))

	// ========== Phase 1: 智能工具路由（当工具数 > 15 时启用） ==========
	if len(availableTools) > 15 {
		routedTools := te.routeTools(query, availableTools)
		if len(routedTools) > 0 {
			availableTools = routedTools
			log.MessageF(log.ModuleLLM, "[工具路由] 从 %d 个工具中筛选出 %d 个相关工具", len(mcp.GetAvailableLLMTools(selectedTools)), len(availableTools))
		}
	}

	var fullResponse strings.Builder
	var toolCallLog []string // 跟踪本轮调用的工具

	// 在聊天流中显示本次使用的工具数量
	toolCountMsg := fmt.Sprintf("[🔧 本次加载 %d 个工具]", len(availableTools))
	fmt.Fprintf(te.writer, "data: %s\n\n", url.QueryEscape(toolCountMsg))
	te.flusher.Flush()

	// Initial LLM call
	_, toolCalls, err := SendStreamingLLMRequest(messages, availableTools, te.writer, te.flusher, &fullResponse)
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Initial streaming LLM request failed: %v", err)
		return fmt.Errorf("initial streaming LLM request failed: %v", err)
	}

	// Tool calling loop with max iterations
	maxCall := te.maxCalls
	for len(toolCalls) > 0 && maxCall > 0 {
		maxCall--
		log.DebugF(log.ModuleLLM, "Tool calling iteration, remaining: %d", maxCall)

		// Process tool calls
		log.DebugF(log.ModuleLLM, "Processing %d tool calls", len(toolCalls))
		for _, toolCall := range toolCalls {
			// Log tool call status but don't send to client to keep response clean
			log.DebugF(log.ModuleLLM, fmt.Sprintf("\n[Calling tool %s with args %s]\n", toolCall.Function.Name, toolCall.Function.Arguments))

			toolName := toolCall.Function.Name
			toolArgs := make(map[string]interface{})
			toolCallLog = append(toolCallLog, toolName)

			fmt.Fprintf(te.writer, "data: %s\n\n", url.QueryEscape(fmt.Sprintf("[Calling tool %s with args %s]", toolCall.Function.Name, toolCall.Function.Arguments)))
			te.flusher.Flush()

			// Parse tool arguments with validation
			if toolCall.Function.Arguments == "" {
				log.WarnF(log.ModuleLLM, "Tool call %s has empty arguments, skipping", toolName)
				continue
			}

			// Validate JSON format first
			if !IsValidJSON(toolCall.Function.Arguments) {
				log.ErrorF(log.ModuleLLM, "Tool call %s has invalid JSON arguments: %s", toolName, toolCall.Function.Arguments)
				continue
			}

			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs); err != nil {
				log.ErrorF(log.ModuleLLM, "Failed to parse tool arguments for %s: %v, args: %s", toolName, err, toolCall.Function.Arguments)
				continue
			}

			// Call the tool
			log.InfoF(log.ModuleLLM, "Tool call begin: %s %v", toolName, toolArgs)
			result := mcp.CallMCPTool(toolName, toolArgs)
			log.InfoF(log.ModuleLLM, "Tool call result: %s %v %v", toolName, toolArgs, result)

			// Add tool call and result to message history
			messages = append(messages, Message{
				Role:      "assistant",
				ToolCalls: []mcp.ToolCall{SanitizeToolCall(toolCall)},
			})

			toolContent := TruncateString(fmt.Sprintf("%v", result.Result), MaxToolResultChars)
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallId: toolCall.ID,
				Content:    toolContent,
			})

			// Add tool call info to full response for saving
			save := config.GetConfigWithAccount(config.GetAdminAccount(), "assistant_save_mcp_result")
			// len(result.Result) < 32 indicates short result, not privacy data, can be stored in Assistant_xxx
			if strings.ToLower(save) == "true" || len(fmt.Sprintf("%v", result.Result)) < 32 {
				fullResponse.WriteString(fmt.Sprintf("\n[Tool %s called with result: %v]\n", toolName, result.Result))
			} else {
				// Don't display tool callback returned data, involves privacy, sending to LLM is fine, but caching and displaying on UI is problematic
				fullResponse.WriteString(fmt.Sprintf("\n[Tool %s called with result: %s]\n", toolName, "###$#&$#*$)@$&$%&$())!@###"))
			}
		}

		// Next LLM call with updated messages
		log.InfoF(log.ModuleLLM, "Tool calls processed, sending next LLM request")
		// 上下文压缩：当消息过长时压缩旧消息（参考 Anthropic Compaction）
		messages = CompactMessages(messages, te.account)
		_, toolCalls, err = SendStreamingLLMRequest(messages, availableTools, te.writer, te.flusher, &fullResponse)
		if err != nil {
			log.ErrorF(log.ModuleLLM, "LLM call failed in tool loop: %v", err)
			break
		}
		log.InfoF(log.ModuleLLM, "Next LLM response received, tool calls: %d", len(toolCalls))
	}

	// Send completion signal to client
	log.DebugF(log.ModuleLLM, "Tool processing complete, sending DONE signal")
	fmt.Fprintf(te.writer, "data: [DONE]\n\n")
	te.flusher.Flush()

	// Save complete response to diary
	go SaveLLMResponseToDiary(te.account, query, fullResponse.String())
	// Save structured session progress for cross-session memory
	go SaveSessionProgress(te.account, query, fullResponse.String(), toolCallLog)
	return nil
}

// ProcessQueryStreaming is a convenience function wrapping ToolExecutor
func ProcessQueryStreaming(account string, query string, selectedTools []string, w http.ResponseWriter, flusher http.Flusher) error {
	executor := NewToolExecutor(account, nil, w, flusher)
	return executor.ExecuteToolLoop(query, selectedTools)
}

// routeTools 工具路由：用 LLM 从工具目录中筛选与用户问题相关的工具
func (te *ToolExecutor) routeTools(query string, allTools []mcp.LLMTool) []mcp.LLMTool {
	// 构建工具目录（仅 name + description，不含参数 schema，节省 token）
	var catalog strings.Builder
	toolMap := make(map[string]mcp.LLMTool, len(allTools))
	for i, tool := range allTools {
		catalog.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, tool.Function.Name, tool.Function.Description))
		toolMap[tool.Function.Name] = tool
	}

	routePrompt := fmt.Sprintf(`你是一个工具路由器。根据用户的问题，从以下工具目录中选择所有可能需要用到的工具。

用户问题: %s

工具目录:
%s
选择规则：
1. 宁多勿少，把所有可能相关的工具都选上
2. 如果任务需要日期信息，必须包含 RawCurrentDate
3. 如果涉及查询数据，同时选择获取数据的工具和可能需要的辅助工具
4. 只返回JSON数组，不要其他文字

示例: ["RawCurrentDate", "RawGetExerciseByDateRange"]
如果不需要任何工具，返回 []`, query, catalog.String())

	routeMessages := []Message{
		{Role: "user", Content: routePrompt},
	}

	resp, err := SendSyncLLMRequestNoTools(context.Background(), routeMessages, te.account)
	if err != nil {
		log.WarnF(log.ModuleLLM, "[工具路由] LLM 调用失败: %v, 使用全部工具", err)
		return nil // fallback 到全部工具
	}

	// 解析 JSON 数组
	resp = strings.TrimSpace(resp)
	// 去掉可能的 markdown 代码块包裹
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var toolNames []string
	if err := json.Unmarshal([]byte(resp), &toolNames); err != nil {
		log.WarnF(log.ModuleLLM, "[工具路由] 解析失败: %v, 原始响应: %s", err, resp)
		return nil // fallback 到全部工具
	}

	if len(toolNames) == 0 {
		log.MessageF(log.ModuleLLM, "[工具路由] LLM 判断无需工具")
		return []mcp.LLMTool{} // 返回空，让 LLM 直接回答
	}

	// 筛选出对应的完整工具定义
	var selected []mcp.LLMTool
	for _, name := range toolNames {
		if tool, ok := toolMap[name]; ok {
			selected = append(selected, tool)
		}
	}

	if len(selected) == 0 {
		log.WarnF(log.ModuleLLM, "[工具路由] 未匹配到任何工具，使用全部工具")
		return nil
	}

	log.MessageF(log.ModuleLLM, "[工具路由] 选中工具: %v", toolNames)
	return selected
}
