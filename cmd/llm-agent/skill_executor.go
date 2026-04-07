package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

type skillLoopSink struct {
	base      EventSink
	skillName string
}

func (s *skillLoopSink) OnChunk(text string) {}

func (s *skillLoopSink) OnEvent(event, text string) {
	if s.base == nil {
		return
	}
	switch event {
	case "tool_call":
		s.base.OnEvent("skill_tool_call", text)
	case "tool_result":
		s.base.OnEvent("skill_tool_result", text)
	case "subtask_response":
		s.base.OnEvent("skill_tool_result", text)
	case "tool_expand":
		s.base.OnEvent("skill_tool_result", text)
	default:
		s.base.OnEvent(event, text)
	}
}

func (s *skillLoopSink) Streaming() bool { return false }

// executeSkillSubTask 在独立子任务中执行技能
func (b *Bridge) executeSkillSubTask(ctx *TaskContext, skillName, query string, parentTools []LLMTool) string {
	start := time.Now()

	// 1. 查找 skill
	skill := b.skillMgr.GetSkill(skillName)
	if skill == nil {
		log.Printf("[SkillSubTask] 技能不存在: %s", skillName)
		return fmt.Sprintf("技能 '%s' 不存在，可用技能请参考 Skill 目录。", skillName)
	}

	// 1.5 检查所需 agent 是否在线（同时检查 agentInfo 和 agentTools）
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
				// 回退：检查 agentTools（DiscoverTools 填充，可能先于 DiscoverAgents）
				for agentID := range b.agentTools {
					if strings.HasPrefix(agentID, requiredPrefix) {
						found = true
						break
					}
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

	// 注入执行策略（根据 skill 类型调整）
	hasSessionTools := false // 是否包含会话类工具（AcpStartSession、DeployProject 等）
	for _, t := range skill.Tools {
		if strings.HasPrefix(t, "Acp") || strings.HasPrefix(t, "Deploy") || strings.HasPrefix(t, "Codegen") {
			hasSessionTools = true
			break
		}
	}

	if hasSessionTools {
		// 会话类 skill（如 coding、deploy）：工具返回结果即完成，不要额外验证
		sb.WriteString("\n## 执行策略\n")
		sb.WriteString("- 调用子任务描述中指定的工具完成任务\n")
		sb.WriteString("- AcpStartSession/DeployProject/DeployAdhoc 返回后，任务即完成，**立即停止工具调用**，回复执行结果\n")
		sb.WriteString("- **禁止**在上述工具成功后继续调用 ExecuteCode 等补充工具\n")
		sb.WriteString("- 最终回复要简洁，包含执行结果和关键数据\n")
	} else {
		// 通用 skill：优先用 ExecuteCode 批量调用
		sb.WriteString("\n## 执行策略\n")
		sb.WriteString("**必须优先使用 ExecuteCode 工具**批量调用多个工具并整合数据。\n")
		sb.WriteString("- 将多个工具调用组合到一段代码中一次性执行，避免逐个调用工具进行多轮交互\n")
		sb.WriteString("- 在 ExecuteCode 代码中，直接使用 call_tool 调用具体工具（如 RawAddTodo, RawGetTodosByDate），不要调用 execute_skill\n")
		sb.WriteString("- call_tool 返回值已自动解析为 dict/list，无需再 json.loads\n")
		sb.WriteString("- 只在 ExecuteCode 无法覆盖的场景才单独调用工具\n")
		sb.WriteString("- 最终回复要简洁，直接给出用户需要的数据结果\n")
	}

	toolView := b.buildSkillToolRuntimeView(skill, parentTools)
	filteredTools := toolView.Visible()
	log.Printf("[SkillSubTask] skill=%s tools=%d all=%d query=%s", skillName, len(filteredTools), len(toolView.AllTools), query)

	// 注入工具参数参考（让 LLM 在 ExecuteCode 中写 call_tool 时有直接参考）
	toolRef := b.buildToolParamReference(filteredTools)
	if toolRef != "" {
		sb.WriteString(toolRef)
	}

	systemPrompt := sb.String()

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}
	session := NewRootSession("skill-"+newSessionID(), skillName, ctx.Account)
	session.Source = ctx.Source
	session.Messages = nil
	session.ToolCalls = nil
	session.AppendMessage(messages[0])
	session.AppendMessage(messages[1])

	orchCfg := *b.cfg
	orchCfg.SubTaskMaxIterations = 5
	orch := &Orchestrator{
		bridge:        b,
		cfg:           &orchCfg,
		activeHandles: make(map[string]*SubTaskHandle),
	}
	sink := &skillLoopSink{base: ctx.Sink, skillName: skillName}
	sendEvent := func(event, text string) {
		sink.OnEvent(event, text)
	}

	subtask := SubTaskPlan{
		ID:          "skill_" + newSessionID(),
		Title:       "skill:" + skillName,
		Description: query,
		ToolsHint:   skill.Tools,
	}

	finalText, loopErr := orch.runSubTaskLoop(ctx.Ctx, ctx.TaskID, subtask, session, messages, toolView, sendEvent, nil, time.Time{})
	if loopErr != nil {
		log.Printf("[SkillSubTask] LLM/loop 失败 skill=%s error=%v", skillName, loopErr)
		return fmt.Sprintf("技能 %s 执行失败: %v", skillName, loopErr)
	}

	duration := time.Since(start)
	ctx.Sink.OnEvent("skill_done", fmt.Sprintf("技能 %s 执行完成 (%s)", skillName, fmtDuration(duration)))
	session.mu.Lock()
	toolCalls := make([]ToolCallRecord, len(session.ToolCalls))
	copy(toolCalls, session.ToolCalls)
	session.mu.Unlock()
	log.Printf("[SkillSubTask] ✓ skill=%s 完成 duration=%v resultLen=%d calls=%d", skillName, duration, len(finalText), len(toolCalls))

	// 构建结构化返回：执行日志 + LLM 总结
	// 让主 LLM 清楚知道子任务做了什么、调了哪些工具、结果如何
	var result strings.Builder
	if len(toolCalls) > 0 {
		result.WriteString(fmt.Sprintf("技能 %s 执行日志（%s，%d次工具调用）:\n", skillName, fmtDuration(duration), len(toolCalls)))
		for _, sc := range toolCalls {
			status := "✅"
			if !sc.Success {
				status = "❌"
			}
			result.WriteString(fmt.Sprintf("  %s %s(%s) → %s\n", status, sc.ToolName, sc.Arguments, sc.Result))
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

	// skill.Tools 非空 → 只保留声明的工具
	hintSet := make(map[string]bool, len(skill.Tools))
	for _, t := range skill.Tools {
		hintSet[t] = true
		hintSet[sanitizeToolName(t)] = true
	}

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

// extractParamSummary 从 JSON Schema 提取参数摘要（如 "account*(string,账号), date*(string,日期,格式2025-01-01)"）
// 始终包含类型信息，让 LLM 在 ExecuteCode 中写 call_tool 时知道参数类型
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
		// 始终包含类型 + 描述，让 LLM 知道参数类型和格式
		typeName := prop.Type
		if typeName == "" {
			typeName = "string"
		}
		if prop.Description != "" {
			entry += "(" + typeName + "," + prop.Description + ")"
		} else {
			entry += "(" + typeName + ")"
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ", ")
}
