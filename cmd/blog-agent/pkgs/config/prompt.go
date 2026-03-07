package config

import (
	"bufio"
	"fmt"
	log "mylog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ========== Prompt 外部化配置模块 ==========
// 支持从 markdown 文件加载提示词，热重载，默认值兜底

// PromptStore 单账户提示词缓存
type PromptStore struct {
	prompts map[string]string // key → 提示词模板
	mu      sync.RWMutex
}

// PromptManager 多账户提示词管理器
type PromptManager struct {
	stores   map[string]*PromptStore
	defaults map[string]string // key → 默认提示词
	mu       sync.RWMutex
}

var promptManager *PromptManager

// InitPromptManager 初始化提示词管理器
func InitPromptManager() {
	promptManager = &PromptManager{
		stores:   make(map[string]*PromptStore),
		defaults: make(map[string]string),
	}
	registerDefaultPrompts()
	log.Message(log.ModuleConfig, "PromptManager initialized")
}

// GetPrompt 获取提示词（配置文件优先 → 默认值兜底）
func GetPrompt(account, key string) string {
	if promptManager == nil {
		// 未初始化时使用默认值
		return getDefaultPrompt(key)
	}

	store := getPromptStore(account)
	store.mu.RLock()
	if tmpl, ok := store.prompts[key]; ok {
		store.mu.RUnlock()
		return tmpl
	}
	store.mu.RUnlock()

	return getDefaultPrompt(key)
}

// GetPromptWithDefault 获取提示词（配置文件优先 → 自定义兜底）
func GetPromptWithDefault(account, key, def string) string {
	if promptManager == nil {
		return def
	}

	store := getPromptStore(account)
	store.mu.RLock()
	if tmpl, ok := store.prompts[key]; ok {
		store.mu.RUnlock()
		return tmpl
	}
	store.mu.RUnlock()

	// 尝试默认值
	promptManager.mu.RLock()
	if tmpl, ok := promptManager.defaults[key]; ok {
		promptManager.mu.RUnlock()
		return tmpl
	}
	promptManager.mu.RUnlock()

	return def
}

// ReloadPrompts 重新加载指定账户的提示词
func ReloadPrompts(account string) {
	if promptManager == nil {
		return
	}

	store := getPromptStore(account)
	filePath := getPromptsFilePath(account)

	content, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WarnF(log.ModuleConfig, "ReloadPrompts failed for %s: %v", account, err)
		}
		// 文件不存在时清空缓存，使用默认值
		store.mu.Lock()
		store.prompts = make(map[string]string)
		store.mu.Unlock()
		return
	}

	prompts := parsePromptSections(string(content))
	store.mu.Lock()
	store.prompts = prompts
	store.mu.Unlock()

	log.InfoF(log.ModuleConfig, "Reloaded %d prompts for account: %s", len(prompts), account)
}

// GetAllPrompts 获取指定账户所有提示词（调试用）
func GetAllPrompts(account string) map[string]string {
	result := make(map[string]string)

	// 先填充默认值
	if promptManager != nil {
		promptManager.mu.RLock()
		for k, v := range promptManager.defaults {
			result[k] = v
		}
		promptManager.mu.RUnlock()
	}

	// 再覆盖配置文件的值
	if promptManager != nil {
		store := getPromptStore(account)
		store.mu.RLock()
		for k, v := range store.prompts {
			result[k] = v
		}
		store.mu.RUnlock()
	}

	return result
}

// GetSysPromptsConfigTitle 返回提示词配置的博客标题
func GetSysPromptsConfigTitle() string {
	return "sys_prompts"
}

// SafeSprintf 安全的 Sprintf，防止占位符数量不匹配导致 panic
func SafeSprintf(format string, args ...interface{}) string {
	defer func() {
		if r := recover(); r != nil {
			log.WarnF(log.ModuleConfig, "SafeSprintf panic recovered: %v", r)
		}
	}()

	// 计算 format 中 %s / %d 等占位符数量
	expectedArgs := countFormatVerbs(format)
	if expectedArgs != len(args) {
		log.WarnF(log.ModuleConfig, "SafeSprintf: format expects %d args but got %d, using default", expectedArgs, len(args))
		// 参数数量不匹配时，尝试补齐或截断
		if expectedArgs > len(args) {
			// 补齐缺少的参数
			padded := make([]interface{}, expectedArgs)
			copy(padded, args)
			for i := len(args); i < expectedArgs; i++ {
				padded[i] = ""
			}
			return fmt.Sprintf(format, padded...)
		}
		// 参数多于占位符，截断
		return fmt.Sprintf(format, args[:expectedArgs]...)
	}

	return fmt.Sprintf(format, args...)
}

// ========== 内部函数 ==========

// getPromptStore 获取或创建指定账户的提示词存储
func getPromptStore(account string) *PromptStore {
	promptManager.mu.RLock()
	if store, ok := promptManager.stores[account]; ok {
		promptManager.mu.RUnlock()
		return store
	}
	promptManager.mu.RUnlock()

	promptManager.mu.Lock()
	defer promptManager.mu.Unlock()

	// double check
	if store, ok := promptManager.stores[account]; ok {
		return store
	}

	store := &PromptStore{
		prompts: make(map[string]string),
	}
	promptManager.stores[account] = store

	// 懒加载：首次访问时尝试从文件加载
	go loadPromptsFromFile(account, store)

	return store
}

// loadPromptsFromFile 从文件加载提示词
func loadPromptsFromFile(account string, store *PromptStore) {
	filePath := getPromptsFilePath(account)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WarnF(log.ModuleConfig, "loadPromptsFromFile failed for %s: %v", account, err)
		}
		return
	}

	prompts := parsePromptSections(string(content))
	store.mu.Lock()
	store.prompts = prompts
	store.mu.Unlock()

	log.InfoF(log.ModuleConfig, "Loaded %d prompts from file for account: %s", len(prompts), account)
}

// getPromptsFilePath 获取提示词文件路径
func getPromptsFilePath(account string) string {
	return filepath.Join(GetBlogsPath(account), GetSysPromptsConfigTitle()+".md")
}

// getDefaultPrompt 获取默认提示词
func getDefaultPrompt(key string) string {
	if promptManager == nil {
		return ""
	}
	promptManager.mu.RLock()
	defer promptManager.mu.RUnlock()
	if tmpl, ok := promptManager.defaults[key]; ok {
		return tmpl
	}
	return ""
}

// promptKeyPattern 提示词 key 的合法格式：小写字母开头，只包含小写字母、数字、下划线
var promptKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// parsePromptSections 解析 markdown 格式的提示词配置
// 新格式：## key 后用 ``` 代码块包裹内容，代码块内所有行原样保留
// 旧格式（兼容）：## key 后直接写内容，到下一个有效 key 为止
// 只有符合 [a-z][a-z0-9_]* 格式的 ## 标题才被视为 key，
// 其他 ## 标题（如 "## 当前账户"）视为内容的一部分
func parsePromptSections(content string) map[string]string {
	prompts := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentKey string
	var currentContent strings.Builder
	inCodeBlock := false
	waitingForCodeBlock := false // 刚遇到 ## key，等待判断是代码块还是旧格式

	for scanner.Scan() {
		line := scanner.Text()

		// 检测代码块边界 ```
		if currentKey != "" && strings.TrimSpace(line) == "```" {
			if inCodeBlock {
				// 代码块结束 → 保存内容并重置
				prompts[currentKey] = strings.TrimSpace(currentContent.String())
				currentKey = ""
				currentContent.Reset()
				inCodeBlock = false
				waitingForCodeBlock = false
				continue
			}
			if waitingForCodeBlock {
				// 代码块开始
				inCodeBlock = true
				waitingForCodeBlock = false
				continue
			}
		}

		// 代码块内：原样累积所有行（包括 ## 中文标题等）
		if inCodeBlock {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n")
			}
			currentContent.WriteString(line)
			continue
		}

		// 检测 ## key 格式的标题（key 必须是英文小写+下划线格式）
		if strings.HasPrefix(line, "## ") {
			candidate := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if promptKeyPattern.MatchString(candidate) {
				// 保存上一个 key 的内容
				if currentKey != "" {
					prompts[currentKey] = strings.TrimSpace(currentContent.String())
				}
				// 开始新的 key，等待判断格式
				currentKey = candidate
				currentContent.Reset()
				waitingForCodeBlock = true
				continue
			}
		}

		// 无当前 key 时跳过
		if currentKey == "" {
			continue
		}

		// 等待代码块期间，跳过空行
		if waitingForCodeBlock {
			if strings.TrimSpace(line) == "" {
				continue
			}
			// 遇到非空非 ``` 行 → 进入旧格式兼容模式
			waitingForCodeBlock = false
		}

		// 累积内容（旧格式兼容，包括内容中的 ## 中文标题）
		if currentContent.Len() > 0 || line != "" {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n")
			}
			currentContent.WriteString(line)
		}
	}

	// 保存最后一个 key（兼容旧格式或未关闭的代码块）
	if currentKey != "" {
		prompts[currentKey] = strings.TrimSpace(currentContent.String())
	}

	return prompts
}

// countFormatVerbs 计算格式字符串中的占位符数量
func countFormatVerbs(format string) int {
	count := 0
	for i := 0; i < len(format)-1; i++ {
		if format[i] == '%' {
			next := format[i+1]
			if next == 's' || next == 'd' || next == 'v' || next == 'f' {
				count++
				i++ // 跳过下一个字符
			} else if next == '%' {
				i++ // %% 转义，跳过
			}
		}
	}
	return count
}

// registerDefaultPrompts 注册所有默认提示词
func registerDefaultPrompts() {
	d := promptManager.defaults

	// === agent/agent.go ===
	d["wechat_system"] = `你是 Go Blog 智能助手，通过企业微信与用户对话。当前用户账号是 "%s"，请直接使用此账号调用工具，不要询问用户账号。重要规则：1. 收到指令后直接执行，不要反问确认、不要列出方案让用户选择，自行决定最合理的参数并立即调用工具。2. 回复必须精简，控制在500字以内，只输出执行结果和关键数据，不要冗余解释。适合手机屏幕阅读。`

	// === agent/prompts.go ===
	d["planning_system"] = `你是一个任务规划专家。你的职责是将复杂任务分解为可执行的子任务。

重要规则:
1. 分析任务的复杂度和依赖关系
2. 选择合适的执行模式（串行/并行）
3. 标记需要进一步拆解的复杂子任务
4. 返回严格的 JSON 格式
5. 当前用户账号: %s`

	d["execution_system"] = `你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 调用工具时必须提供所有 required 参数，且使用正确的参数名
3. 调用完工具后返回简单直接的执行结果给用户`

	d["tool_selection_system"] = `你是一个工具选择助手。根据任务描述，从工具目录中选择需要的工具。只返回 JSON 格式的工具名称数组。`

	d["node_planning"] = `你是一个任务规划专家，擅长识别可并行执行的任务。请将任务分解为子任务，**优先考虑并行执行**。

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
}`

	d["node_execution"] = `执行以下任务并返回结果。

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
2. 关键数据或信息（如有）`

	d["node_execution_retry"] = `这是一个重试执行的任务。之前的执行尝试失败了，请从失败点继续执行。

## 当前账户
%s

## 任务信息
标题: %s
描述: %s
目标: %s

## 上下文
%s

## 之前的执行历史
以下是之前执行尝试中已完成的工具调用记录，请不要重复这些已成功的操作：
%s

## 重要规则
1. 所有工具调用都必须传递 "account": "%s" 参数
2. **不要重复已成功的工具调用**，从失败点继续执行
3. 参考之前的执行结果，在此基础上继续完成任务
4. 如果之前的工具调用结果中已包含所需数据，直接使用即可
5. 返回结果要简洁明了，包含关键信息

## 返回格式
执行完成后，请返回：
1. 执行结果的简要描述
2. 关键数据或信息（如有）`

	d["tool_selection"] = `根据任务选择需要的工具。只返回 JSON 数组格式的工具名称列表。

## 任务描述
%s

## 可用工具目录
%s

## 选择规则
1. 只选择任务**确实需要**的工具
2. 一般选择 1-5 个工具
3. 返回 JSON 数组，例如: ["CreateReminder", "RawCurrentDate"]
4. 不要返回其他内容，只返回 JSON 数组

返回格式: ["工具名1", "工具名2"]`

	d["result_synthesis"] = `你是一个结果整合专家。请将多个子任务的执行结果整合为一个清晰的最终结果。

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
请直接返回整合后的结果摘要，不需要 JSON 格式。`

	// === agent/planner.go ===
	d["result_synthesis_system"] = `你是一个结果整合专家，擅长将多个子任务的结果整合为清晰的最终结果。`

	d["planner_execution_system"] = `你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 调用工具时使用正确的参数名，参考工具定义中的 required 字段
3. 调用完工具后返回简单直接的执行结果给用户

你可以帮助用户完成各种任务，如创建提醒、查询博客、分析数据等。`

	d["planner_execution_tools_system"] = `你是一个任务执行助手。当前用户账号是: %s

重要规则:
1. 所有工具调用都必须传递 "account": "%s" 参数
2. 调用工具时使用正确的参数名
3. 调用完工具后返回简单直接的执行结果给用户`

	// === agent/report.go ===
	d["daily_report"] = `你是一个智能报告助手。请根据以下数据生成一份简洁的日报。

日期: %s

## 今日数据

### 待办事项
%s

### 运动记录
%s

### 运动统计
%s

### 阅读情况
%s

### 任务进度
%s

## 报告要求
1. 用 Markdown 格式输出
2. 包含以下部分：今日总结、完成情况、运动数据、阅读进展、明日建议
3. 语气专业但友好
4. 如果某部分没有数据，简要说明即可
5. 在末尾给出1-2条针对性的改进建议`

	d["weekly_report"] = `你是一个智能报告助手。请根据以下数据生成一份详细的周报。

周期: %s 至 %s

## 本周数据

### 待办事项（本周所有）
%s

### 运动统计（7天）
%s

### 运动详情
%s

### 阅读情况
%s

### 任务进度
%s

## 报告要求
1. 用 Markdown 格式输出
2. 包含：本周总结、待办完成率分析、运动趋势、阅读进展、任务推进、下周计划建议
3. 对比上周数据给出趋势分析（如果有的话）
4. 给出2-3条具体可执行的改进建议
5. 语气专业、有洞察力`

	d["monthly_report"] = `你是一个智能报告助手。请根据以下数据生成一份全面的月报。

月份: %d年%d月

### 待办数据
%s

### 运动统计（30天）
%s

### 阅读情况
%s

### 本月目标
%s

### 任务进度
%s

## 报告要求
1. Markdown 格式
2. 包含：月度总结、目标达成率、运动/阅读分析、关键成就、不足与改进
3. 给出下月目标调整建议
4. 数据驱动，有具体数字`

	// === agent/scheduler.go ===
	d["smart_reminder"] = `你是一个智能提醒助手。请根据以下信息生成一条简洁、有温度的提醒消息。

提醒标题: %s
原始消息: %s
当前时间: %s

用户今日待办: %s
用户近7天运动: %s

要求:
1. 消息简洁，不超过200字
2. 结合用户的待办和运动数据给出个性化提醒
3. 语气温暖友好，带有鼓励
4. 直接输出消息内容，不要加任何前缀`

	// === llm/user_context.go ===
	d["ai_assistant_system"] = `你是一个智能助手，是用户的私人AI管家。
重要规则：
1. 当前用户账号是 "%s"，调用任何工具时直接使用此账号作为account参数，不要向用户询问账号。
2. 需要日期时，先调用 RawCurrentDate 获取当前日期，再基于日期调用其他工具。
3. 自行决定调用哪些工具获取数据，得到结果后不要重复调用相同工具。
4. 最后返回简洁直接的分析结果给用户。
5. 你了解用户的待办事项、运动记录、阅读进度和年度目标，可以主动给出建议。%s`

	d["ai_assistant_context"] = `

以下是用户当前的个人数据摘要（今天是 %s）：
%s

请结合以上用户数据摘要，给出个性化、具体的回答。如果用户询问的内容与其个人数据相关，优先使用上述数据。如果需要更详细的数据，可以使用工具获取。
如果有"近期对话记忆"，可以自然地引用之前的对话，例如"上次你问过..."。`

	// === llm/tool_handler.go ===
	d["tool_routing"] = `你是一个工具路由器。根据用户的问题，从以下工具目录中选择所有可能需要用到的工具。

用户问题: %s

工具目录:
%s
选择规则：
1. 宁多勿少，把所有可能相关的工具都选上
2. 如果任务需要日期信息，必须包含 RawCurrentDate
3. 如果涉及查询数据，同时选择获取数据的工具和可能需要的辅助工具
4. 只返回JSON数组，不要其他文字

示例: ["RawCurrentDate", "RawGetExerciseByDateRange"]
如果不需要任何工具，返回 []`

	// === codegen/remote.go ===
	d["claude_code_system"] = `重要：你的工作目录就是当前项目目录，只能在当前目录（.）下操作，禁止访问上级目录或其他项目的文件。所有文件操作必须在当前目录内。你必须完成完整的开发流程：1. 编写代码；2. 构建/编译项目（如 go build、npm run build 等），确认无编译错误；3. 运行程序并验证输出正确；4. 如有测试则运行测试；5. 最后汇报结果：创建了哪些文件、构建是否成功、运行输出是什么。不要只写代码就停止，必须验证代码能正常工作。绝对禁止使用 AskUserQuestion 工具或任何需要用户交互的操作。你在无人值守的自动化环境中运行，没有人可以回答你的问题。遇到不确定的地方自己做出最合理的决定，不要询问用户。不要进入 plan mode，不要使用 EnterPlanMode，直接执行任务。`

	d["opencode_system"] = `重要：你的工作目录就是当前项目目录，只能在当前目录（.）下操作，禁止访问上级目录或其他项目的文件。所有文件操作必须在当前目录内。你必须完成完整的开发流程：1. 编写代码；2. 构建/编译项目（如 go build、npm run build 等），确认无编译错误；3. 运行程序并验证输出正确；4. 如有测试则运行测试；5. 最后汇报结果：创建了哪些文件、构建是否成功、运行输出是什么。不要只写代码就停止，必须验证代码能正常工作。你在无人值守的自动化环境中运行，没有人可以回答你的问题。遇到不确定的地方自己做出最合理的决定，不要询问用户。直接执行任务，不要进行多余的交互式确认。`

	// === mcp/ai_tools.go ===
	d["exercise_companion"] = `你是用户的私人健身教练。请根据以上运动数据，给出以下建议：
1. 今日运动推荐（考虑最近训练的身体部位，避免连续练同一位置）
2. 本周运动趋势评价（运动量是否足够、是否规律）
3. 如果用户运动量不足，给出温和的鼓励和具体建议`

	d["reading_companion"] = `你是用户的阅读伙伴。请根据以上数据，给出以下建议：
1. 正在阅读的书籍的进度评价和预计完成时间
2. 阅读速度分析
3. 如果有已完成的书，推荐下一本应该读什么
4. 鼓励用户保持阅读习惯`

	d["task_decomposition"] = `请将用户的复杂任务拆解为3-7个具体的、可独立完成的子任务。每个子任务应该:
1. 足够具体，一次可以完成
2. 有明确的完成标准
3. 不与已有待办重复

拆解后，询问用户是否同意添加这些子任务。如果用户同意，使用 RawAddTodo 工具逐一添加。`
}
