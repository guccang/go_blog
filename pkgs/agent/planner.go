package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"llm"
	"mcp"
	log "mylog"
	"statistics"
	"strings"
	"time"
)

// 上下文长度限制
const (
	MaxContextLength = 20000 // 上下文最大长度，超过则保存为博客
)

// TaskPlanner 任务规划器
type TaskPlanner struct {
	account  string
	maxDepth int
}

// NewTaskPlanner 创建任务规划器
func NewTaskPlanner(account string) *TaskPlanner {
	return &TaskPlanner{
		account:  account,
		maxDepth: DefaultMaxDepth,
	}
}

// SetMaxDepth 设置最大递归深度
func (p *TaskPlanner) SetMaxDepth(depth int) {
	p.maxDepth = depth
}

// ============================================================================
// 新版规划结构（支持递归拆解）
// ============================================================================

// NodePlanningResult 节点规划结果
type NodePlanningResult struct {
	Title         string            `json:"title"`
	Goal          string            `json:"goal"`
	ExecutionMode ExecutionMode     `json:"execution_mode"`
	SubTasks      []SubTaskPlanNode `json:"subtasks"`
	Reasoning     string            `json:"reasoning"`
}

// SubTaskPlanNode 子任务规划节点
type SubTaskPlanNode struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Goal         string   `json:"goal"`
	Tools        []string `json:"tools"`
	CanDecompose bool     `json:"can_decompose"`
	DependsOn    []string `json:"depends_on"` // 依赖的子任务 title
}

// PlanNode 规划任务节点（新版，支持递归拆解）
func (p *TaskPlanner) PlanNode(ctx context.Context, node *TaskNode) (*NodePlanningResult, error) {
	log.MessageF(log.ModuleAgent, "Planning node: %s (depth: %d)", node.Title, node.Depth)

	// 构建上下文
	contextStr := node.Context.BuildLLMContext()

	// 使用集中管理的提示词模板
	prompt := BuildNodePlanningPrompt(
		p.account,
		node.Title,
		node.Description,
		node.Goal,
		contextStr,
		p.getAvailableToolsDescription(),
		p.maxDepth,
		node.Depth,
	)

	// 诊断日志：记录上下文和提示词长度
	log.MessageF(log.ModuleAgent, "[规划诊断] 节点: %s, 上下文长度: %d 字符, 提示词长度: %d 字符, 深度: %d",
		node.Title, len(contextStr), len(prompt), node.Depth)

	// 检查上下文是否超长，超长则保存为博客
	if len(contextStr) > MaxContextLength {
		log.WarnF(log.ModuleAgent, "[上下文超长] 节点: '%s', 长度: %d, 阈值: %d, 将保存为博客", node.Title, len(contextStr), MaxContextLength)
		contextStr = p.truncateContextAsBlog(node, contextStr)
		prompt = BuildNodePlanningPrompt(
			p.account,
			node.Title,
			node.Description,
			node.Goal,
			contextStr,
			p.getAvailableToolsDescription(),
			p.maxDepth,
			node.Depth,
		)
		log.MessageF(log.ModuleAgent, "[上下文压缩] 节点: %s, 新上下文长度: %d 字符", node.Title, len(contextStr))
	}

	// 调用 LLM
	startTime := time.Now()
	response, err := p.callPlanningLLM(ctx, prompt)
	duration := time.Since(startTime).Milliseconds()

	// 记录 LLM 交互历史
	if node != nil {
		node.AddLLMInteraction("planning", prompt, response, 0, duration)
	}

	if err != nil {
		log.WarnF(log.ModuleAgent, "[规划失败] 节点: '%s', 错误: %v", node.Title, err)
		return nil, fmt.Errorf("LLM调用失败 (节点: %s): %w", node.Title, err)
	}

	// 解析结果
	var result NodePlanningResult
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.WarnF(log.ModuleAgent, "Failed to parse planning result: %v, response: %s", err, response)
		// 解析失败，返回空结果（直接执行）
		return &NodePlanningResult{
			Title:         node.Title,
			Goal:          node.Goal,
			ExecutionMode: ModeSequential,
			SubTasks:      []SubTaskPlanNode{},
			Reasoning:     "解析失败，直接执行",
		}, nil
	}

	// 验证执行模式
	if result.ExecutionMode != ModeSequential && result.ExecutionMode != ModeParallel {
		result.ExecutionMode = ModeSequential
	}

	log.MessageF(log.ModuleAgent, "Node planned: %d subtasks, mode: %s, reasoning: %s",
		len(result.SubTasks), result.ExecutionMode, result.Reasoning)

	return &result, nil
}

// ============================================================================
// 两阶段工具选择（减少上下文占用）
// ============================================================================

// SelectToolsForTask 根据任务描述选择需要的工具（阶段1）
// 返回选中的工具名称列表，用于第二阶段获取完整定义
func (p *TaskPlanner) SelectToolsForTask(ctx context.Context, taskDescription string) ([]string, error) {
	log.DebugF(log.ModuleAgent, "Selecting tools for task: %s", taskDescription)

	// 获取工具目录
	catalog := mcp.GetToolCatalogFormatted()

	// 使用集中管理的工具选择提示词
	prompt := BuildToolSelectionPrompt(taskDescription, catalog)

	// 调用简单 LLM（不需要 function calling）
	response, err := p.callToolSelectionLLM(ctx, prompt)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Tool selection failed: %v, using all tools", err)
		return nil, err
	}

	// 解析选中的工具
	var selected []string
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &selected); err != nil {
		log.WarnF(log.ModuleAgent, "Failed to parse tool selection: %v, response: %s", err, response)
		return nil, err
	}

	log.MessageF(log.ModuleAgent, "Selected %d tools: %v", len(selected), selected)
	return selected, nil
}

// callToolSelectionLLM 调用 LLM 进行工具选择（简单调用，无 function calling）
func (p *TaskPlanner) callToolSelectionLLM(ctx context.Context, prompt string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: PromptToolSelectionSystem.Template},
		{Role: "user", Content: prompt},
	}

	// 使用无 tools 的简单调用
	return llm.SendSyncLLMRequestNoTools(ctx, messages, p.account)
}

// ExecuteNode 执行任务节点（新版，支持两阶段工具选择）
func (p *TaskPlanner) ExecuteNode(ctx context.Context, node *TaskNode) (*TaskResult, error) {
	log.MessageF(log.ModuleAgent, "Executing node: %s", node.Title)

	// 阶段1: 选择工具（减少上下文占用）
	selectedTools, err := p.SelectToolsForTask(ctx, node.Description)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Tool selection failed, using all tools: %v", err)
		selectedTools = nil // fallback: 使用全部工具
	}

	// 构建上下文
	contextStr := node.Context.BuildLLMContext()

	// 使用集中管理的执行提示词
	prompt := BuildNodeExecutionPrompt(
		p.account,
		node.Title,
		node.Description,
		node.Goal,
		contextStr,
	)

	// 诊断日志：记录执行阶段的上下文和提示词长度
	toolsInfo := "全部"
	if selectedTools != nil {
		toolsInfo = fmt.Sprintf("%d个", len(selectedTools))
	}
	log.MessageF(log.ModuleAgent, "[执行诊断] 节点: %s, 上下文长度: %d 字符, 提示词长度: %d 字符, 工具: %s",
		node.Title, len(contextStr), len(prompt), toolsInfo)

	// 检查上下文是否超长，超长则保存为博客
	if len(contextStr) > MaxContextLength {
		log.WarnF(log.ModuleAgent, "[上下文超长] 节点: '%s', 长度: %d, 阈值: %d, 将保存为博客", node.Title, len(contextStr), MaxContextLength)
		contextStr = p.truncateContextAsBlog(node, contextStr)
		prompt = BuildNodeExecutionPrompt(
			p.account,
			node.Title,
			node.Description,
			node.Goal,
			contextStr,
		)
		log.MessageF(log.ModuleAgent, "[上下文压缩] 节点: %s, 新上下文长度: %d 字符", node.Title, len(contextStr))
	}

	// 阶段2: 调用 LLM 执行（仅使用选中的工具）
	startTime := time.Now()
	response, err := p.callExecutionLLMWithTools(ctx, prompt, selectedTools)
	duration := time.Since(startTime).Milliseconds()

	// 记录 LLM 交互历史
	if node != nil {
		node.AddLLMInteraction("execution", prompt, response, 0, duration)
	}

	if err != nil {
		log.WarnF(log.ModuleAgent, "[执行失败] 节点: '%s', 错误: %v", node.Title, err)
		return NewTaskResultError(fmt.Sprintf("节点 '%s' 执行失败: %v", node.Title, err)), err
	}

	// 创建结果
	result := NewTaskResult(response, p.summarizeResponse(response))
	return result, nil
}

// summarizeResponse 生成响应摘要
func (p *TaskPlanner) summarizeResponse(response string) string {
	// 简单的摘要逻辑：取前 200 个字符
	if len(response) <= 200 {
		return response
	}
	return response[:200] + "..."
}

// ============================================================================
// 结果整合
// ============================================================================

// SynthesizeResults 使用 LLM 整合子任务结果
func (p *TaskPlanner) SynthesizeResults(ctx context.Context, node *TaskNode, childResults string) (string, error) {
	log.MessageF(log.ModuleAgent, "Synthesizing results for node: %s", node.Title)

	prompt := BuildResultSynthesisPrompt(node.Title, node.Goal, childResults)

	messages := []llm.Message{
		{Role: "system", Content: "你是一个结果整合专家，擅长将多个子任务的结果整合为清晰的最终结果。"},
		{Role: "user", Content: prompt},
	}

	startTime := time.Now()
	response, err := llm.SendSyncLLMRequestNoTools(ctx, messages, p.account)
	duration := time.Since(startTime).Milliseconds()

	// 记录 LLM 交互历史
	if node != nil {
		node.AddLLMInteraction("synthesis", prompt, response, 0, duration)
	}

	if err != nil {
		log.WarnF(log.ModuleAgent, "[结果整合失败] 节点: '%s', 错误: %v", node.Title, err)
		return "", err
	}

	return response, nil
}

// ============================================================================
// 用户输入请求支持
// ============================================================================

// InputRequestResult 用户输入请求的结果
type InputRequestResult struct {
	NeedsInput bool   `json:"needs_input"`
	InputType  string `json:"input_type"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	Options    []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"options,omitempty"`
}

// CheckIfNeedsUserInput 检查 LLM 响应是否需要用户输入
func (p *TaskPlanner) CheckIfNeedsUserInput(response string) (*InputRequestResult, bool) {
	// 检查 LLM 响应中的特殊标记
	// LLM 可能会返回类似 [NEEDS_INPUT] 的标记
	if !strings.Contains(response, "[NEEDS_INPUT]") &&
		!strings.Contains(response, "需要用户确认") &&
		!strings.Contains(response, "请用户选择") {
		return nil, false
	}

	// 尝试解析输入请求
	result := &InputRequestResult{
		NeedsInput: true,
		InputType:  "text",
		Title:      "需要确认",
		Message:    response,
	}

	// 如果包含确认关键字
	if strings.Contains(response, "确认") || strings.Contains(response, "是否") {
		result.InputType = "confirm"
		result.Options = []struct {
			Value string `json:"value"`
			Label string `json:"label"`
		}{
			{Value: "yes", Label: "是"},
			{Value: "no", Label: "否"},
		}
	}

	return result, true
}

// ============================================================================
// LLM 调用函数
// ============================================================================

// callPlanningLLM 调用 LLM 进行任务规划
func (p *TaskPlanner) callPlanningLLM(ctx context.Context, prompt string) (string, error) {
	log.DebugF(log.ModuleAgent, "Planning LLM call with prompt length: %d", len(prompt))

	systemPrompt := fmt.Sprintf(`你是一个任务规划专家。你的职责是将复杂任务分解为可执行的子任务。

重要规则:
1. 分析任务的复杂度和依赖关系
2. 选择合适的执行模式（串行/并行）
3. 标记需要进一步拆解的复杂子任务
4. 返回严格的 JSON 格式
5. 当前用户账号: %s`, p.account)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	return llm.SendSyncLLMRequestWithContext(ctx, messages, p.account)
}

// callExecutionLLM 调用 LLM 执行任务
func (p *TaskPlanner) callExecutionLLM(ctx context.Context, prompt string) (string, error) {
	log.DebugF(log.ModuleAgent, "Execution LLM call with prompt length: %d", len(prompt))

	systemPrompt := fmt.Sprintf(`你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 调用工具时使用正确的参数名，参考工具定义中的 required 字段
3. 调用完工具后返回简单直接的执行结果给用户

你可以帮助用户完成各种任务，如创建提醒、查询博客、分析数据等。`, p.account, p.account)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	return llm.SendSyncLLMRequestWithContext(ctx, messages, p.account)
}

// callExecutionLLMWithTools 调用 LLM 执行任务（使用指定的工具，减少上下文占用）
func (p *TaskPlanner) callExecutionLLMWithTools(ctx context.Context, prompt string, selectedTools []string) (string, error) {
	log.DebugF(log.ModuleAgent, "Execution LLM call with %d selected tools", len(selectedTools))

	systemPrompt := fmt.Sprintf(`你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 调用工具时使用正确的参数名
3. 调用完工具后返回简单直接的执行结果给用户`, p.account, p.account)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	return llm.SendSyncLLMRequestWithSelectedTools(ctx, messages, p.account, selectedTools)
}

// ============================================================================
// 旧版兼容接口（保留向后兼容）
// ============================================================================

// PlanningResult 规划结果（旧版，保留兼容）
type PlanningResult struct {
	Title string `json:"title"`
}

// ============================================================================
// 工具函数
// ============================================================================

// getAvailableToolsDescription 获取可用工具描述
func (p *TaskPlanner) getAvailableToolsDescription() string {
	tools := mcp.GetAvailableLLMTools(nil)
	var descriptions []string
	for _, tool := range tools {
		descriptions = append(descriptions, fmt.Sprintf("- %s: %s",
			tool.Function.Name, tool.Function.Description))
	}
	return strings.Join(descriptions, "\n")
}

// truncateContextAsBlog 将超长上下文保存为博客，返回简短摘要+链接
func (p *TaskPlanner) truncateContextAsBlog(node *TaskNode, contextStr string) string {
	// 生成博客标题
	timestamp := time.Now().Format("20060102_150405")
	nodeTitle := node.Title
	if len(nodeTitle) > 15 {
		nodeTitle = nodeTitle[:15]
	}
	blogTitle := fmt.Sprintf("Agent上下文_%s_%s", nodeTitle, timestamp)

	// 保存为私有博客
	result := statistics.RawCreateBlog(
		node.Account,
		blogTitle,
		contextStr,
		"Agent|上下文|自动保存",
		1, // 私有
		0, // 不加密
	)

	log.MessageF(log.ModuleAgent, "[上下文保存] 节点: '%s', 博客: '%s', 保存结果: %s", node.Title, blogTitle, result)

	// 生成简短摘要
	link := fmt.Sprintf("[%s](/get?blogname=%s)", blogTitle, blogTitle)

	// 提取关键信息作为摘要
	summary := fmt.Sprintf("## 上下文参考\n完整上下文已保存: %s\n\n", link)

	// 保留原始用户请求部分（通常在开头）
	if idx := strings.Index(contextStr, "## 父任务执行结果"); idx > 0 && idx < 2000 {
		summary += contextStr[:idx]
	} else if len(contextStr) > 1000 {
		summary += "（原始请求）\n" + contextStr[:1000] + "...\n"
	}

	// 添加提示
	summary += fmt.Sprintf("\n> 注意: 完整上下文(%d字符)已保存为博客，请点击链接查看详情。\n", len(contextStr))

	return summary
}

// extractJSON 从响应中提取 JSON
func extractJSON(response string) string {
	// 去除可能的 markdown 代码块标记
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	}
	if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}
	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}

	// 尝试找到 JSON 对象
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start != -1 && end != -1 && end > start {
		return response[start : end+1]
	}

	return response
}
