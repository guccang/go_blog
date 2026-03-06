package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ========================= 任务计划数据模型 =========================

// TaskPlan 任务执行计划
type TaskPlan struct {
	SubTasks      []SubTaskPlan `json:"subtasks"`
	ExecutionMode string        `json:"execution_mode"` // sequential / parallel / dag
	Reasoning     string        `json:"reasoning"`
}

// SubTaskPlan 子任务计划
type SubTaskPlan struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on"`
	ToolsHint   []string `json:"tools_hint,omitempty"` // 提示可能用到的工具
}

// ========================= 规划器 =========================

// planAndExecuteTool 虚拟工具定义（注入到 LLM 工具列表中）
var planAndExecuteTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "plan_and_execute",
		Description: "当任务需要多个步骤、有前后依赖关系时，调用此工具进行任务拆解和编排执行。适用于：需要先获取数据再分析、需要处理多个独立子目标、步骤超过3步且有依赖等复杂场景。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"reasoning": {
					"type": "string",
					"description": "说明为什么需要拆解任务，预期包含哪些步骤"
				}
			},
			"required": ["reasoning"]
		}`),
	},
}

// PlanTask 调用 LLM 生成结构化任务计划
func PlanTask(cfg *LLMConfig, query string, tools []LLMTool, account string, maxSubTasks int) (*TaskPlan, error) {
	log.Printf("[Planner] ▶ 开始规划 query=%s account=%s maxSubTasks=%d availableTools=%d",
		truncate(query, 100), account, maxSubTasks, len(tools))
	// 构建工具目录（name + description + 核心参数，帮助 LLM 精确规划）
	var toolCatalog strings.Builder
	for i, tool := range tools {
		// 跳过虚拟工具
		if tool.Function.Name == "plan_and_execute" {
			continue
		}
		// 提取核心参数信息
		paramInfo := extractParamInfo(tool.Function.Parameters)
		if paramInfo != "" {
			toolCatalog.WriteString(fmt.Sprintf("- %s: %s [参数: %s]\n", tool.Function.Name, tool.Function.Description, paramInfo))
		} else {
			toolCatalog.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
		}
		if i > 50 {
			toolCatalog.WriteString("... (更多工具省略)\n")
			break
		}
	}

	planPrompt := fmt.Sprintf(`你是一个任务规划专家。请分析用户的请求，将其拆解为可执行的子任务。

## 用户信息
当前用户账号: %s
当前日期: %s

## 用户请求
%s

## 可用工具
%s

## 规划要求
1. 每个子任务必须是独立的执行单元，可以通过工具调用完成
2. 正确标注子任务之间的依赖关系（depends_on 引用其他子任务的 id）
3. 子任务数量不超过 %d 个
4. 每个子任务的描述要清晰，包含足够的上下文让 AI 独立执行
5. tools_hint 列出该子任务可能需要的工具名称
6. 子任务描述中必须包含用户账号（account=%s），不要再向用户询问账号信息
7. 异步操作（如编码 CodegenStartSession、部署 CodegenStartDeploy）会自动推送进度和结果，不需要额外的"检查状态"或"发送通知"子任务
8. CodegenStartSession 会在项目不存在时自动创建项目，通常不需要单独的 CodegenCreateProject 步骤
9. 编码和部署是两个核心步骤，不要将"创建项目""检查状态""发送通知"拆为独立子任务

## 输出格式
仅返回 JSON，不要其他文字：
{
  "subtasks": [
    {
      "id": "t1",
      "title": "子任务标题",
      "description": "详细描述，包含执行目标和所需参数",
      "depends_on": [],
      "tools_hint": ["ToolName1"]
    },
    {
      "id": "t2",
      "title": "子任务标题",
      "description": "详细描述，可以引用 t1 的结果",
      "depends_on": ["t1"],
      "tools_hint": ["ToolName2"]
    }
  ],
  "execution_mode": "dag",
  "reasoning": "拆解理由和执行顺序说明"
}`, account, time.Now().Format("2006-01-02"), query, toolCatalog.String(), maxSubTasks, account)

	messages := []Message{
		{Role: "user", Content: planPrompt},
	}

	planStart := time.Now()
	resp, _, err := SendLLMRequest(cfg, messages, nil)
	if err != nil {
		log.Printf("[Planner] ✗ LLM规划失败 duration=%v error=%v", time.Since(planStart), err)
		return nil, fmt.Errorf("LLM planning failed: %v", err)
	}
	log.Printf("[Planner] ← LLM规划响应 duration=%v responseLen=%d", time.Since(planStart), len(resp))

	// 解析 JSON 响应
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var plan TaskPlan
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		return nil, fmt.Errorf("parse plan JSON failed: %v (raw: %s)", err, resp)
	}

	// 校验计划
	if len(plan.SubTasks) == 0 {
		return nil, fmt.Errorf("plan has no subtasks")
	}
	if len(plan.SubTasks) > maxSubTasks {
		plan.SubTasks = plan.SubTasks[:maxSubTasks]
	}

	// 校验依赖引用合法性
	idSet := make(map[string]bool)
	for _, st := range plan.SubTasks {
		idSet[st.ID] = true
	}
	for _, st := range plan.SubTasks {
		for _, dep := range st.DependsOn {
			if !idSet[dep] {
				log.Printf("[Planner] warn: subtask %s depends on unknown %s, removing", st.ID, dep)
			}
		}
	}

	log.Printf("[Planner] generated plan with %d subtasks, mode=%s", len(plan.SubTasks), plan.ExecutionMode)
	return &plan, nil
}

// MakeFailureDecision 子任务失败后调用 LLM 决策
func MakeFailureDecision(cfg *LLMConfig, subtask SubTaskPlan, errorMsg string, completedResults map[string]string) (*FailureDecision, error) {
	log.Printf("[Planner] ▶ 失败决策 subtask=%s error=%s", subtask.ID, truncate(errorMsg, 100))
	// 构建上下文
	var context strings.Builder
	context.WriteString(fmt.Sprintf("子任务 [%s] %s 执行失败\n", subtask.ID, subtask.Title))
	context.WriteString(fmt.Sprintf("错误信息: %s\n", errorMsg))
	context.WriteString(fmt.Sprintf("任务描述: %s\n", subtask.Description))

	if len(completedResults) > 0 {
		context.WriteString("\n已完成的兄弟任务结果:\n")
		for id, result := range completedResults {
			// 截断过长结果
			if len(result) > 500 {
				result = result[:500] + "..."
			}
			context.WriteString(fmt.Sprintf("- %s: %s\n", id, result))
		}
	}

	decisionPrompt := fmt.Sprintf(`%s

请决定下一步操作，仅返回 JSON：
{
  "action": "retry" 或 "skip" 或 "abort" 或 "modify",
  "reason": "决策理由",
  "modifications": "如果 action 是 modify，填写修改后的任务描述；否则留空"
}

决策指南：
- retry: 可能是临时错误（超时、网络问题），重试一次
- modify: 任务描述有问题或参数不对，修改后重新执行
- skip: 该子任务非关键，跳过不影响最终结果
- abort: 该子任务是关键步骤，失败后无法继续`, context.String())

	messages := []Message{
		{Role: "user", Content: decisionPrompt},
	}

	resp, _, err := SendLLMRequest(cfg, messages, nil)
	if err != nil {
		// LLM 调用失败，默认 skip
		return &FailureDecision{
			SubTaskID: subtask.ID,
			Action:    "skip",
			Reason:    fmt.Sprintf("LLM decision failed: %v, defaulting to skip", err),
			Timestamp: time.Now(),
		}, nil
	}

	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var decision struct {
		Action        string `json:"action"`
		Reason        string `json:"reason"`
		Modifications string `json:"modifications"`
	}
	if err := json.Unmarshal([]byte(resp), &decision); err != nil {
		return &FailureDecision{
			SubTaskID: subtask.ID,
			Action:    "skip",
			Reason:    fmt.Sprintf("parse decision failed: %v, defaulting to skip", err),
			Timestamp: time.Now(),
		}, nil
	}

	// 校验 action
	switch decision.Action {
	case "retry", "skip", "abort", "modify":
		// valid
	default:
		decision.Action = "skip"
		decision.Reason = "unknown action, defaulting to skip"
	}

	return &FailureDecision{
		SubTaskID:     subtask.ID,
		Action:        decision.Action,
		Reason:        decision.Reason,
		Modifications: decision.Modifications,
		Timestamp:     time.Now(),
	}, nil
}

// extractParamInfo 从工具的 Parameters JSON schema 中提取核心参数描述
// 返回格式如: "project(必填), deploy_target(部署目标), port(可选)"
func extractParamInfo(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}

	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(params, &schema); err != nil {
		return ""
	}
	if len(schema.Properties) == 0 {
		return ""
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	// 跳过 account（几乎所有工具都有，冗余信息）
	var parts []string
	for name, prop := range schema.Properties {
		if name == "account" {
			continue
		}
		label := name
		if prop.Description != "" {
			// 使用简短描述
			desc := prop.Description
			if len([]rune(desc)) > 15 {
				desc = string([]rune(desc)[:15])
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
