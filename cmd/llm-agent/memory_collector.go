package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// MemoryCollector 自动记忆收集器
type MemoryCollector struct {
	memoryMgr *MemoryManager
	bridge    *Bridge // 用于触发 skill 迭代时调用 LLM
	threshold int     // 同类错误触发 skill 迭代的阈值
}

// NewMemoryCollector 创建记忆收集器
func NewMemoryCollector(memoryMgr *MemoryManager, bridge *Bridge, threshold int) *MemoryCollector {
	if threshold <= 0 {
		threshold = 3
	}
	return &MemoryCollector{
		memoryMgr: memoryMgr,
		bridge:    bridge,
		threshold: threshold,
	}
}

// CollectAfterTask 任务完成后分析工具调用记录，自动收集记忆
func (c *MemoryCollector) CollectAfterTask(toolCalls []ToolCallRecord) {
	if c.memoryMgr == nil || len(toolCalls) == 0 {
		return
	}

	today := time.Now().Format("2006-01-02")

	for _, tc := range toolCalls {
		if tc.Success {
			continue
		}

		// 提取错误模式作为 errorKey
		errorKey := buildErrorKey(tc.ToolName, tc.Result)

		// 记录错误到记忆
		content := fmt.Sprintf("%s 调用失败。参数: %s\n错误: %s",
			tc.ToolName, truncate(tc.Arguments, 200), truncate(tc.Result, 300))

		c.memoryMgr.AddEntry(MemoryEntry{
			Date:     today,
			Category: "error",
			Source:   "tool_call",
			Content:  content,
		})

		// 累计错误计数
		count := c.memoryMgr.TrackError(errorKey)
		log.Printf("[MemoryCollector] 错误记录: %s (累计 %d 次)", errorKey, count)

		// 达到阈值 → 触发 skill 迭代
		if count >= c.threshold {
			go c.TriggerSkillIteration(errorKey, tc.ToolName)
		}
	}
}

// buildErrorKey 从工具名和错误结果中提取错误模式键
func buildErrorKey(toolName, result string) string {
	// 提取错误类型的关键词
	errorType := "unknown"
	lowerResult := strings.ToLower(result)

	switch {
	case strings.Contains(lowerResult, "timeout"):
		errorType = "timeout"
	case strings.Contains(lowerResult, "not found"):
		errorType = "not_found"
	case strings.Contains(lowerResult, "permission"):
		errorType = "permission"
	case strings.Contains(lowerResult, "syntax"):
		errorType = "syntax_error"
	case strings.Contains(lowerResult, "parameter") || strings.Contains(lowerResult, "参数"):
		errorType = "bad_params"
	case strings.Contains(lowerResult, "offline"):
		errorType = "agent_offline"
	}

	return toolName + ":" + errorType
}

// TriggerSkillIteration 自动 skill 迭代：分析累积错误，更新 SKILL.md
func (c *MemoryCollector) TriggerSkillIteration(errorKey, toolName string) {
	if c.bridge == nil || c.bridge.skillMgr == nil {
		return
	}

	log.Printf("[MemoryCollector] 触发 skill 迭代: errorKey=%s toolName=%s", errorKey, toolName)

	// 收集该工具相关的所有错误记忆
	c.memoryMgr.mu.RLock()
	var relatedErrors []string
	for _, entry := range c.memoryMgr.entries {
		if entry.Category == "error" && strings.Contains(entry.Content, toolName) {
			relatedErrors = append(relatedErrors, entry.Content)
		}
	}
	c.memoryMgr.mu.RUnlock()

	if len(relatedErrors) == 0 {
		return
	}

	// 构建 LLM prompt 分析错误模式
	var errorSummary strings.Builder
	for i, e := range relatedErrors {
		if i >= 10 { // 最多分析 10 条
			break
		}
		errorSummary.WriteString(fmt.Sprintf("%d. %s\n", i+1, e))
	}

	prompt := fmt.Sprintf(`分析以下工具 %s 的重复错误模式，生成一段简洁的使用指南（不超过 200 字），
帮助 AI 在未来避免同类错误。只输出指南内容，不要输出其他内容。

错误记录:
%s`, toolName, errorSummary.String())

	messages := []Message{
		{Role: "system", Content: "你是一个错误分析助手，负责从重复错误中提取模式并生成简洁的工具使用指南。"},
		{Role: "user", Content: prompt},
	}

	// 调用 LLM 分析
	text, _, err := c.bridge.sendLLM(messages, nil)
	if err != nil {
		log.Printf("[MemoryCollector] skill 迭代 LLM 调用失败: %v", err)
		return
	}

	if text == "" {
		return
	}

	// 记录 skill 迭代结果到记忆
	c.memoryMgr.AddEntry(MemoryEntry{
		Date:     time.Now().Format("2006-01-02"),
		Category: "auto_skill",
		Source:   "skill_iteration",
		Content:  fmt.Sprintf("工具 %s 使用指南（自动生成）:\n%s", toolName, text),
	})

	// 尝试更新对应的 SKILL.md
	c.updateSkillFile(toolName, text)

	log.Printf("[MemoryCollector] skill 迭代完成: %s → %d 字符指南", toolName, len(text))
}

// updateSkillFile 尝试将生成的指南追加到对应 skill 的 SKILL.md
func (c *MemoryCollector) updateSkillFile(toolName, guide string) {
	if c.bridge.skillMgr == nil {
		return
	}

	// 查找包含该工具的 skill
	skills := c.bridge.skillMgr.GetAllSkills()
	var targetSkill *SkillEntry
	for i, skill := range skills {
		for _, t := range skill.Tools {
			if t == toolName || strings.Contains(t, toolName) {
				targetSkill = &skills[i]
				break
			}
		}
		if targetSkill != nil {
			break
		}
	}

	if targetSkill == nil || targetSkill.FilePath == "" {
		log.Printf("[MemoryCollector] 未找到工具 %s 对应的 skill，跳过文件更新", toolName)
		return
	}

	// 追加到 SKILL.md
	appendContent := fmt.Sprintf("\n\n## 自动生成的使用指南 (%s)\n\n%s\n",
		time.Now().Format("2006-01-02"), guide)

	f, err := openFileAppend(targetSkill.FilePath)
	if err != nil {
		log.Printf("[MemoryCollector] 打开 SKILL.md 失败: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(appendContent); err != nil {
		log.Printf("[MemoryCollector] 写入 SKILL.md 失败: %v", err)
	}
}

// openFileAppend 以追加模式打开文件
func openFileAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
}
