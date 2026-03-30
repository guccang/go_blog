package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ========================= 评估类型定义 =========================

// EvalSeverity 评估严重等级
type EvalSeverity string

const (
	SeverityCritical EvalSeverity = "critical"
	SeverityWarning  EvalSeverity = "warning"
	SeverityInfo     EvalSeverity = "info"
)

// EvalCategory 评估问题类别
type EvalCategory string

const (
	CategoryDuplicateTool    EvalCategory = "duplicate_tool"
	CategoryCoverageGap      EvalCategory = "coverage_gap"
	CategorySemanticAmbiguity EvalCategory = "semantic_ambiguity"
	CategoryKeywordConflict  EvalCategory = "keyword_conflict"
	CategoryDescriptionWeak  EvalCategory = "description_weak"
)

// EvalIssue 单条评估问题
type EvalIssue struct {
	Category    EvalCategory `json:"category"`
	Severity    EvalSeverity `json:"severity"`
	Subject     string       `json:"subject"`
	RelatedTo   string       `json:"related_to,omitempty"`
	Description string       `json:"description"`
	Suggestion  string       `json:"suggestion"`
}

// EvalReport 完整评估报告
type EvalReport struct {
	Timestamp  string      `json:"timestamp"`
	ToolCount  int         `json:"tool_count"`
	SkillCount int         `json:"skill_count"`
	AgentCount int         `json:"agent_count"`
	Issues     []EvalIssue `json:"issues"`
	Summary    string      `json:"summary"`
	DurationMs int64       `json:"duration_ms"`
}

// ToolEvaluator 工具评估器
type ToolEvaluator struct {
	bridge *Bridge
}

// NewToolEvaluator 创建工具评估器
func NewToolEvaluator(bridge *Bridge) *ToolEvaluator {
	return &ToolEvaluator{bridge: bridge}
}

// Evaluate 执行全部 4 个维度检查，返回评估报告
func (te *ToolEvaluator) Evaluate() *EvalReport {
	start := time.Now()

	// 快照数据（加读锁）
	te.bridge.catalogMu.RLock()
	tools := make([]LLMTool, len(te.bridge.llmTools))
	copy(tools, te.bridge.llmTools)
	catalog := make(map[string]string, len(te.bridge.toolCatalog))
	for k, v := range te.bridge.toolCatalog {
		catalog[k] = v
	}
	agentInfo := make(map[string]AgentInfo, len(te.bridge.agentInfo))
	for k, v := range te.bridge.agentInfo {
		agentInfo[k] = v
	}
	te.bridge.catalogMu.RUnlock()

	// 获取 skill 数据
	var skills []SkillEntry
	if te.bridge.skillMgr != nil {
		skills = te.bridge.skillMgr.GetAllSkills()
	}

	report := &EvalReport{
		Timestamp:  time.Now().Format(time.RFC3339),
		ToolCount:  len(tools),
		SkillCount: len(skills),
		AgentCount: len(agentInfo),
	}

	log.Printf("[ToolEvaluator] ========== 开始工具评估 ==========")
	log.Printf("[ToolEvaluator] 评估范围: %d tools, %d skills, %d agents",
		report.ToolCount, report.SkillCount, report.AgentCount)

	// 维度 1：工具重复检测
	report.Issues = append(report.Issues, te.checkDuplicateTools(tools, catalog)...)

	// 维度 2：Skill-Tool 覆盖检测
	report.Issues = append(report.Issues, te.checkSkillToolCoverage(tools, skills)...)

	// 维度 3：语义模糊检测
	report.Issues = append(report.Issues, te.checkSemanticAmbiguity(tools)...)

	// 维度 4：关键词冲突检测
	report.Issues = append(report.Issues, te.checkKeywordConflicts(skills)...)

	report.DurationMs = time.Since(start).Milliseconds()

	// 生成摘要
	var criticalCount, warningCount, infoCount int
	for _, issue := range report.Issues {
		switch issue.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityWarning:
			warningCount++
		case SeverityInfo:
			infoCount++
		}
	}
	report.Summary = fmt.Sprintf("%d critical, %d warning, %d info",
		criticalCount, warningCount, infoCount)

	log.Printf("[ToolEvaluator] ========== 评估完成: %s (%dms) ==========",
		report.Summary, report.DurationMs)

	return report
}

// ========================= 维度 1：工具重复检测 =========================

func (te *ToolEvaluator) checkDuplicateTools(tools []LLMTool, catalog map[string]string) []EvalIssue {
	var issues []EvalIssue

	// 按裸名分组
	type toolEntry struct {
		name    string
		agentID string
		tool    LLMTool
	}
	bareGroups := make(map[string][]toolEntry)
	for _, t := range tools {
		bare := extractBareName(t.Function.Name)
		agentID := catalog[t.Function.Name]
		bareGroups[bare] = append(bareGroups[bare], toolEntry{
			name:    t.Function.Name,
			agentID: agentID,
			tool:    t,
		})
	}

	// 同名来自不同 agent → critical
	for bare, entries := range bareGroups {
		if len(entries) <= 1 {
			continue
		}
		agents := make(map[string]bool)
		var names []string
		for _, e := range entries {
			if e.agentID != "" {
				agents[e.agentID] = true
			}
			names = append(names, e.name)
		}
		if len(agents) > 1 {
			issues = append(issues, EvalIssue{
				Category:    CategoryDuplicateTool,
				Severity:    SeverityCritical,
				Subject:     bare,
				RelatedTo:   strings.Join(names, ", "),
				Description: fmt.Sprintf("同名工具 %s 来自 %d 个不同 agent", bare, len(agents)),
				Suggestion:  "合并或统一注册，避免 LLM 混淆",
			})
		}
	}

	// 两两比较 description 相似度 + 参数 key 重合率
	for i := 0; i < len(tools); i++ {
		for j := i + 1; j < len(tools); j++ {
			nameA := tools[i].Function.Name
			nameB := tools[j].Function.Name
			if extractBareName(nameA) == extractBareName(nameB) {
				continue // 已在上面处理
			}

			descSim := jaccardSimilarity(
				tokenize(tools[i].Function.Description),
				tokenize(tools[j].Function.Description),
			)
			paramSim := compareParamKeys(tools[i].Function.Parameters, tools[j].Function.Parameters)

			if descSim > 0.7 && paramSim > 0.8 {
				issues = append(issues, EvalIssue{
					Category:    CategoryDuplicateTool,
					Severity:    SeverityWarning,
					Subject:     nameA,
					RelatedTo:   nameB,
					Description: fmt.Sprintf("描述相似度 %.2f，参数重合率 %.2f，疑似功能重复", descSim, paramSim),
					Suggestion:  "检查两工具是否功能重复，考虑合并或差异化描述",
				})
			}
		}
	}

	return issues
}

// ========================= 维度 2：Skill-Tool 覆盖检测 =========================

func (te *ToolEvaluator) checkSkillToolCoverage(tools []LLMTool, skills []SkillEntry) []EvalIssue {
	var issues []EvalIssue

	// 在线工具名集合
	onlineTools := make(map[string]bool, len(tools))
	for _, t := range tools {
		onlineTools[t.Function.Name] = true
	}

	// 被 skill 管理的工具集合
	managedTools := make(map[string]bool)

	for _, skill := range skills {
		if len(skill.Tools) == 0 {
			continue
		}

		var online, offline []string
		for _, t := range skill.Tools {
			if onlineTools[t] {
				online = append(online, t)
			} else {
				offline = append(offline, t)
			}
			managedTools[t] = true
		}

		if len(online) == 0 && len(offline) > 0 {
			// 全部工具不在线 → critical
			issues = append(issues, EvalIssue{
				Category:    CategoryCoverageGap,
				Severity:    SeverityCritical,
				Subject:     skill.Name,
				RelatedTo:   strings.Join(offline, ", "),
				Description: fmt.Sprintf("技能 %s 声明的 %d 个工具全部不在线，技能失效", skill.Name, len(offline)),
				Suggestion:  "检查关联 agent 是否已启动，或更新技能的工具配置",
			})
		} else if len(offline) > 0 {
			// 部分工具不在线 → warning
			issues = append(issues, EvalIssue{
				Category:    CategoryCoverageGap,
				Severity:    SeverityWarning,
				Subject:     skill.Name,
				RelatedTo:   strings.Join(offline, ", "),
				Description: fmt.Sprintf("技能 %s 有 %d 个工具不在线: %s", skill.Name, len(offline), strings.Join(offline, ", ")),
				Suggestion:  "检查对应 agent 状态或更新技能工具列表",
			})
		}
	}

	// 在线工具未被任何 skill 管理 → info
	for _, t := range tools {
		if !managedTools[t.Function.Name] {
			issues = append(issues, EvalIssue{
				Category:    CategoryCoverageGap,
				Severity:    SeverityInfo,
				Subject:     t.Function.Name,
				Description: fmt.Sprintf("工具 %s 未被任何 skill 管理", t.Function.Name),
				Suggestion:  "考虑将该工具纳入合适的 skill 中以提升匹配准确率",
			})
		}
	}

	return issues
}

// ========================= 维度 3：语义模糊检测 =========================

// ambiguityCandidate 语义模糊候选对
type ambiguityCandidate struct {
	IndexA    int
	IndexB    int
	NameA     string
	NameB     string
	DescA     string
	DescB     string
	ParamsA   string
	ParamsB   string
	Similarity float64
}

func (te *ToolEvaluator) checkSemanticAmbiguity(tools []LLMTool) []EvalIssue {
	var issues []EvalIssue
	var candidates []ambiguityCandidate

	// 阶段 A：静态检测
	for i, t := range tools {
		// description 过短
		desc := t.Function.Description
		if utf8.RuneCountInString(desc) < 10 {
			issues = append(issues, EvalIssue{
				Category:    CategoryDescriptionWeak,
				Severity:    SeverityWarning,
				Subject:     t.Function.Name,
				Description: fmt.Sprintf("工具描述过短（%d 字符）: \"%s\"", utf8.RuneCountInString(desc), desc),
				Suggestion:  "补充更详细的工具描述，帮助 LLM 准确选择工具",
			})
		}

		// 两两比较 Jaccard > 0.6 → 标记候选对
		for j := i + 1; j < len(tools); j++ {
			sim := jaccardSimilarity(
				tokenize(tools[i].Function.Description),
				tokenize(tools[j].Function.Description),
			)
			if sim > 0.6 {
				candidates = append(candidates, ambiguityCandidate{
					IndexA:     i,
					IndexB:     j,
					NameA:      tools[i].Function.Name,
					NameB:      tools[j].Function.Name,
					DescA:      tools[i].Function.Description,
					DescB:      tools[j].Function.Description,
					ParamsA:    summarizeParams(tools[i].Function.Parameters),
					ParamsB:    summarizeParams(tools[j].Function.Parameters),
					Similarity: sim,
				})
			}
		}
	}

	// 阶段 B：LLM 语义评估（仅候选对 > 0 时调用）
	if len(candidates) > 0 {
		llmIssues := te.llmEvaluateAmbiguity(candidates)
		issues = append(issues, llmIssues...)
	}

	return issues
}

// llmEvaluateAmbiguity 使用 LLM 评估候选对的语义模糊性
func (te *ToolEvaluator) llmEvaluateAmbiguity(candidates []ambiguityCandidate) []EvalIssue {
	var issues []EvalIssue

	// 构建批量评估 prompt
	var sb strings.Builder
	sb.WriteString("你是一个技能工具评估专家。分析以下工具对，判断是否存在语义模糊或功能重复。\n")
	sb.WriteString("每对包含：工具名、描述、参数摘要、静态相似度。\n\n")

	for i, c := range candidates {
		sb.WriteString(fmt.Sprintf("--- 工具对 %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("工具A: %s\n描述A: %s\n参数A: %s\n", c.NameA, c.DescA, c.ParamsA))
		sb.WriteString(fmt.Sprintf("工具B: %s\n描述B: %s\n参数B: %s\n", c.NameB, c.DescB, c.ParamsB))
		sb.WriteString(fmt.Sprintf("静态相似度: %.2f\n\n", c.Similarity))
	}

	sb.WriteString("输出 JSON 数组，每个元素对应一个工具对：\n")
	sb.WriteString(`[{"pair_index":1, "verdict":"duplicate|ambiguous|clear", "confidence":0.9, "reason":"...", "suggestion":"..."}]`)
	sb.WriteString("\n\n只输出 JSON，不要其他内容。")

	messages := []Message{
		{Role: "user", Content: sb.String()},
	}

	cfg := &te.bridge.cfg.LLM
	fallbacks := te.bridge.cfg.Fallbacks
	cooldown := time.Duration(te.bridge.cfg.FallbackCooldownSec) * time.Second

	resp, _, err := SendLLMRequestWithFallback(cfg, fallbacks, cooldown, messages, nil, te.bridge.cfg.Providers)
	if err != nil {
		log.Printf("[ToolEvaluator] LLM 语义评估失败: %v，跳过 LLM 阶段", err)
		// 降级：直接将候选对标记为 warning
		for _, c := range candidates {
			issues = append(issues, EvalIssue{
				Category:    CategorySemanticAmbiguity,
				Severity:    SeverityWarning,
				Subject:     c.NameA,
				RelatedTo:   c.NameB,
				Description: fmt.Sprintf("静态相似度 %.2f，LLM 评估不可用", c.Similarity),
				Suggestion:  "检查两工具是否存在语义模糊，考虑差异化描述",
			})
		}
		return issues
	}

	// 清洗 JSON
	resp = cleanLLMJSON(resp)

	// 解析 LLM 响应
	type llmVerdict struct {
		PairIndex  int     `json:"pair_index"`
		Verdict    string  `json:"verdict"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
		Suggestion string  `json:"suggestion"`
	}

	var verdicts []llmVerdict
	if err := json.Unmarshal([]byte(resp), &verdicts); err != nil {
		log.Printf("[ToolEvaluator] 解析 LLM 评估响应失败: %v (raw: %.200s)", err, resp)
		return issues
	}

	for _, v := range verdicts {
		idx := v.PairIndex - 1 // 1-based → 0-based
		if idx < 0 || idx >= len(candidates) {
			continue
		}
		c := candidates[idx]

		switch v.Verdict {
		case "duplicate":
			issues = append(issues, EvalIssue{
				Category:    CategorySemanticAmbiguity,
				Severity:    SeverityCritical,
				Subject:     c.NameA,
				RelatedTo:   c.NameB,
				Description: fmt.Sprintf("LLM 评估=duplicate (置信度 %.2f): %s", v.Confidence, v.Reason),
				Suggestion:  v.Suggestion,
			})
		case "ambiguous":
			issues = append(issues, EvalIssue{
				Category:    CategorySemanticAmbiguity,
				Severity:    SeverityWarning,
				Subject:     c.NameA,
				RelatedTo:   c.NameB,
				Description: fmt.Sprintf("LLM 评估=ambiguous (置信度 %.2f): %s", v.Confidence, v.Reason),
				Suggestion:  v.Suggestion,
			})
		case "clear":
			// clear 不产生 issue
		default:
			log.Printf("[ToolEvaluator] 未知 verdict: %s for pair %d", v.Verdict, v.PairIndex)
		}
	}

	return issues
}

// ========================= 维度 4：关键词冲突检测 =========================

func (te *ToolEvaluator) checkKeywordConflicts(skills []SkillEntry) []EvalIssue {
	var issues []EvalIssue

	// 构建倒排索引：keyword → 使用该关键词的 skill 列表
	keywordIndex := make(map[string][]string)
	for _, skill := range skills {
		for _, kw := range skill.Keywords {
			kw = strings.ToLower(strings.TrimSpace(kw))
			if kw != "" {
				keywordIndex[kw] = append(keywordIndex[kw], skill.Name)
			}
		}
	}

	// 同一关键词被多个 skill 使用
	for kw, skillNames := range keywordIndex {
		if len(skillNames) > 1 {
			issues = append(issues, EvalIssue{
				Category:    CategoryKeywordConflict,
				Severity:    SeverityWarning,
				Subject:     kw,
				RelatedTo:   strings.Join(skillNames, ", "),
				Description: fmt.Sprintf("关键词 \"%s\" 被 %d 个技能使用: %s", kw, len(skillNames), strings.Join(skillNames, ", ")),
				Suggestion:  "细化关键词，避免多个技能争抢同一关键词",
			})
		}
	}

	// 两 skill 关键词重叠率 > 0.5
	for i := 0; i < len(skills); i++ {
		for j := i + 1; j < len(skills); j++ {
			if len(skills[i].Keywords) == 0 || len(skills[j].Keywords) == 0 {
				continue
			}
			overlap := keywordOverlapRate(skills[i].Keywords, skills[j].Keywords)
			if overlap > 0.5 {
				issues = append(issues, EvalIssue{
					Category:    CategoryKeywordConflict,
					Severity:    SeverityWarning,
					Subject:     skills[i].Name,
					RelatedTo:   skills[j].Name,
					Description: fmt.Sprintf("技能 %s 与 %s 关键词重叠率 %.2f", skills[i].Name, skills[j].Name, overlap),
					Suggestion:  "差异化两个技能的关键词，降低匹配冲突",
				})
			}
		}
	}

	return issues
}

// ========================= 辅助函数 =========================

// tokenize 中英混合分词：中文 bigram，英文空格分割
func tokenize(text string) map[string]bool {
	tokens := make(map[string]bool)
	text = strings.ToLower(text)

	var chineseRunes []rune
	var englishWord strings.Builder

	flushEnglish := func() {
		if englishWord.Len() > 0 {
			tokens[englishWord.String()] = true
			englishWord.Reset()
		}
	}

	flushChinese := func() {
		// 生成 bigram
		for i := 0; i+1 < len(chineseRunes); i++ {
			tokens[string(chineseRunes[i:i+2])] = true
		}
		// 单字也保留（避免极短中文串丢失）
		if len(chineseRunes) == 1 {
			tokens[string(chineseRunes)] = true
		}
		chineseRunes = chineseRunes[:0]
	}

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			flushEnglish()
			chineseRunes = append(chineseRunes, r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			flushChinese()
			englishWord.WriteRune(r)
		} else {
			// 分隔符
			flushEnglish()
			flushChinese()
		}
	}
	flushEnglish()
	flushChinese()

	return tokens
}

// jaccardSimilarity 计算两个 token 集合的 Jaccard 相似度
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}

	intersection := 0
	for k := range a {
		if b[k] {
			intersection++
		}
	}

	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// keywordOverlapRate 关键词重叠率（交集 / 较小集合大小）
func keywordOverlapRate(a, b []string) float64 {
	setA := make(map[string]bool, len(a))
	for _, k := range a {
		setA[strings.ToLower(strings.TrimSpace(k))] = true
	}
	setB := make(map[string]bool, len(b))
	for _, k := range b {
		setB[strings.ToLower(strings.TrimSpace(k))] = true
	}

	intersection := 0
	for k := range setA {
		if setB[k] {
			intersection++
		}
	}

	minSize := len(setA)
	if len(setB) < minSize {
		minSize = len(setB)
	}
	if minSize == 0 {
		return 0
	}

	return float64(intersection) / float64(minSize)
}

// compareParamKeys 比较两个 JSON schema 的参数 key 重合率
func compareParamKeys(a, b json.RawMessage) float64 {
	keysA := extractParamKeys(a)
	keysB := extractParamKeys(b)

	if len(keysA) == 0 && len(keysB) == 0 {
		return 1.0 // 都无参数视为完全匹配
	}
	if len(keysA) == 0 || len(keysB) == 0 {
		return 0
	}

	intersection := 0
	for k := range keysA {
		if keysB[k] {
			intersection++
		}
	}

	minSize := len(keysA)
	if len(keysB) < minSize {
		minSize = len(keysB)
	}

	return float64(intersection) / float64(minSize)
}

// extractParamKeys 从 JSON schema 提取参数 key 集合
func extractParamKeys(params json.RawMessage) map[string]bool {
	if len(params) == 0 {
		return nil
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(params, &schema); err != nil {
		return nil
	}

	keys := make(map[string]bool, len(schema.Properties))
	for k := range schema.Properties {
		if k != "account" { // 跳过通用 account 参数
			keys[k] = true
		}
	}
	return keys
}

// extractBareName 去除 agent 前缀，提取裸名
// 例如: "codegen_ReadFile" → "ReadFile", "deploy.Build" → "Build"
func extractBareName(name string) string {
	// 尝试点号分割（原始名格式 agent.Tool）
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	// 尝试下划线分割（sanitize 后格式 agent_Tool）
	if idx := strings.Index(name, "_"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// summarizeParams 参数 schema 可读摘要
func summarizeParams(params json.RawMessage) string {
	if len(params) == 0 {
		return "(无参数)"
	}

	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(params, &schema); err != nil {
		return "(解析失败)"
	}
	if len(schema.Properties) == 0 {
		return "(无参数)"
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	var parts []string
	for name, prop := range schema.Properties {
		if name == "account" {
			continue
		}
		label := name
		if prop.Description != "" {
			desc := prop.Description
			if len([]rune(desc)) > 30 {
				desc = string([]rune(desc)[:30]) + "..."
			}
			label = fmt.Sprintf("%s(%s)", name, desc)
		}
		if requiredSet[name] {
			label += "[必填]"
		}
		parts = append(parts, label)
	}

	return strings.Join(parts, ", ")
}
