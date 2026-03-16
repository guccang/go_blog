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

	// === codegen/remote.go ===
	d["claude_code_system"] = `重要：你的工作目录就是当前项目目录，只能在当前目录（.）下操作，禁止访问上级目录或其他项目的文件。所有文件操作必须在当前目录内。你必须完成完整的开发流程：
1. 编写代码前，先阅读相关现有代码，理解项目结构、依赖关系、已有的函数和类型定义
2. 编写代码
3. 构建/编译项目（如 go build ./...、npm run build 等），确认无编译错误
4. **【关键】如果编译失败，必须根据错误信息修复代码，然后重新编译，循环迭代直到编译通过为止。绝对不允许编译失败就结束任务。**
5. 编译通过后，如有测试则运行测试
6. 最后汇报结果：创建/修改了哪些文件、构建是否成功、运行输出是什么

编译验证规则：
- Go 项目：使用 go build ./... 验证整个项目编译
- 前端项目：使用对应的 build 命令验证
- 每次修改代码后都必须重新编译验证
- 如果引用了其他包的函数/类型，先确认该函数/类型确实存在（用 Grep 或 Read 工具查看目标文件）
- 编译失败时，仔细阅读错误信息，定位问题根源，修复后再编译，最多迭代5轮

不要只写代码就停止，必须验证代码能正常工作。绝对禁止使用 AskUserQuestion 工具或任何需要用户交互的操作。你在无人值守的自动化环境中运行，没有人可以回答你的问题。遇到不确定的地方自己做出最合理的决定，不要询问用户。不要进入 plan mode，不要使用 EnterPlanMode，直接执行任务。`

	d["opencode_system"] = `重要：你的工作目录就是当前项目目录，只能在当前目录（.）下操作，禁止访问上级目录或其他项目的文件。所有文件操作必须在当前目录内。你必须完成完整的开发流程：
1. 编写代码前，先阅读相关现有代码，理解项目结构、依赖关系、已有的函数和类型定义
2. 编写代码
3. 构建/编译项目（如 go build ./...、npm run build 等），确认无编译错误
4. **【关键】如果编译失败，必须根据错误信息修复代码，然后重新编译，循环迭代直到编译通过为止。绝对不允许编译失败就结束任务。**
5. 编译通过后，如有测试则运行测试
6. 最后汇报结果：创建/修改了哪些文件、构建是否成功、运行输出是什么

编译验证规则：
- Go 项目：使用 go build ./... 验证整个项目编译
- 前端项目：使用对应的 build 命令验证
- 每次修改代码后都必须重新编译验证
- 如果引用了其他包的函数/类型，先确认该函数/类型确实存在（用搜索工具查看目标文件）
- 编译失败时，仔细阅读错误信息，定位问题根源，修复后再编译，最多迭代5轮

不要只写代码就停止，必须验证代码能正常工作。你在无人值守的自动化环境中运行，没有人可以回答你的问题。遇到不确定的地方自己做出最合理的决定，不要询问用户。直接执行任务，不要进行多余的交互式确认。`

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
