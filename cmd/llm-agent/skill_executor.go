package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// executeSkillSubTask 在独立子任务中执行技能
func (b *Bridge) executeSkillSubTask(ctx *TaskContext, skillName, query string, parentTools []LLMTool) string {
	start := time.Now()

	// 1. 查找 skill
	skill := b.skillMgr.GetSkill(skillName)
	if skill == nil {
		log.Printf("[SkillSubTask] 技能不存在: %s", skillName)
		return fmt.Sprintf("技能 '%s' 不存在，可用技能请参考 Skill 目录。", skillName)
	}

	// 1.5 检查所需 agent 是否在线
	if len(skill.Agents) > 0 {
		b.catalogMu.RLock()
		for _, requiredPrefix := range skill.Agents {
			found := false
			for agentID := range b.agentInfo {
				if strings.HasPrefix(agentID, requiredPrefix) {
					found = true
					break
				}
			}
			if !found {
				b.catalogMu.RUnlock()
				log.Printf("[SkillSubTask] ✗ 技能 %s 所需 agent %s 不在线", skillName, requiredPrefix)
				return fmt.Sprintf("技能 %s 无法执行：所需 agent %s offline", skillName, requiredPrefix)
			}
		}
		b.catalogMu.RUnlock()
	}

	// 2. 构建子任务 system prompt
	var sb strings.Builder
	now := time.Now()
	sb.WriteString(fmt.Sprintf("account: %s\n当前时间: %s %s\n\n",
		ctx.Account, now.Format("2006-01-02 15:04"), chineseWeekday(now.Weekday())))

	// 注入 agent 能力描述
	agentBlock := b.getAgentDescriptionBlock()
	if agentBlock != "" {
		sb.WriteString(agentBlock)
		sb.WriteString("\n")
	}

	// 注入 skill 详情（含历史经验）
	skillBlock := b.skillMgr.BuildSkillBlock([]SkillEntry{*skill})
	sb.WriteString(skillBlock)

	// 注入执行策略：优先 ExecuteCode 批量调用
	sb.WriteString("\n## 执行策略\n")
	sb.WriteString("**必须优先使用 ExecuteCode 工具**批量调用多个工具并整合数据。\n")
	sb.WriteString("- 将多个工具调用组合到一段代码中一次性执行，避免逐个调用工具进行多轮交互\n")
	sb.WriteString("- 在 ExecuteCode 代码中，直接使用 call_tool 调用具体工具（如 RawAddTodo, RawGetTodosByDate），不要调用 execute_skill\n")
	sb.WriteString("- call_tool 返回值已自动解析为 dict/list，无需再 json.loads\n")
	sb.WriteString("- 只在 ExecuteCode 无法覆盖的场景才单独调用工具\n")
	sb.WriteString("- 最终回复要简洁，直接给出用户需要的数据结果\n")

	// 3. 过滤工具（从全量工具列表中筛选，因为主列表已隐藏 skill 工具）
	allTools := b.getLLMTools()
	filteredTools := b.filterToolsForSkill(skill, allTools)
	log.Printf("[SkillSubTask] skill=%s tools=%d query=%s", skillName, len(filteredTools), truncate(query, 200))

	// 注入工具参数参考（让 LLM 在 ExecuteCode 中写 call_tool 时有直接参考）
	toolRef := b.buildToolParamReference(filteredTools)
	if toolRef != "" {
		sb.WriteString(toolRef)
	}

	systemPrompt := sb.String()

	// 4. Mini agentic loop（使用 ExecuteCode 批量调用时通常 2-3 轮即可完成）
	maxIter := 5
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}

	llmCfg, llmFallbacks := b.GetLLMConfigForSource(ctx.Source)
	var finalText string

	// 记录子任务中的工具调用摘要（用于返回给主 LLM 提供充分上下文）
	type skillToolCall struct {
		Name    string
		Args    string
		Success bool
		Result  string
	}
	var skillCalls []skillToolCall

	for i := 0; i < maxIter; i++ {
		// 检查取消
		if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
			log.Printf("[SkillSubTask] 取消 skill=%s", skillName)
			return "技能执行已取消。"
		}

		// 消息压缩
		if len(messages) > 15 || estimateChars(messages) > processMaxTotalChars*80/100 {
			before := len(messages)
			messages = sanitizeProcessMessages(messages)
			if len(messages) != before {
				log.Printf("[SkillSubTask] skill=%s 消息压缩: %d → %d", skillName, before, len(messages))
			}
		}

		log.Printf("[SkillSubTask] skill=%s 迭代 %d/%d messages=%d", skillName, i+1, maxIter, len(messages))

		// 最后一轮强制收敛
		iterTools := filteredTools
		if i == maxIter-1 {
			iterTools = nil
			messages = append(messages, Message{
				Role:    "system",
				Content: "请立即给出最终回复，总结已完成的工作和结果。不要再调用任何工具。",
			})
		}

		text, toolCalls, err := b.sendLLMWithConfig(llmCfg, llmFallbacks, messages, iterTools)
		if err != nil {
			log.Printf("[SkillSubTask] LLM 失败 skill=%s error=%v", skillName, err)
			return fmt.Sprintf("技能 %s 执行失败: %v", skillName, err)
		}

		// 无工具调用 → 完成
		if len(toolCalls) == 0 {
			finalText = text
			break
		}

		messages = append(messages, Message{Role: "assistant", Content: "", ToolCalls: toolCalls})

		// 执行工具调用
		for _, tc := range toolCalls {
			originalName := b.resolveToolName(tc.Function.Name)
			log.Printf("[SkillSubTask] skill=%s → 调用工具: %s args=%s",
				skillName, originalName, truncate(tc.Function.Arguments, 500))

			ctx.Sink.OnEvent("skill_tool_call", fmt.Sprintf("[%s] 调用 %s\n参数: %s",
				skillName, originalName, truncate(tc.Function.Arguments, 300)))

			// 检查工具是否在可用列表中（防止 LLM 编造工具名）
			if !isToolInList(originalName, filteredTools) {
				var availNames []string
				for _, ft := range filteredTools {
					availNames = append(availNames, ft.Function.Name)
				}
				result := fmt.Sprintf("工具 %s 不存在。可用工具: %s\n请使用正确的工具名重试。",
					originalName, strings.Join(availNames, ", "))
				log.Printf("[SkillSubTask] skill=%s 工具不存在: %s", skillName, originalName)
				ctx.Sink.OnEvent("skill_tool_result", fmt.Sprintf("[%s] ❌ %s 不存在，已提示可用工具列表",
					skillName, originalName))
				messages = append(messages, Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				})
				continue
			}

			callCtx := ctx.Ctx
			if callCtx == nil {
				callCtx = context.Background()
			}
			toolStart := time.Now()
			tcResult, err := b.CallToolCtx(callCtx, originalName, json.RawMessage(tc.Function.Arguments))

			var result string
			if tcResult != nil {
				result = tcResult.Result
			}
			if err != nil {
				result = fmt.Sprintf("工具调用失败: %v", err)
				log.Printf("[SkillSubTask] skill=%s 工具失败: %s error=%v", skillName, originalName, err)
				ctx.Sink.OnEvent("skill_tool_result", fmt.Sprintf("[%s] ❌ %s 失败 (%s)\n%s",
					skillName, originalName, fmtDuration(time.Since(toolStart)), truncate(result, 500)))
				skillCalls = append(skillCalls, skillToolCall{Name: originalName, Args: truncate(tc.Function.Arguments, 150), Success: false, Result: truncate(result, 200)})
			} else {
				log.Printf("[SkillSubTask] skill=%s ← 工具返回: %s resultLen=%d",
					skillName, originalName, len(result))
				ctx.Sink.OnEvent("skill_tool_result", fmt.Sprintf("[%s] ✅ %s 成功 (%s, %d字符)\n%s",
					skillName, originalName, fmtDuration(time.Since(toolStart)), len(result), truncate(result, 500)))
				skillCalls = append(skillCalls, skillToolCall{Name: originalName, Args: truncate(tc.Function.Arguments, 150), Success: true, Result: truncate(result, 300)})
			}

			messages = append(messages, Message{
				Role:       "tool",
				Content:    truncateToolResult(result, i),
				ToolCallID: tc.ID,
			})
		}
	}

	duration := time.Since(start)
	ctx.Sink.OnEvent("skill_done", fmt.Sprintf("技能 %s 执行完成 (%s)", skillName, fmtDuration(duration)))
	log.Printf("[SkillSubTask] ✓ skill=%s 完成 duration=%v resultLen=%d calls=%d", skillName, duration, len(finalText), len(skillCalls))

	// 构建结构化返回：执行日志 + LLM 总结
	// 让主 LLM 清楚知道子任务做了什么、调了哪些工具、结果如何
	var result strings.Builder
	if len(skillCalls) > 0 {
		result.WriteString(fmt.Sprintf("技能 %s 执行日志（%s，%d次工具调用）:\n", skillName, fmtDuration(duration), len(skillCalls)))
		for _, sc := range skillCalls {
			status := "✅"
			if !sc.Success {
				status = "❌"
			}
			result.WriteString(fmt.Sprintf("  %s %s(%s) → %s\n", status, sc.Name, sc.Args, sc.Result))
		}
		result.WriteString("\n")
	}
	if finalText != "" {
		result.WriteString(finalText)
	} else {
		result.WriteString(fmt.Sprintf("技能 %s 已执行但未产生总结。", skillName))
	}
	return result.String()
}

// filterToolsForSkill 按 skill 声明过滤工具
func (b *Bridge) filterToolsForSkill(skill *SkillEntry, parentTools []LLMTool) []LLMTool {
	// skill.Tools 为空 → 使用全量 parentTools（排除虚拟工具）
	if len(skill.Tools) == 0 {
		var filtered []LLMTool
		for _, t := range parentTools {
			name := t.Function.Name
			if name == "plan_and_execute" || name == "execute_skill" || name == "set_persona" || name == "set_rule" {
				continue
			}
			filtered = append(filtered, t)
		}
		return filtered
	}

	// skill.Tools 非空 → 只保留声明的工具 + ExecuteCode 基础工具
	hintSet := make(map[string]bool, len(skill.Tools))
	for _, t := range skill.Tools {
		hintSet[t] = true
		hintSet[sanitizeToolName(t)] = true
	}
	// 始终保留基础工具
	hintSet["ExecuteCode"] = true

	var filtered []LLMTool
	for _, t := range parentTools {
		name := t.Function.Name
		originalName := b.resolveToolName(name)
		if name == "plan_and_execute" || name == "execute_skill" || name == "set_persona" || name == "set_rule" {
			continue
		}
		if hintSet[name] || hintSet[originalName] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// isToolInList 检查工具名是否在工具列表中（考虑 sanitize）
func isToolInList(toolName string, tools []LLMTool) bool {
	for _, t := range tools {
		if t.Function.Name == toolName || unsanitizeToolName(t.Function.Name) == toolName {
			return true
		}
	}
	return false
}

// buildToolParamReference 从工具列表生成参数参考摘要，供 ExecuteCode 中 call_tool 使用
// 利用 toolCatalog 标注工具所属 agent
func (b *Bridge) buildToolParamReference(tools []LLMTool) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## call_tool 参数参考\n")
	sb.WriteString("在 ExecuteCode 中使用 call_tool(tool_name, {参数}) 调用，参数为 dict。\n\n")

	for _, t := range tools {
		name := unsanitizeToolName(t.Function.Name)
		desc := t.Function.Description

		// 查找工具所属 agent
		agentID := ""
		b.catalogMu.RLock()
		if id, ok := b.toolCatalog[name]; ok {
			agentID = id
		}
		b.catalogMu.RUnlock()

		// 解析 parameters JSON Schema 提取字段摘要
		paramSummary := extractParamSummary(t.Function.Parameters)

		// 格式化：有 agentID 时标注
		var toolLine string
		if agentID != "" {
			toolLine = fmt.Sprintf("- **%s** [%s]: %s\n", name, agentID, desc)
		} else {
			toolLine = fmt.Sprintf("- **%s**: %s\n", name, desc)
		}

		sb.WriteString(toolLine)
		if paramSummary != "" {
			sb.WriteString(fmt.Sprintf("  参数: %s\n", paramSummary))
		}
	}
	return sb.String()
}

// extractParamSummary 从 JSON Schema 提取参数摘要（如 "account*(str), date*(str,2025-01-01), id*(str)"）
func extractParamSummary(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}

	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(params, &schema); err != nil || len(schema.Properties) == 0 {
		return ""
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	var parts []string
	for name, prop := range schema.Properties {
		entry := name
		if requiredSet[name] {
			entry += "*"
		}
		detail := prop.Type
		if prop.Description != "" {
			detail = prop.Description
		}
		entry += "(" + detail + ")"
		parts = append(parts, entry)
	}
	return strings.Join(parts, ", ")
}
