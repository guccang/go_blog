package main

import (
	"log"
	"strings"
)

// PolicyResult 策略管道执行结果
type PolicyResult struct {
	Tools          []LLMTool    // 过滤后的工具列表
	SelectedSkills []SkillEntry // 匹配到的 skill（仅用于 prompt 注入，不做工具隔离）
}

// ApplyPolicyPipeline 静态工具策略管道（零 LLM 调用）
// Layer 1: GlobalPolicy — 已在 DiscoverTools() 中由 applyToolPolicy() 执行，此处无需重复
// Layer 2: AgentPolicy — 关键词匹配 query vs agent 工具名/描述，工具数 ≤15 时跳过
// Layer 3: SkillMatch  — 关键词匹配 query vs skill keywords/description，仅返回匹配的 skill
// Layer 4: BaseToolGuard — 确保 ExecuteCode/Bash/文件工具始终存在
func (b *Bridge) ApplyPolicyPipeline(query string, tools []LLMTool) PolicyResult {
	result := PolicyResult{Tools: tools}

	// Layer 2: AgentPolicy（静态关键词匹配）
	if len(tools) > 15 {
		filtered := b.applyAgentPolicyStatic(query, tools)
		if len(filtered) > 0 && len(filtered) < len(tools) {
			log.Printf("[PolicyPipeline] Layer2 AgentPolicy: %d → %d", len(tools), len(filtered))
			result.Tools = filtered
		} else {
			log.Printf("[PolicyPipeline] Layer2 AgentPolicy: 跳过（全匹配或无匹配）")
		}
	} else {
		log.Printf("[PolicyPipeline] Layer2 AgentPolicy: 跳过（工具数 %d ≤ 15）", len(tools))
	}

	// Layer 3: SkillMatch（静态关键词匹配，仅返回 skill，不过滤工具）
	result.SelectedSkills = b.matchSkillsStatic(query, result.Tools)
	if len(result.SelectedSkills) > 0 {
		var names []string
		for _, s := range result.SelectedSkills {
			names = append(names, s.Name)
		}
		log.Printf("[PolicyPipeline] Layer3 SkillMatch: 匹配 %d 个 skill: %v", len(result.SelectedSkills), names)
	} else {
		log.Printf("[PolicyPipeline] Layer3 SkillMatch: 无匹配")
	}

	// Layer 4: BaseToolGuard（确保基础工具始终存在）
	result.Tools = b.ensureBaseTools(result.Tools, tools)

	return result
}

// applyAgentPolicyStatic Layer 2: 静态关键词匹配 agent 工具
func (b *Bridge) applyAgentPolicyStatic(query string, tools []LLMTool) []LLMTool {
	b.catalogMu.RLock()
	agentInfoCopy := make(map[string]AgentInfo, len(b.agentInfo))
	for k, v := range b.agentInfo {
		agentInfoCopy[k] = v
	}
	agentToolsCopy := make(map[string][]LLMTool, len(b.agentTools))
	for k, v := range b.agentTools {
		agentToolsCopy[k] = v
	}
	b.catalogMu.RUnlock()

	queryLower := strings.ToLower(query)

	// 中文关键词 → agent 工具名前缀映射
	cnKeywords := map[string][]string{
		"博客": {"blog", "raw"},
		"待办": {"todo", "raw"},
		"运动": {"exercise", "raw"},
		"阅读": {"reading", "raw"},
		"部署": {"deploy"},
		"编码": {"codegen"},
		"代码": {"codegen"},
		"搜索": {"web"},
		"网页": {"web"},
		"日记": {"blog", "raw"},
		"周报": {"raw"},
		"统计": {"raw"},
		"记录": {"raw"},
	}

	// 收集匹配的 agent ID
	matchedAgentIDs := make(map[string]bool)

	for _, info := range agentInfoCopy {
		// 基础 agent 始终保留
		if isExecuteCodeAgent(info) || isFileToolAgent(info) {
			matchedAgentIDs[info.ID] = true
			continue
		}

		matched := false

		// 检查中文关键词
		for keyword, prefixes := range cnKeywords {
			if strings.Contains(queryLower, keyword) {
				for _, prefix := range prefixes {
					for _, toolName := range info.ToolNames {
						if strings.HasPrefix(strings.ToLower(toolName), prefix) {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
			}
			if matched {
				break
			}
		}

		// 检查英文关键词（从工具名提取前缀）
		if !matched {
			enPrefixes := []string{"blog", "todo", "exercise", "deploy", "codegen", "web", "raw", "bash"}
			for _, prefix := range enPrefixes {
				if strings.Contains(queryLower, prefix) {
					for _, toolName := range info.ToolNames {
						if strings.HasPrefix(strings.ToLower(toolName), prefix) {
							matched = true
							break
						}
					}
				}
				if matched {
					break
				}
			}
		}

		// 检查 agent description
		if !matched && info.Description != "" {
			descLower := strings.ToLower(info.Description)
			// 提取 query 中的关键词片段（≥2 字符）进行模糊匹配
			for keyword := range cnKeywords {
				if strings.Contains(queryLower, keyword) && strings.Contains(descLower, keyword) {
					matched = true
					break
				}
			}
		}

		if matched {
			matchedAgentIDs[info.ID] = true
		}
	}

	// 无匹配或全匹配 → 不过滤
	if len(matchedAgentIDs) == 0 || len(matchedAgentIDs) == len(agentInfoCopy) {
		return tools
	}

	// 构建匹配 agent 的工具集
	matchedToolSet := make(map[string]bool)
	for agentID := range matchedAgentIDs {
		for _, tool := range agentToolsCopy[agentID] {
			matchedToolSet[tool.Function.Name] = true
		}
	}

	var filtered []LLMTool
	for _, tool := range tools {
		if matchedToolSet[tool.Function.Name] {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// matchSkillsStatic Layer 3: 静态关键词匹配 skill
func (b *Bridge) matchSkillsStatic(query string, tools []LLMTool) []SkillEntry {
	if b.skillMgr == nil {
		return nil
	}

	allSkills := b.skillMgr.GetAllSkills()
	if len(allSkills) == 0 {
		return nil
	}

	// 构建在线工具集
	onlineToolSet := make(map[string]bool, len(tools)*2)
	for _, t := range tools {
		onlineToolSet[t.Function.Name] = true
		onlineToolSet[unsanitizeToolName(t.Function.Name)] = true
	}

	queryLower := strings.ToLower(query)

	var matched []SkillEntry
	for _, skill := range allSkills {
		// 检查至少一个声明工具在线
		hasOnlineTool := false
		for _, toolName := range skill.Tools {
			if onlineToolSet[toolName] || onlineToolSet[sanitizeToolName(toolName)] {
				hasOnlineTool = true
				break
			}
		}
		if !hasOnlineTool {
			continue
		}

		// 检查关键词匹配
		if matchSkillKeywords(queryLower, skill) {
			matched = append(matched, skill)
		}
	}

	return matched
}

// matchSkillKeywords 检查 query 是否匹配 skill 的关键词
func matchSkillKeywords(queryLower string, skill SkillEntry) bool {
	// 优先使用 SKILL.md 的 keywords 字段
	if len(skill.Keywords) > 0 {
		for _, kw := range skill.Keywords {
			if strings.Contains(queryLower, strings.ToLower(kw)) {
				return true
			}
		}
		return false
	}

	// 无 keywords 字段：用 skill name 和 description 做模糊匹配
	if strings.Contains(queryLower, strings.ToLower(skill.Name)) {
		return true
	}
	if skill.Description != "" && len(skill.Description) > 2 {
		descLower := strings.ToLower(skill.Description)
		// 从 description 中提取中文关键词
		for _, r := range []rune(descLower) {
			_ = r // description 匹配作为兜底
		}
		if strings.Contains(queryLower, descLower) || strings.Contains(descLower, queryLower) {
			return true
		}
	}
	return false
}

// ApplySubtaskPolicy 子任务工具过滤（替代 filterToolsByHint）
// 按 ToolsHint 过滤 + 始终保留 base tools
func (b *Bridge) ApplySubtaskPolicy(tools []LLMTool, hints []string) []LLMTool {
	if len(hints) == 0 {
		return tools
	}

	hintSet := make(map[string]bool, len(hints)*2)
	for _, h := range hints {
		hintSet[h] = true
		hintSet[sanitizeToolName(h)] = true
	}

	var filtered []LLMTool
	for _, tool := range tools {
		if hintSet[tool.Function.Name] || b.isBaseTool(unsanitizeToolName(tool.Function.Name)) {
			filtered = append(filtered, tool)
		}
	}

	// 过滤后为空则回退全部工具
	if len(filtered) == 0 {
		log.Printf("[ApplySubtaskPolicy] 过滤后为空，回退全部工具")
		return tools
	}

	log.Printf("[ApplySubtaskPolicy] %d → %d (hints=%v)", len(tools), len(filtered), hints)
	return filtered
}

// ensureBaseTools Layer 4: 确保基础工具始终存在
func (b *Bridge) ensureBaseTools(filtered []LLMTool, allTools []LLMTool) []LLMTool {
	// 检查 filtered 中是否已包含所有基础工具
	existingSet := make(map[string]bool, len(filtered))
	for _, t := range filtered {
		existingSet[t.Function.Name] = true
	}

	var added int
	for _, t := range allTools {
		originalName := unsanitizeToolName(t.Function.Name)
		if b.isBaseTool(originalName) && !existingSet[t.Function.Name] {
			filtered = append(filtered, t)
			existingSet[t.Function.Name] = true
			added++
		}
	}

	if added > 0 {
		log.Printf("[PolicyPipeline] Layer4 BaseToolGuard: 补充 %d 个基础工具", added)
	}
	return filtered
}

// isBaseTool 判断是否为基础工具（始终保留，不参与过滤）
func (b *Bridge) isBaseTool(name string) bool {
	// 默认基础工具
	if name == "ExecuteCode" || name == "Bash" || isFileToolName(name) {
		return true
	}
	// 配置的额外基础工具
	if b.cfg.Pipeline != nil {
		for _, bt := range b.cfg.Pipeline.BaseTools {
			if name == bt {
				return true
			}
		}
	}
	return false
}
