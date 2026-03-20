package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	// 收集所有失败的工具调用，同时更新错误计数
	var failedCalls []ToolCallRecord
	for _, tc := range toolCalls {
		if tc.Success {
			continue
		}

		errorKey := buildErrorKey(tc.ToolName, tc.Result)
		count := c.memoryMgr.TrackError(errorKey)
		log.Printf("[MemoryCollector] 错误记录: %s (累计 %d 次)", errorKey, count)

		failedCalls = append(failedCalls, tc)

		// 达到阈值 → 触发 skill 迭代
		if count >= c.threshold {
			go c.TriggerSkillIteration(errorKey, tc.ToolName)
		}
	}

	if len(failedCalls) > 0 {
		go c.summarizeAndStore(failedCalls, today)
	}

	// 任务结束后：整理 auto_skill 日期文件为汇总文件
	go c.CompactAutoSkills()
}

// summarizeAndStore 用 LLM 将原始错误整理为简洁经验后存入记忆
func (c *MemoryCollector) summarizeAndStore(failedCalls []ToolCallRecord, today string) {
	// 构建错误摘要
	var errorDetails strings.Builder
	for i, tc := range failedCalls {
		errorDetails.WriteString(fmt.Sprintf("%d. 工具: %s\n   参数: %s\n   错误: %s\n",
			i+1, tc.ToolName, truncate(tc.Arguments, 150), truncate(tc.Result, 200)))
	}

	prompt := fmt.Sprintf(`将以下 %d 条工具调用错误整理为简洁的经验教训。
要求：
- 每条经验一行，提取错误模式和正确做法
- 不要复述原始参数和错误堆栈
- 总字数不超过 200 字
- 只输出整理后的内容

错误记录:
%s`, len(failedCalls), errorDetails.String())

	messages := []Message{
		{Role: "system", Content: "你是一个错误分析助手，负责将工具调用错误整理为简洁实用的经验教训。"},
		{Role: "user", Content: prompt},
	}

	text, _, err := c.bridge.sendLLM(messages, nil)
	if err != nil {
		log.Printf("[MemoryCollector] LLM 整理失败，存储简短摘要: %v", err)
		// fallback: 存一条简短摘要，不存原始日志
		var toolNames []string
		seen := make(map[string]bool)
		for _, tc := range failedCalls {
			if !seen[tc.ToolName] {
				toolNames = append(toolNames, tc.ToolName)
				seen[tc.ToolName] = true
			}
		}
		c.memoryMgr.AddEntry(MemoryEntry{
			Date:     today,
			Category: "error",
			Source:   "tool_call",
			Content:  fmt.Sprintf("%d 个工具调用失败: %s", len(failedCalls), strings.Join(toolNames, ", ")),
		})
		return
	}

	c.memoryMgr.AddEntry(MemoryEntry{
		Date:     today,
		Category: "pattern",
		Source:   "tool_call",
		Content:  strings.TrimSpace(text),
	})

	log.Printf("[MemoryCollector] 错误整理完成: %d 条错误 → %d 字符经验", len(failedCalls), len(text))
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

// TriggerSkillIteration 自动 skill 迭代：分析累积错误，写入按技能+日期文件
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

	// 记录 skill 迭代结果到记忆（appendToFile 内部根据 category 自动分流到 auto_skill 文件）
	c.memoryMgr.AddEntry(MemoryEntry{
		Date:     time.Now().Format("2006-01-02"),
		Category: "auto_skill",
		Source:   "skill_iteration",
		Content:  fmt.Sprintf("工具 %s 使用指南（自动生成）:\n%s", toolName, text),
	})

	log.Printf("[MemoryCollector] skill 迭代完成: %s → %d 字符指南", toolName, len(text))
}

// findSkillNameForTool 通过 SkillManager 查找工具名对应的技能名
func (c *MemoryCollector) findSkillNameForTool(toolName string) string {
	if c.bridge == nil || c.bridge.skillMgr == nil {
		return ""
	}
	skills := c.bridge.skillMgr.GetAllSkills()
	for _, skill := range skills {
		for _, t := range skill.Tools {
			if t == toolName || strings.Contains(t, toolName) {
				return skill.Name
			}
		}
	}
	return ""
}

// CompactAutoSkills 整理 auto_skill 日期文件为汇总文件
func (c *MemoryCollector) CompactAutoSkills() {
	if c.memoryMgr == nil || c.bridge == nil {
		return
	}

	// 扫描所有未整理的 auto_skill 日期文件
	datedFiles, err := c.memoryMgr.listAutoSkillDatedFiles()
	if err != nil || len(datedFiles) == 0 {
		return
	}

	// 按 skillName 分组
	grouped := make(map[string][]string) // skillName → []filePath
	for _, f := range datedFiles {
		base := filepath.Base(f)
		skillName, _ := parseAutoSkillFilename(base)
		if skillName != "" {
			grouped[skillName] = append(grouped[skillName], f)
		}
	}

	for skillName, files := range grouped {
		c.compactOneSkill(skillName, files)
	}
}

// compactOneSkill 整理单个技能的日期文件为汇总
func (c *MemoryCollector) compactOneSkill(skillName string, datedFiles []string) {
	// 读取所有日期文件内容
	var newEntries strings.Builder
	for _, f := range datedFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("[MemoryCollector] 读取 %s 失败: %v", filepath.Base(f), err)
			continue
		}
		newEntries.WriteString(string(data))
		newEntries.WriteString("\n")
	}

	if newEntries.Len() == 0 {
		return
	}

	// 读取现有汇总文件
	existingSummary := ""
	summaryPath := c.memoryMgr.autoSkillSummaryFilePath(skillName)
	if data, err := os.ReadFile(summaryPath); err == nil {
		existingSummary = string(data)
	}

	// 调用 LLM 整理
	compacted, err := c.llmCompactAutoSkill(existingSummary, newEntries.String(), skillName)
	if err != nil {
		log.Printf("[MemoryCollector] LLM 整理 %s 失败: %v", skillName, err)
		return
	}

	// 写入汇总文件（覆盖）
	content := fmt.Sprintf("# %s 技能经验汇总\n\n%s\n", skillName, compacted)
	if err := os.WriteFile(summaryPath, []byte(content), 0644); err != nil {
		log.Printf("[MemoryCollector] 写入汇总文件 %s 失败: %v", filepath.Base(summaryPath), err)
		return
	}

	// 已整理的日期文件重命名加 .done 后缀
	for _, f := range datedFiles {
		donePath := f + ".done"
		if err := os.Rename(f, donePath); err != nil {
			log.Printf("[MemoryCollector] 重命名 %s → .done 失败: %v", filepath.Base(f), err)
		}
	}

	log.Printf("[MemoryCollector] 整理 %s 完成: %d 个日期文件 → 汇总 %d 字符",
		skillName, len(datedFiles), len(compacted))
}

// llmCompactAutoSkill 调用 LLM 将新经验整合到现有汇总中
func (c *MemoryCollector) llmCompactAutoSkill(existingSummary, newEntries, skillName string) (string, error) {
	var prompt string
	if existingSummary != "" {
		prompt = fmt.Sprintf(`你是技能经验整理助手。请将新的错误经验整合到现有汇总中，去重合并，保持简洁（不超过 500 字）。
只输出整理后的汇总内容，不要输出标题或其他说明。

技能名: %s

现有汇总:
%s

新增经验:
%s`, skillName, existingSummary, newEntries)
	} else {
		prompt = fmt.Sprintf(`你是技能经验整理助手。请从以下错误经验中提取关键模式，整理为简洁的经验汇总（不超过 500 字）。
只输出汇总内容，不要输出标题或其他说明。

技能名: %s

经验记录:
%s`, skillName, newEntries)
	}

	messages := []Message{
		{Role: "system", Content: "你是一个技能经验整理助手，负责将零散的错误经验整合为简洁、实用的使用指南。"},
		{Role: "user", Content: prompt},
	}

	text, _, err := c.bridge.sendLLM(messages, nil)
	if err != nil {
		return "", fmt.Errorf("LLM compact: %v", err)
	}

	return strings.TrimSpace(text), nil
}
