package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"uap"
)

// handleChat 处理聊天消息：构建上下文 → LLM 对话 → 工具调用循环 → 返回最终回复
func (b *Bridge) handleChat(fromAgent, wechatUser, content string) {
	log.Printf("[Chat] from=%s user=%s content=%s", fromAgent, wechatUser, content)

	// 1. 构建 system prompt（并发获取用户上下文）
	systemPrompt := b.buildSystemPrompt(wechatUser)

	// 2. 初始化消息列表
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: content},
	}

	// 3. 获取工具列表（智能路由：>15 时用 LLM 筛选相关工具）
	tools := b.getLLMTools()
	if len(tools) > 15 {
		tools = b.routeTools(content, tools)
	}

	// 4. 工具调用循环
	maxIter := b.cfg.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	var finalText string

	for i := 0; i < maxIter; i++ {
		log.Printf("[Chat] iteration %d/%d, messages=%d", i+1, maxIter, len(messages))

		text, toolCalls, err := SendLLMRequest(&b.cfg.LLM, messages, tools)
		if err != nil {
			log.Printf("[Chat] LLM error: %v", err)
			finalText = fmt.Sprintf("抱歉，AI 服务暂时不可用: %v", err)
			break
		}

		// 无工具调用 → 对话结束
		if len(toolCalls) == 0 {
			finalText = text
			break
		}

		// 有工具调用 → 追加 assistant 消息
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   text,
			ToolCalls: toolCalls,
		})

		// 执行每个工具调用
		for _, tc := range toolCalls {
			// 将 LLM 函数名还原为原始工具名
			originalName := unsanitizeToolName(tc.Function.Name)

			log.Printf("[Chat] tool_call: %s args=%s", originalName, tc.Function.Arguments)

			result, err := b.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				log.Printf("[Chat] tool_call %s failed: %v", originalName, err)
				result = fmt.Sprintf("工具调用失败: %v", err)
			}

			// 追加 tool 消息
			messages = append(messages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		// 如果是最后一次迭代，说明工具调用过多
		if i == maxIter-1 {
			finalText = "抱歉，处理过程过于复杂，请尝试简化您的请求。"
		}
	}

	if finalText == "" {
		finalText = "抱歉，未能生成回复。"
	}

	// 5. 发送最终回复给 wechat-agent
	err := b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      wechatUser,
		Content: finalText,
	})
	if err != nil {
		log.Printf("[Chat] send reply failed: %v", err)
	} else {
		log.Printf("[Chat] reply sent to %s via %s (%d chars)", wechatUser, fromAgent, len(finalText))
	}
}

// buildSystemPrompt 构建系统提示（含用户上下文数据）
func (b *Bridge) buildSystemPrompt(wechatUser string) string {
	var sb strings.Builder
	sb.WriteString(b.cfg.SystemPromptPrefix)
	sb.WriteString("\n\n")

	account := b.cfg.DefaultAccount
	today := time.Now().Format("2006-01-02")

	sb.WriteString(fmt.Sprintf("当前用户: %s\n", wechatUser))
	sb.WriteString(fmt.Sprintf("当前日期: %s\n", today))

	// 并发获取上下文数据（3 秒超时，失败跳过）
	type ctxResult struct {
		label string
		data  string
	}

	var wg sync.WaitGroup
	results := make(chan ctxResult, 2)

	// 获取今日待办
	wg.Add(1)
	go func() {
		defer wg.Done()
		args, _ := json.Marshal(map[string]string{"account": account, "date": today})
		data, err := b.callToolWithTimeout("todolist.GetTodos", args, 3*time.Second)
		if err == nil && data != "" {
			results <- ctxResult{label: "今日待办", data: data}
		}
	}()

	// 获取今日运动
	wg.Add(1)
	go func() {
		defer wg.Done()
		args, _ := json.Marshal(map[string]string{"account": account, "date": today})
		data, err := b.callToolWithTimeout("exercise.GetRecords", args, 3*time.Second)
		if err == nil && data != "" {
			results <- ctxResult{label: "今日运动", data: data}
		}
	}()

	// 等待所有完成后关闭 channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var ctxParts []string
	for r := range results {
		ctxParts = append(ctxParts, fmt.Sprintf("[%s]\n%s", r.label, r.data))
	}

	if len(ctxParts) > 0 {
		sb.WriteString("\n用户当前数据:\n")
		sb.WriteString(strings.Join(ctxParts, "\n\n"))
	}

	return sb.String()
}

// callToolWithTimeout 带超时的工具调用
func (b *Bridge) callToolWithTimeout(toolName string, args json.RawMessage, timeout time.Duration) (string, error) {
	// 检查工具是否在目录中
	if _, ok := b.getToolAgent(toolName); !ok {
		return "", fmt.Errorf("tool %s not in catalog", toolName)
	}

	type result struct {
		data string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := b.CallTool(toolName, args)
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout after %v", timeout)
	}
}
