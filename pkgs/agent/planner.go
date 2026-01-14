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
	account string
}

// NewTaskPlanner 创建任务规划器
func NewTaskPlanner(account string) *TaskPlanner {
	return &TaskPlanner{account: account}
}

// PlanningResult 规划结果
type PlanningResult struct {
	Title    string        `json:"title"`
	SubTasks []SubTaskPlan `json:"subtasks"`
}

// SubTaskPlan 子任务规划
type SubTaskPlan struct {
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

// PlanTask 将自然语言分解为可执行任务
func (p *TaskPlanner) PlanTask(userInput string) ([]SubTask, error) {
	log.MessageF(log.ModuleAgent, "Planning task: %s", userInput)

	// 构建规划提示词
	prompt := fmt.Sprintf(`你是一个任务规划助手。当前用户账号是: %s
请将用户请求分解为可执行的子任务。

用户请求: %s

可用的工具列表:
%s

请返回JSON格式（不要包含markdown代码块标记）:
{
  "title": "任务标题（简短描述）",
  "subtasks": [
    {"description": "子任务1描述", "tools": ["工具名1"]},
    {"description": "子任务2描述", "tools": ["工具名2", "工具名3"]}
  ]
}

注意:
1. 每个子任务应该是独立可执行的
2. tools 可以为空数组，表示不需要工具
3. 子任务数量控制在1-5个
4. 描述要具体、可操作
5. 所有工具调用都需要传递 account: "%s" 参数`, p.account, userInput, p.getAvailableToolsDescription(), p.account)

	// 调用 LLM 进行任务分解
	response, err := p.callLLM(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM调用失败: %w", err)
	}

	// 解析结果
	var result PlanningResult
	// 尝试提取 JSON
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.WarnF(log.ModuleAgent, "Failed to parse planning result: %v, response: %s", err, response)
		// 如果解析失败，创建一个简单的单任务
		return []SubTask{
			{
				ID:          generateTaskID(),
				Description: userInput,
				ToolCalls:   []string{},
				Status:      "pending",
			},
		}, nil
	}

	// 转换为 SubTask
	subtasks := make([]SubTask, len(result.SubTasks))
	for i, st := range result.SubTasks {
		subtasks[i] = SubTask{
			ID:          generateTaskID(),
			Description: st.Description,
			ToolCalls:   st.Tools,
			Status:      "pending",
		}
	}

	log.MessageF(log.ModuleAgent, "Task planned: %d subtasks", len(subtasks))
	return subtasks, nil
}

// ExecuteSubTask 执行子任务
func (p *TaskPlanner) ExecuteSubTask(task *AgentTask, subtask *SubTask) (string, error) {
	log.MessageF(log.ModuleAgent, "Executing subtask: %s", subtask.Description)

	// 构建执行提示词 - 始终通过 LLM 执行任务，LLM 会自动选择并调用工具
	prompt := fmt.Sprintf(`执行以下任务并返回结果。当前用户账号是: %s

任务描述: %s

重要:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 如果需要使用工具，请按照工具定义中的参数格式调用
3. 直接执行任务并返回结果`, p.account, subtask.Description, p.account)

	// 通过 LLM 执行任务，LLM 会自动调用所需的工具并传入正确的参数
	response, err := p.callLLM(prompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

// callLLM 调用 LLM API，使用 llm 包的统一接口
func (p *TaskPlanner) callLLM(prompt string) (string, error) {
	log.DebugF(log.ModuleAgent, "LLM call with prompt length: %d", len(prompt))

	// 构建系统提示词
	systemPrompt := fmt.Sprintf(`你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 如果工具需要 date 参数，使用 RawCurrentDate 先获取当前日期
3. 如果用户需要创建提醒，使用 CreateReminder 工具
4. 调用工具时使用正确的参数名，参考工具定义中的 required 字段
5. 调用完工具后返回简单直接的执行结果给用户

你可以帮助用户完成各种任务，如创建提醒、查询博客、分析数据等。`, p.account, p.account)

	// 使用 llm 包的统一接口
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	return llm.SendSyncLLMRequest(messages, p.account)
}

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
