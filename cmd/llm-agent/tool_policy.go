package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"unicode/utf8"
)

// DisclosureLevel 查询披露级别（控制 system prompt 注入量）
type DisclosureLevel int

const (
	LevelZero DisclosureLevel = 0 // 闲聊：不注入任何 skill/agent 信息
	LevelOne  DisclosureLevel = 1 // 问答：仅注入 skill 目录（含 summary）
	LevelTwo  DisclosureLevel = 2 // 任务：目录 + 匹配的 skill 详情
)

// PolicyResult 策略管道执行结果
type PolicyResult struct {
	Tools          []LLMTool       // 过滤后的工具列表
	SelectedSkills []SkillEntry    // 匹配到的 skill（仅用于 prompt 注入，不做工具隔离）
	Level          DisclosureLevel // 查询披露级别
}

// ApplyPolicyPipeline 静态工具策略管道（零 LLM 调用）
// Layer 0: QueryClassify — 判断查询披露级别（LevelZero/LevelOne/LevelTwo）
// Layer 1: GlobalPolicy — 已在 DiscoverTools() 中由 applyToolPolicy() 执行，此处无需重复
// Layer 2: AgentPolicy — 关键词匹配 query vs agent 工具名/描述，工具数 ≤15 时跳过
// Layer 3: SkillMatch  — 评分匹配 query vs skill keywords，Top-N 限制
// Layer 4: BaseToolGuard — 确保 ExecuteCode/Bash/文件工具始终存在
func (b *Bridge) ApplyPolicyPipeline(query string, tools []LLMTool) PolicyResult {
	result := PolicyResult{Tools: tools}

	// Layer 0: 查询分级
	result.Level = classifyQueryLevel(query, b.skillMgr)
	log.Printf("[PolicyPipeline] Layer0 QueryClassify: Level=%d query=%s", result.Level, truncate(query, 50))

	// LevelZero: 闲聊，跳过 skill 匹配
	if result.Level == LevelZero {
		result.SelectedSkills = nil
		// 仍然执行 Layer2 和 Layer4（工具过滤仍需要）
	}

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

	// Layer 3: SkillMatch（LevelTwo 时才执行评分匹配）
	if result.Level == LevelTwo {
		maxN := b.cfg.MaxMatchedSkills
		if maxN <= 0 {
			maxN = 2
		}
		result.SelectedSkills = b.matchSkillsScored(query, result.Tools, maxN)
		if len(result.SelectedSkills) > 0 {
			var names []string
			for _, s := range result.SelectedSkills {
				names = append(names, s.Name)
			}
			log.Printf("[PolicyPipeline] Layer3 SkillMatch: 匹配 %d 个 skill: %v", len(result.SelectedSkills), names)
		} else {
			log.Printf("[PolicyPipeline] Layer3 SkillMatch: 无匹配")
		}
	} else {
		result.SelectedSkills = nil
		log.Printf("[PolicyPipeline] Layer3 SkillMatch: 跳过（Level=%d）", result.Level)
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
		"博客":   {"blog", "raw"},
		"文章":   {"blog", "raw"},
		"待办":   {"todo", "raw"},
		"任务":   {"todo", "raw", "corn"},
		"运动":   {"exercise", "raw"},
		"锻炼":   {"exercise", "raw"},
		"健身":   {"exercise", "raw"},
		"跑步":   {"exercise", "raw"},
		"慢跑":   {"exercise", "raw"},
		"引体向上": {"exercise", "raw"},
		"俯卧撑":  {"exercise", "raw"},
		"阅读":   {"reading", "raw"},
		"读书":   {"reading", "raw"},
		"部署":   {"deploy"},
		"编码":   {"codegen"},
		"代码":   {"codegen"},
		"搜索":   {"web"},
		"网页":   {"web"},
		"日记":   {"blog", "raw"},
		"周报":   {"raw"},
		"统计":   {"raw"},
		"记录":   {"raw"},
		"定时":   {"corn"},
		"提醒":   {"corn"},
		"闹钟":   {"corn"},
		"每隔":   {"corn"},
		"周期":   {"corn"},
		"定期":   {"corn"},
		"分钟后":  {"corn"},
		"小时后":  {"corn"},
		"cron":  {"corn"},
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
			enPrefixes := []string{"blog", "todo", "exercise", "deploy", "codegen", "web", "raw", "bash", "corn"}
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

	// 收集 query 命中的所有工具名前缀（兜底：直接按工具名匹配，不依赖 agent 元数据）
	var queryMatchedPrefixes []string
	for keyword, prefixes := range cnKeywords {
		if strings.Contains(queryLower, keyword) {
			queryMatchedPrefixes = append(queryMatchedPrefixes, prefixes...)
		}
	}
	for _, prefix := range []string{"blog", "todo", "exercise", "deploy", "codegen", "web", "raw", "bash", "corn"} {
		if strings.Contains(queryLower, prefix) {
			queryMatchedPrefixes = append(queryMatchedPrefixes, prefix)
		}
	}

	var filtered []LLMTool
	for _, tool := range tools {
		if matchedToolSet[tool.Function.Name] {
			filtered = append(filtered, tool)
			continue
		}
		// 兜底：工具名前缀直接匹配 query 关键词
		toolNameLower := strings.ToLower(b.resolveToolName(tool.Function.Name))
		for _, prefix := range queryMatchedPrefixes {
			if strings.HasPrefix(toolNameLower, prefix) {
				filtered = append(filtered, tool)
				break
			}
		}
	}

	return filtered
}

// classifyQueryLevel 纯启发式查询分级（零 LLM 调用）
// LevelZero: 短问候/闲聊
// LevelOne: 纯问答无工具意图
// LevelTwo: 默认（含 skill 关键词或明确任务意图）
func classifyQueryLevel(query string, skillMgr *SkillManager) DisclosureLevel {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)
	runeCount := utf8.RuneCountInString(q)

	// 收集所有 skill 关键词用于检测
	var allKeywords []string
	if skillMgr != nil {
		for _, skill := range skillMgr.GetAllSkills() {
			allKeywords = append(allKeywords, skill.Keywords...)
		}
	}

	hasSkillKeyword := false
	for _, kw := range allKeywords {
		if strings.Contains(qLower, strings.ToLower(kw)) {
			hasSkillKeyword = true
			break
		}
	}

	// LevelZero: 短问候/闲聊
	greetings := []string{"你好", "hi", "hello", "hey", "谢谢", "感谢", "好的", "ok", "嗯", "哈哈", "呵呵", "嘿"}
	if runeCount <= 6 && !hasSkillKeyword {
		for _, g := range greetings {
			if strings.Contains(qLower, g) {
				return LevelZero
			}
		}
		// 极短且不含关键词
		if runeCount <= 4 {
			return LevelZero
		}
	}

	// 有 skill 关键词 → LevelTwo
	if hasSkillKeyword {
		return LevelTwo
	}

	// LevelOne: 纯问答模式（"什么是"、"解释"、"为什么"、"怎么理解"等提问）
	questionPatterns := []string{"什么是", "是什么", "解释", "为什么", "怎么理解", "区别是", "概念", "原理", "含义", "意思是"}
	for _, p := range questionPatterns {
		if strings.Contains(qLower, p) {
			return LevelOne
		}
	}

	// 默认 LevelTwo（任务意图）
	return LevelTwo
}

// skillScore 评分匹配结果
type skillScore struct {
	skill SkillEntry
	score int
}

// matchSkillsScored Layer 3: 评分匹配 skill，返回 Top-N
func (b *Bridge) matchSkillsScored(query string, tools []LLMTool, maxN int) []SkillEntry {
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
		onlineToolSet[b.resolveToolName(t.Function.Name)] = true
	}

	queryLower := strings.ToLower(query)

	var scored []skillScore
	var scoreLog []string

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

		// 评分
		score := scoreSkill(queryLower, skill)
		if score > 0 {
			scored = append(scored, skillScore{skill: skill, score: score})
			scoreLog = append(scoreLog, fmt.Sprintf("%s=%d", skill.Name, score))
		}
	}

	if len(scored) == 0 {
		return nil
	}

	// 按分数降序排列
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Top-N
	if len(scored) > maxN {
		var topNames []string
		for _, s := range scored[:maxN] {
			topNames = append(topNames, s.skill.Name)
		}
		log.Printf("[PolicyPipeline] Layer3 SkillMatch: %s → top %d: %s",
			strings.Join(scoreLog, ", "), maxN, strings.Join(topNames, ", "))
		scored = scored[:maxN]
	}

	result := make([]SkillEntry, len(scored))
	for i, s := range scored {
		result[i] = s.skill
	}
	return result
}

// scoreSkill 对单个 skill 进行评分
func scoreSkill(queryLower string, skill SkillEntry) int {
	score := 0
	matchedCount := 0

	// skill name 完全包含在 query 中 +5 分
	if strings.Contains(queryLower, strings.ToLower(skill.Name)) {
		score += 5
	}

	// 关键词匹配评分
	if len(skill.Keywords) > 0 {
		for _, kw := range skill.Keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(queryLower, kwLower) {
				score += 2
				matchedCount++
				// 长关键词加分（更具体）：中文≥3字符 或 英文≥5字符
				kwRuneCount := utf8.RuneCountInString(kw)
				isChinese := kwRuneCount < len(kw) // 含多字节字符即视为中文
				if (isChinese && kwRuneCount >= 3) || (!isChinese && kwRuneCount >= 5) {
					score++
				}
			}
		}
	} else {
		// 无 keywords：用 description 做模糊匹配
		if skill.Description != "" {
			descLower := strings.ToLower(skill.Description)
			if strings.Contains(queryLower, descLower) || strings.Contains(descLower, queryLower) {
				score += 2
				matchedCount++
			}
		}
	}

	// 多重匹配奖励
	if matchedCount >= 2 {
		score += 2
	}

	return score
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
		if hintSet[tool.Function.Name] || b.isBaseTool(b.resolveToolName(tool.Function.Name)) {
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
		originalName := b.resolveToolName(t.Function.Name)
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
	if name == "ExecuteCode" || name == "Bash" || strings.HasSuffix(name, ".Bash") || isFileToolName(name) {
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
