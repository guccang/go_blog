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
