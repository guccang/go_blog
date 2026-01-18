package agent

import "fmt"

// ============================================================================
// 提示词管理模块
// 所有 LLM 提示词集中管理，方便维护和查询
// ============================================================================

// PromptTemplate 提示词模板
type PromptTemplate struct {
	Name        string // 模板名称
	Description string // 用途说明
	Template    string // 模板内容（支持 %s 占位符）
}

// 系统角色提示词
var (
	// PromptPlanningSystem 任务规划系统提示词
	PromptPlanningSystem = PromptTemplate{
		Name:        "planning_system",
		Description: "任务规划专家系统提示词",
		Template: `你是一个任务规划专家。你的职责是将复杂任务分解为可执行的子任务。

重要规则:
1. 分析任务的复杂度和依赖关系
2. 选择合适的执行模式（串行/并行）
3. 标记需要进一步拆解的复杂子任务
4. 返回严格的 JSON 格式
5. 当前用户账号: %s`,
	}

	// PromptExecutionSystem 任务执行系统提示词
	PromptExecutionSystem = PromptTemplate{
		Name:        "execution_system",
		Description: "任务执行助手系统提示词",
		Template: `你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 调用工具时必须提供所有 required 参数，且使用正确的参数名
3. 调用完工具后返回简单直接的执行结果给用户`,
	}

	// PromptToolSelectionSystem 工具选择系统提示词
	PromptToolSelectionSystem = PromptTemplate{
		Name:        "tool_selection_system",
		Description: "工具选择助手系统提示词",
		Template:    "你是一个工具选择助手。根据任务描述，从工具目录中选择需要的工具。只返回 JSON 格式的工具名称数组。",
	}
)

// 任务规划提示词
var (
	// PromptNodePlanning 节点规划提示词（并行优先）
	PromptNodePlanning = PromptTemplate{
		Name:        "node_planning",
		Description: "任务节点规划提示词，优化为并行优先",
		Template: `你是一个任务规划专家，擅长识别可并行执行的任务。请将任务分解为子任务，**优先考虑并行执行**。

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

## 并行化策略（重要！）

### 优先使用 parallel 模式的场景：
1. 数据收集类：同时获取多个独立数据源（如天气+日历+新闻）
2. 多目标创建：同时创建多个独立对象（如多个提醒、多条记录）
3. 批量处理：对多个项目执行相同操作
4. 独立查询：多个不相互依赖的查询

### 必须使用 sequential 模式的场景：
1. 前后依赖：后一步需要前一步的输出结果
2. 条件分支：根据前一步结果决定后续操作
3. 数据修改后读取：修改数据后需要验证

### 依赖分析规则
- depends_on 只在 parallel 模式下有效
- depends_on 使用 **子任务标题** 作为引用
- 无依赖的任务应该 depends_on: []
- 示例：任务A无依赖，任务B依赖A → A和B可以在 parallel 模式下，B设置 depends_on: ["任务A标题"]
- **⚠️ 严禁循环依赖**：
  - A 不能依赖 B，同时 B 也依赖 A（直接或间接都不允许）
  - 子任务不能依赖父任务或祖先任务（会导致死锁）
  - depends_on 只能引用**同级的其他子任务**，不能引用父级任务


## 其他规则
1. 子任务 1-5 个
2. 标记复杂子任务 can_decompose: true
3. 最大拆解深度: %d，当前深度: %d
4. 所有工具调用传递 account: "%s"
5. 简单任务返回空 subtasks 数组

## 返回 JSON 格式（无 markdown）
{
  "title": "任务标题",
  "goal": "期望目标",
  "execution_mode": "parallel 或 sequential",
  "subtasks": [
    {
      "title": "子任务标题",
      "description": "详细描述",
      "goal": "子任务目标",
      "tools": ["工具名"],
      "can_decompose": false,
      "depends_on": []
    }
  ],
  "reasoning": "选择 parallel/sequential 的原因，说明任务间依赖关系"
}`,
	}

	// PromptNodeExecution 节点执行提示词
	PromptNodeExecution = PromptTemplate{
		Name:        "node_execution",
		Description: "任务节点执行提示词",
		Template: `执行以下任务并返回结果。

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
	}

	// PromptToolSelection 工具选择提示词
	PromptToolSelection = PromptTemplate{
		Name:        "tool_selection",
		Description: "工具选择提示词（减少上下文占用）",
		Template: `根据任务选择需要的工具。只返回 JSON 数组格式的工具名称列表。

## 任务描述
%s

## 可用工具目录
%s

## 选择规则
1. 只选择任务**确实需要**的工具
2. 一般选择 1-5 个工具
3. 返回 JSON 数组，例如: ["CreateReminder", "RawCurrentDate"]
4. 不要返回其他内容，只返回 JSON 数组

返回格式: ["工具名1", "工具名2"]`,
	}

	// PromptResultSynthesis 结果整合提示词
	PromptResultSynthesis = PromptTemplate{
		Name:        "result_synthesis",
		Description: "子任务结果整合提示词",
		Template: `你是一个结果整合专家。请将多个子任务的执行结果整合为一个清晰的最终结果。

## 父任务信息
标题: %s
目标: %s

## 子任务执行结果
%s

## 整合规则
1. 提取每个子任务的关键信息
2. 合并重复或相关的内容
3. 按逻辑顺序组织结果
4. 输出简洁明了的最终结果
5. 如果有任何子任务失败，明确指出

## 返回格式
请直接返回整合后的结果摘要，不需要 JSON 格式。`,
	}
)

// ============================================================================
// 提示词生成函数
// ============================================================================

// BuildPlanningSystemPrompt 构建规划系统提示词
func BuildPlanningSystemPrompt(account string) string {
	return fmt.Sprintf(PromptPlanningSystem.Template, account)
}

// BuildExecutionSystemPrompt 构建执行系统提示词
func BuildExecutionSystemPrompt(account string) string {
	return fmt.Sprintf(PromptExecutionSystem.Template, account, account)
}

// BuildNodePlanningPrompt 构建节点规划提示词
func BuildNodePlanningPrompt(account, title, description, goal, context, tools string, maxDepth, currentDepth int) string {
	return fmt.Sprintf(PromptNodePlanning.Template,
		account, title, description, goal, context, tools,
		maxDepth, currentDepth, account)
}

// BuildNodeExecutionPrompt 构建节点执行提示词
func BuildNodeExecutionPrompt(account, title, description, goal, context string) string {
	return fmt.Sprintf(PromptNodeExecution.Template,
		account, title, description, goal, context, account)
}

// BuildToolSelectionPrompt 构建工具选择提示词
func BuildToolSelectionPrompt(taskDescription, toolCatalog string) string {
	return fmt.Sprintf(PromptToolSelection.Template, taskDescription, toolCatalog)
}

// BuildResultSynthesisPrompt 构建结果整合提示词
func BuildResultSynthesisPrompt(title, goal, childResults string) string {
	return fmt.Sprintf(PromptResultSynthesis.Template, title, goal, childResults)
}

// ============================================================================
// 提示词查询
// ============================================================================

// GetAllPromptTemplates 获取所有提示词模板（用于调试和管理）
func GetAllPromptTemplates() []PromptTemplate {
	return []PromptTemplate{
		PromptPlanningSystem,
		PromptExecutionSystem,
		PromptToolSelectionSystem,
		PromptNodePlanning,
		PromptNodeExecution,
		PromptToolSelection,
	}
}

// GetPromptByName 根据名称获取提示词模板
func GetPromptByName(name string) *PromptTemplate {
	for _, p := range GetAllPromptTemplates() {
		if p.Name == name {
			return &p
		}
	}
	return nil
}
