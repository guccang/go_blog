package main

import (
	"log"
	"strings"
	"unicode/utf8"
)

// isGreeting 判断是否为短问候/闲聊（不需要工具）
func isGreeting(query string) bool {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)
	runeCount := utf8.RuneCountInString(q)

	greetings := []string{"你好", "hi", "hello", "hey", "谢谢", "感谢", "好的", "ok", "嗯", "哈哈", "呵呵", "嘿"}
	if runeCount <= 6 {
		for _, g := range greetings {
			if strings.Contains(qLower, g) {
				return true
			}
		}
		// 极短消息
		if runeCount <= 4 {
			return true
		}
	}
	return false
}

// ApplySubtaskPolicy 子任务工具过滤
// 当 hints 匹配到 skill 时：只保留 skill 声明的工具（不保留 ExecuteCode 等基础工具）
// 当 hints 未匹配 skill 时：保留 hint 工具 + 基础工具
func (b *Bridge) ApplySubtaskPolicy(tools []LLMTool, hints []string) []LLMTool {
	if len(hints) == 0 {
		return tools
	}

	hintSet := make(map[string]bool, len(hints)*3)
	for _, h := range hints {
		hintSet[h] = true
		hintSet[sanitizeToolName(h)] = true
		// 裸名→canonical→sanitized 映射，确保命名空间工具也能匹配
		if canonical := b.resolveToolName(h); canonical != h {
			hintSet[sanitizeToolName(canonical)] = true
		}
	}

	// 检测 hints 是否匹配到 skill —— 匹配到则不保留基础工具
	hasSkillMatch := false
	if b.skillMgr != nil {
		matched := b.skillMgr.MatchByTools(hints)
		if len(matched) > 0 {
			hasSkillMatch = true
			// 将 skill 声明的所有工具也加入 hintSet
			for _, skill := range matched {
				for _, t := range skill.Tools {
					hintSet[t] = true
					hintSet[sanitizeToolName(t)] = true
					if canonical := b.resolveToolName(t); canonical != t {
						hintSet[sanitizeToolName(canonical)] = true
					}
				}
			}
			log.Printf("[ApplySubtaskPolicy] skill 匹配，扩展工具集: hints=%v", hints)
		}
	}

	var filtered []LLMTool
	for _, tool := range tools {
		if hintSet[tool.Function.Name] {
			filtered = append(filtered, tool)
		} else if !hasSkillMatch && b.isBaseTool(b.resolveToolName(tool.Function.Name)) {
			// 未匹配 skill 时才保留基础工具
			filtered = append(filtered, tool)
		}
	}

	// 过滤后为空则回退全部工具
	if len(filtered) == 0 {
		log.Printf("[ApplySubtaskPolicy] 过滤后为空，回退全部工具")
		return tools
	}

	log.Printf("[ApplySubtaskPolicy] %d → %d (hints=%v hasSkill=%v)", len(tools), len(filtered), hints, hasSkillMatch)
	return filtered
}

// ensureBaseTools 确保基础工具始终存在
func (b *Bridge) ensureBaseTools(filtered []LLMTool, allTools []LLMTool) []LLMTool {
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
		log.Printf("[ensureBaseTools] 补充 %d 个基础工具", added)
	}
	return filtered
}

// isBaseTool 判断是否为基础工具（始终保留，不参与过滤）
func (b *Bridge) isBaseTool(name string) bool {
	// 默认基础工具：仅保留 ExecuteCode 和文件工具
	// 移除 Bash，防止子任务绕过计划指定的工具
	if name == "ExecuteCode" || isFileToolName(name) {
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
