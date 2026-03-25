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
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	DependsOn   []string               `json:"depends_on"`
	ToolsHint   []string               `json:"tools_hint,omitempty"`  // 提示可能用到的工具
	ToolParams  map[string]interface{} `json:"tool_params,omitempty"` // 预期工具调用参数
}

// ========================= 规划器 =========================

// planAndExecuteTool 虚拟工具定义（注入到 LLM 工具列表中）
var planAndExecuteTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "plan_and_execute",
		Description: "任务拆解与编排执行。收到用户任务后必须先评估是否需要调用此工具。必须使用的场景：需要 2 个以上工具获取数据后综合处理、多步骤依赖流程（编码→部署、查数据→分析→报告）、多个可并行的独立子目标。优势：子任务独立会话上下文干净、无依赖子任务自动并行、失败可独立重试。单一工具调用或简单查询不要使用。",
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
// completedWork: 之前简单路径中已完成的工具调用摘要（可为空）
// skillBlock: 匹配到的 skill 领域指引（可为空）
func PlanTask(cfg *LLMConfig, query string, tools []LLMTool, account string, maxSubTasks int, completedWork string, skillBlock string, fallbacks []LLMConfig, cooldown time.Duration) (*TaskPlan, error) {
	log.Printf("[Planner] ▶ 开始规划 query=%s account=%s maxSubTasks=%d availableTools=%d completedWork=%v",
		truncate(query, 100), account, maxSubTasks, len(tools), completedWork != "")
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

	// 构建已完成工作上下文（如果有）
	var completedSection string
	if completedWork != "" {
		completedSection = fmt.Sprintf(`
## 已完成的工作（之前已执行过的工具调用，不要重复执行）
%s
重要：上述工具调用已经执行完毕。请只规划尚未完成的剩余工作。已成功的步骤不要再作为子任务。
`, completedWork)
	}

	// 构建领域指引（来自 skill 匹配）
	var skillSection string
	if skillBlock != "" {
		skillSection = fmt.Sprintf(`
## 领域指引
%s
`, skillBlock)
	}

	planPrompt := fmt.Sprintf(`你是一个任务规划专家。请分析用户的请求，将其拆解为最少数量的可执行子任务。
重要：只返回 JSON，不要输出任何解释文字。

## 用户信息
当前用户账号: %s
当前日期: %s

## 用户请求
%s
%s%s
## 可用工具
%s

## 核心规划原则

### 1. 优先使用 skill 技能（最重要）
- 查看"领域指引"中的可用 skill，任务匹配时必须使用对应 skill 的工具
- 编码开发任务（编写代码、创建项目、修复bug、开发功能）→ 使用 coding skill 的工具（AcpStartSession 等）
- 部署上线任务（部署项目、配置服务）→ 使用 deploy skill 的工具（DeployProject/DeployAdhoc 等）
- Skill 有独立会话管理和专业执行策略，比 ExecuteCode/Bash 直接操作更可靠
- 禁止用 ExecuteCode 或 Bash 替代已有 skill 的功能

### 2. ExecuteCode 用于数据处理和脚本操作
- 适用场景：数据获取+分析、批量查询+统计、文件处理等无对应 skill 的任务
- 用一个 ExecuteCode 子任务编写 Python 代码，在代码内通过 call_tool() 批量调用数据工具并完成分析
- 不要拆分为"获取数据A""获取数据B""分析数据"等多个子任务，应合并为 1 个 ExecuteCode

### 3. 最大化并行执行
- 没有数据依赖的子任务必须设置为并行（depends_on 为空）
- 只有真正需要前一步结果的子任务才设置 depends_on

### 4. 精简子任务数量
- 目标：用最少的子任务完成任务（通常 2-3 个）
- 能用 1 个 ExecuteCode 完成的数据处理任务不要拆成多个

## 其他要求
1. 每个子任务描述要包含足够上下文让 AI 独立执行
2. tools_hint 列出该子任务需要的工具名（必须使用可用工具中的实际工具名）
3. 子任务描述中包含用户账号（account=%s）
4. "同步等待完成"的工具不需要额外的"检查状态"子任务
5. 子任务数量不超过 %d 个
6. 编码任务中 AcpStartSession 的 project 参数必须使用描述性项目名（如 helloworld-web），禁止使用 account 作为项目名
7. 编码→部署流程中，部署子任务描述需明确说明："使用前置编码任务返回的 project_dir 和 project 名称调用 DeployProject"
8. **子任务描述隔离**：每个子任务描述只包含该子任务自身需要完成的工作，严禁带入其他子任务的指令。例如用户请求"编码xx然后部署到yy"，编码子任务描述只写编码需求，不要提及"部署到yy"；部署子任务只写部署需求。AcpStartSession 的 prompt 参数同理，只传编码相关内容

## 输出格式
仅返回 JSON（不要包含 markdown 代码块标记）：
{
  "subtasks": [
    {
      "id": "t1",
      "title": "编写网页应用",
      "description": "使用 AcpStartSession 创建项目并编写网页应用代码，project=helloworld-web，account=xxx",
      "depends_on": [],
      "tools_hint": ["AcpStartSession"]
    },
    {
      "id": "t2",
      "title": "部署到服务器",
      "description": "使用前置编码任务返回的 project_dir 和 project 名称调用 DeployProject 部署到目标服务器，account=xxx",
      "depends_on": ["t1"],
      "tools_hint": ["DeployProject"]
    }
  ],
  "execution_mode": "dag",
  "reasoning": "编码用 coding skill，部署用 deploy skill，有依赖关系顺序执行"
}`, account, time.Now().Format("2006-01-02"), query, completedSection, skillSection, toolCatalog.String(), account, maxSubTasks)

	messages := []Message{
		{Role: "user", Content: planPrompt},
	}

	planStart := time.Now()
	var resp string
	var err error
	if len(fallbacks) > 0 {
		resp, _, err = SendLLMRequestWithFallback(cfg, fallbacks, cooldown, messages, nil)
	} else {
		resp, _, err = SendLLMRequest(cfg, messages, nil)
	}
	if err != nil {
		log.Printf("[Planner] ✗ LLM规划失败 duration=%v error=%v", time.Since(planStart), err)
		return nil, fmt.Errorf("LLM planning failed: %v", err)
	}
	log.Printf("[Planner] ← LLM规划响应 duration=%v responseLen=%d", time.Since(planStart), len(resp))

	// 解析 JSON 响应
	resp = cleanLLMJSON(resp)

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
func MakeFailureDecision(cfg *LLMConfig, subtask SubTaskPlan, errorMsg string, completedResults map[string]string, fallbacks []LLMConfig, cooldown time.Duration) (*FailureDecision, error) {
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
- retry: 临时错误（超时、网络问题、agent_offline），重试一次即可
- modify: 参数错误或代码错误。Python 语法/运行时错误（syntax error、TypeError、KeyError 等）必须选 modify，在 modifications 中修正代码或参数后重新执行
- skip: 非关键子任务，跳过不影响最终结果
- abort: 关键步骤失败且无法修复，后续子任务无法继续`, context.String())

	messages := []Message{
		{Role: "user", Content: decisionPrompt},
	}

	var resp string
	var err error
	if len(fallbacks) > 0 {
		resp, _, err = SendLLMRequestWithFallback(cfg, fallbacks, cooldown, messages, nil)
	} else {
		resp, _, err = SendLLMRequest(cfg, messages, nil)
	}
	if err != nil {
		// LLM 调用失败，默认 skip
		return &FailureDecision{
			SubTaskID: subtask.ID,
			Action:    "skip",
			Reason:    fmt.Sprintf("LLM decision failed: %v, defaulting to skip", err),
			Timestamp: time.Now(),
		}, nil
	}

	resp = cleanLLMJSON(resp)

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

// ========================= 动态计划修订 =========================

// PlanRevisionResult 计划修订评估结果
type PlanRevisionResult struct {
	Action string    `json:"action"` // "continue" | "revise"
	Reason string    `json:"reason"`
	Plan   *TaskPlan `json:"plan,omitempty"`
}

// EvaluateAndRevisePlan 评估已完成结果，决定是否修订剩余计划
func EvaluateAndRevisePlan(
	cfg *LLMConfig,
	originalQuery string,
	currentPlan *TaskPlan,
	completedResults map[string]string,
	remainingSubTasks []SubTaskPlan,
	tools []LLMTool,
	account string,
	fallbacks []LLMConfig,
	cooldown time.Duration,
) (*PlanRevisionResult, error) {
	log.Printf("[Planner] ▶ 评估计划修订 completed=%d remaining=%d", len(completedResults), len(remainingSubTasks))

	// 构建已完成结果摘要
	var completedSummary strings.Builder
	for id, result := range completedResults {
		if len(result) > 500 {
			result = result[:500] + "..."
		}
		completedSummary.WriteString(fmt.Sprintf("- %s: %s\n", id, result))
	}

	// 构建剩余子任务摘要
	var remainingSummary strings.Builder
	for _, st := range remainingSubTasks {
		remainingSummary.WriteString(fmt.Sprintf("- %s: %s (depends: %v)\n", st.ID, st.Title, st.DependsOn))
	}

	// 构建工具目录
	var toolCatalog strings.Builder
	for i, tool := range tools {
		if tool.Function.Name == "plan_and_execute" {
			continue
		}
		toolCatalog.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
		if i > 30 {
			toolCatalog.WriteString("... (更多工具省略)\n")
			break
		}
	}

	revisionPrompt := fmt.Sprintf(`你是一个任务计划评估专家。请根据已完成的子任务结果，评估剩余计划是否需要调整。

## 用户原始请求
%s

## 已完成子任务结果
%s

## 剩余待执行子任务
%s

## 可用工具
%s

## 评估规则
1. **偏向 continue**：只有当已完成结果揭示了必须调整的新信息时才选择 revise
2. revise 的场景：
   - 已完成结果表明原计划的假设不成立（如数据格式不同、接口不存在等）
   - 发现需要额外步骤才能完成用户请求
   - 剩余子任务的参数需要根据已完成结果修正
3. 不需要 revise 的场景：
   - 一切按计划进行
   - 小的参数调整可以在子任务执行时自行处理

## 输出格式
仅返回 JSON：
- 继续执行：{"action": "continue", "reason": "理由"}
- 需要修订：{"action": "revise", "reason": "修订原因", "plan": {"subtasks": [...], "execution_mode": "dag", "reasoning": "..."}}

注意：revise 时返回完整新计划，保留已完成任务 ID（不要重复执行），新增/修改/删除剩余任务。`, originalQuery, completedSummary.String(), remainingSummary.String(), toolCatalog.String())

	messages := []Message{
		{Role: "user", Content: revisionPrompt},
	}

	evalStart := time.Now()
	var resp string
	var err error
	if len(fallbacks) > 0 {
		resp, _, err = SendLLMRequestWithFallback(cfg, fallbacks, cooldown, messages, nil)
	} else {
		resp, _, err = SendLLMRequest(cfg, messages, nil)
	}
	if err != nil {
		log.Printf("[Planner] ✗ 计划修订评估失败 duration=%v error=%v", time.Since(evalStart), err)
		return &PlanRevisionResult{Action: "continue", Reason: fmt.Sprintf("评估失败: %v, 继续执行", err)}, nil
	}
	log.Printf("[Planner] ← 修订评估响应 duration=%v responseLen=%d", time.Since(evalStart), len(resp))

	resp = cleanLLMJSON(resp)

	var result PlanRevisionResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		log.Printf("[Planner] warn: parse revision result failed: %v, defaulting to continue", err)
		return &PlanRevisionResult{Action: "continue", Reason: "解析失败，继续执行"}, nil
	}

	if result.Action != "continue" && result.Action != "revise" {
		result.Action = "continue"
		result.Reason = "未知动作，继续执行"
	}

	if result.Action == "revise" && result.Plan != nil {
		if len(result.Plan.SubTasks) == 0 {
			log.Printf("[Planner] warn: revised plan has no subtasks, keeping original")
			result.Action = "continue"
			result.Reason = "修订后计划为空，保持原计划"
			result.Plan = nil
		}
	}

	log.Printf("[Planner] ✓ 修订评估: action=%s reason=%s", result.Action, result.Reason)
	return &result, nil
}

// ========================= 计划审查 =========================

// PlanReview 计划审查结果
type PlanReview struct {
	Action string    `json:"action"` // "execute" | "optimize"
	Reason string    `json:"reason"`
	Plan   *TaskPlan `json:"plan,omitempty"` // action=optimize 时返回修改后的计划
}

// ReviewPlan LLM 审查任务计划，检查参数是否正确、子任务是否合理
// agentCapabilities: agent 能力描述（可用模型/编码工具等），帮助审查参数有效性
func ReviewPlan(cfg *LLMConfig, query string, plan *TaskPlan, tools []LLMTool, account string, agentCapabilities string, fallbacks []LLMConfig, cooldown time.Duration) (*PlanReview, error) {
	log.Printf("[Planner] ▶ 审查计划 subtasks=%d", len(plan.SubTasks))

	// 序列化当前计划
	planJSON, _ := json.MarshalIndent(plan, "", "  ")

	// 只发送计划中引用的工具 schema（而非全量），节省 token
	hintSet := make(map[string]bool)
	for _, st := range plan.SubTasks {
		for _, h := range st.ToolsHint {
			hintSet[h] = true
			hintSet[sanitizeToolName(h)] = true
		}
	}

	var toolSchemas strings.Builder
	for _, tool := range tools {
		if tool.Function.Name == "plan_and_execute" {
			continue
		}
		if len(hintSet) > 0 && !hintSet[tool.Function.Name] && !hintSet[unsanitizeToolName(tool.Function.Name)] {
			continue
		}
		toolSchemas.WriteString(fmt.Sprintf("- %s: %s\n  参数schema: %s\n",
			tool.Function.Name, tool.Function.Description, string(tool.Function.Parameters)))
	}

	// 构建 agent 能力上下文（可选）
	var capabilitiesSection string
	if agentCapabilities != "" {
		capabilitiesSection = fmt.Sprintf("\n## Agent 能力信息\n%s\n", agentCapabilities)
	}

	reviewPrompt := fmt.Sprintf(`你是一个任务计划审查专家。请审查以下任务计划，确认是否可以直接执行。

## 用户原始请求
%s

## 当前任务计划
%s

## 相关工具参数定义
%s
%s
## 审查要点（按优先级排序）

### 1. 工具参数完整性（最重要）
- 对照用户原始请求和工具参数schema，检查子任务描述中是否遗漏了用户指定的参数
- 例如：用户说"使用deepseek模型"，但子任务描述中未提到model参数 → 必须补充
- 例如：用户说"用opencode"，但子任务描述中未提到tool参数 → 必须补充
- 工具的可选参数如果用户明确指定了值，则必须在子任务描述中体现

### 2. 子任务结构
- 子任务是否有遗漏或冗余（能合并为 ExecuteCode 的是否已合并）
- 依赖关系是否合理（独立的子任务是否正确设置为并行，即 depends_on 为空）
- 子任务描述是否包含足够上下文让 AI 独立执行
- "同步等待完成"的工具是否有冗余的状态检查子任务（应删除）

## 输出格式
仅返回 JSON：
- 计划合理：{"action": "execute", "reason": "审查通过的理由"}
- 需要优化：{"action": "optimize", "reason": "原因", "plan": {优化后的完整计划JSON}}

注意：参数遗漏必须通过 optimize 修复，将缺失的参数补充到子任务 description 中。结构性小问题可以在执行时自行修正。`,
		query, string(planJSON), toolSchemas.String(), capabilitiesSection)

	messages := []Message{
		{Role: "user", Content: reviewPrompt},
	}

	reviewStart := time.Now()
	var resp string
	var err error
	if len(fallbacks) > 0 {
		resp, _, err = SendLLMRequestWithFallback(cfg, fallbacks, cooldown, messages, nil)
	} else {
		resp, _, err = SendLLMRequest(cfg, messages, nil)
	}
	if err != nil {
		log.Printf("[Planner] ✗ 计划审查失败 duration=%v error=%v", time.Since(reviewStart), err)
		return nil, fmt.Errorf("plan review failed: %v", err)
	}
	log.Printf("[Planner] ← 审查响应 duration=%v responseLen=%d", time.Since(reviewStart), len(resp))

	resp = cleanLLMJSON(resp)

	var review PlanReview
	if err := json.Unmarshal([]byte(resp), &review); err != nil {
		log.Printf("[Planner] warn: parse review failed: %v, defaulting to execute", err)
		return &PlanReview{Action: "execute", Reason: "审查响应解析失败，直接执行"}, nil
	}

	// 校验 action
	if review.Action != "execute" && review.Action != "optimize" {
		review.Action = "execute"
		review.Reason = "未知审查动作，直接执行"
	}

	// optimize 时校验返回的计划
	if review.Action == "optimize" && review.Plan != nil {
		if len(review.Plan.SubTasks) == 0 {
			log.Printf("[Planner] warn: optimized plan has no subtasks, keeping original")
			review.Action = "execute"
			review.Reason = "优化后计划为空，保持原计划执行"
			review.Plan = nil
		}
	}

	log.Printf("[Planner] ✓ 审查结果: action=%s reason=%s", review.Action, review.Reason)
	return &review, nil
}

// cleanLLMJSON 清理 LLM 返回的 JSON 响应，移除 think 标签和 markdown 代码块标记
func cleanLLMJSON(s string) string {
	s = strings.TrimSpace(s)

	// 移除 <think>...</think> 标签
	if idx := strings.Index(s, "<think>"); idx >= 0 {
		if endIdx := strings.Index(s[idx:], "</think>"); endIdx >= 0 {
			s = s[:idx] + s[idx+endIdx+8:]
		}
	}
	s = strings.TrimSpace(s)

	// 移除开头的 ```json / ``` 等代码块标记（整行移除，兼容 ```json、```JSON 等变体）
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		} else {
			s = strings.TrimPrefix(s, "```json")
			s = strings.TrimPrefix(s, "```")
		}
	}

	// 移除结尾的 ```
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	s = strings.TrimSpace(s)

	// 兜底：如果清理后仍不是以 { 开头，尝试提取第一个 JSON 对象
	if len(s) > 0 && s[0] != '{' {
		start := strings.Index(s, "{")
		end := strings.LastIndex(s, "}")
		if start >= 0 && end > start {
			s = s[start : end+1]
		}
	}

	return strings.TrimSpace(s)
}

// extractParamInfo 从工具的 Parameters JSON schema 中提取核心参数描述
// 返回格式如: "project(项目名称)[必填], model(模型配置名称（可选）), tool(编码工具（可选，claudecode/opencode）)"
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
			// 保留足够长度以包含参数的合法值信息（如 claudecode/opencode）
			desc := prop.Description
			if len([]rune(desc)) > 40 {
				desc = string([]rune(desc)[:40])
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
