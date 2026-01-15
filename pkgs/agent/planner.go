package agent

import (
	"encoding/json"
	"fmt"
	"llm"
	"mcp"
	log "mylog"
	"strings"
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
func (p *TaskPlanner) PlanNode(node *TaskNode) (*NodePlanningResult, error) {
	log.MessageF(log.ModuleAgent, "Planning node: %s (depth: %d)", node.Title, node.Depth)

	// 构建上下文
	contextStr := node.Context.BuildLLMContext()

	// 构建规划提示词
	prompt := fmt.Sprintf(`你是一个任务规划专家。请将任务分解为可执行的子任务。

## 当前账户
%s

## 任务信息
标题: %s
描述: %s
目标: %s

## 上下文
%s

## 可用工具
%s

## 规则
1. 子任务数量控制在 1-5 个
2. 每个子任务应该是独立可执行的
3. 标记需要进一步拆解的复杂子任务 (can_decompose: true)
4. 明确子任务间的依赖关系 (depends_on: ["依赖的子任务title"])
5. 选择合适的执行模式：
   - sequential: 子任务有依赖，必须按顺序执行
   - parallel: 子任务独立，可以并行执行
6. 最大拆解深度: %d，当前深度: %d
7. 如果任务足够简单可以直接执行，返回空的 subtasks 数组
8. 所有工具调用都需要传递 account: "%s" 参数

## 返回 JSON 格式（无 markdown 标记）
{
  "title": "任务标题",
  "goal": "期望目标",
  "execution_mode": "sequential 或 parallel",
  "subtasks": [
    {
      "title": "子任务标题",
      "description": "详细描述",
      "goal": "子任务目标",
      "tools": ["工具名1", "工具名2"],
      "can_decompose": true或false,
      "depends_on": []
    }
  ],
  "reasoning": "拆解思路说明"
}`,
		p.account,
		node.Title,
		node.Description,
		node.Goal,
		contextStr,
		p.getAvailableToolsDescription(),
		p.maxDepth,
		node.Depth,
		p.account)

	// 调用 LLM
	response, err := p.callPlanningLLM(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM调用失败: %w", err)
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

// ExecuteNode 执行任务节点（新版）
func (p *TaskPlanner) ExecuteNode(node *TaskNode) (*TaskResult, error) {
	log.MessageF(log.ModuleAgent, "Executing node: %s", node.Title)

	// 构建上下文
	contextStr := node.Context.BuildLLMContext()

	// 构建执行提示词
	prompt := fmt.Sprintf(`执行以下任务并返回结果。

## 当前账户
%s

## 任务信息
标题: %s
描述: %s
目标: %s

## 上下文
%s

## 重要规则
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 如果需要使用工具，请按照工具定义中的参数格式调用
3. 直接执行任务并返回结果
4. 返回结果要简洁明了，包含关键信息

## 返回格式
执行完成后，请返回：
1. 执行结果的简要描述
2. 关键数据或信息（如有）`,
		p.account,
		node.Title,
		node.Description,
		node.Goal,
		contextStr,
		p.account)

	// 调用 LLM 执行
	response, err := p.callExecutionLLM(prompt)
	if err != nil {
		return NewTaskResultError(err.Error()), err
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

// callPlanningLLM 调用 LLM 进行任务规划
func (p *TaskPlanner) callPlanningLLM(prompt string) (string, error) {
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

	return llm.SendSyncLLMRequest(messages, p.account)
}

// callExecutionLLM 调用 LLM 执行任务
func (p *TaskPlanner) callExecutionLLM(prompt string) (string, error) {
	log.DebugF(log.ModuleAgent, "Execution LLM call with prompt length: %d", len(prompt))

	systemPrompt := fmt.Sprintf(`你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 如果工具需要 date 参数，使用 RawCurrentDate 先获取当前日期
3. 如果用户需要创建提醒，使用 CreateReminder 工具
4. 调用工具时使用正确的参数名，参考工具定义中的 required 字段
5. 调用完工具后返回简单直接的执行结果给用户

你可以帮助用户完成各种任务，如创建提醒、查询博客、分析数据等。`, p.account, p.account)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	return llm.SendSyncLLMRequest(messages, p.account)
}

// ============================================================================
// 旧版兼容接口（保留向后兼容）
// ============================================================================

// PlanningResult 规划结果（旧版，保留兼容）
type PlanningResult struct {
	Title    string        `json:"title"`
	SubTasks []SubTaskPlan `json:"subtasks"`
}

// SubTaskPlan 子任务规划（旧版，保留兼容）
type SubTaskPlan struct {
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

// PlanTask 将自然语言分解为可执行任务（旧版兼容接口）
func (p *TaskPlanner) PlanTask(userInput string) ([]SubTask, error) {
	log.MessageF(log.ModuleAgent, "Planning task (legacy): %s", userInput)

	// 创建临时节点进行规划
	tempNode := NewTaskNode(p.account, "临时任务", userInput)
	result, err := p.PlanNode(tempNode)
	if err != nil {
		return nil, err
	}

	// 转换为旧版 SubTask
	subtasks := make([]SubTask, len(result.SubTasks))
	for i, st := range result.SubTasks {
		subtasks[i] = SubTask{
			ID:          generateTaskID(),
			Description: st.Description,
			ToolCalls:   st.Tools,
			Status:      "pending",
		}
	}

	return subtasks, nil
}

// ExecuteSubTask 执行子任务（旧版兼容接口）
func (p *TaskPlanner) ExecuteSubTask(task *AgentTask, subtask *SubTask) (string, error) {
	log.MessageF(log.ModuleAgent, "Executing subtask (legacy): %s", subtask.Description)

	// 创建临时节点进行执行
	tempNode := NewTaskNode(p.account, subtask.Description, subtask.Description)
	result, err := p.ExecuteNode(tempNode)
	if err != nil {
		return "", err
	}

	return result.Output, nil
}

// callLLM 调用 LLM API（旧版兼容）
func (p *TaskPlanner) callLLM(prompt string) (string, error) {
	return p.callExecutionLLM(prompt)
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
