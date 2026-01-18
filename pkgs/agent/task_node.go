package agent

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// 常量定义
// ============================================================================

// ExecutionMode 执行模式
type ExecutionMode string

const (
	ModeSequential ExecutionMode = "sequential" // 串行执行
	ModeParallel   ExecutionMode = "parallel"   // 并行执行
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodePending      NodeStatus = "pending"       // 等待中
	NodeRunning      NodeStatus = "running"       // 执行中
	NodePaused       NodeStatus = "paused"        // 已暂停
	NodeWaitingInput NodeStatus = "waiting_input" // 等待用户输入
	NodeDone         NodeStatus = "done"          // 已完成
	NodeFailed       NodeStatus = "failed"        // 失败
	NodeCanceled     NodeStatus = "canceled"      // 已取消
	NodeSkipped      NodeStatus = "skipped"       // 已跳过
)

// LogLevel 日志级别
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
	LogTrace LogLevel = "trace" // 详细追踪
)

// 配置常量
const (
	DefaultMaxDepth      = 3    // 默认最大递归深度
	DefaultMaxContextLen = 4000 // 默认最大上下文长度（字符）
	DefaultMaxRetries    = 3    // 默认最大重试次数
)

// ============================================================================
// TaskNode - 任务节点
// ============================================================================

// LLMInteraction LLM交互记录（用于问题追踪）
type LLMInteraction struct {
	Timestamp  time.Time      `json:"timestamp"`
	Phase      string         `json:"phase"`                // planning/execution/synthesis
	Request    string         `json:"request"`              // 发送给 LLM 的 prompt
	Response   string         `json:"response"`             // LLM 返回的内容
	ToolCalls  []ToolCallInfo `json:"tool_calls,omitempty"` // 工具调用记录
	TokensUsed int            `json:"tokens_used,omitempty"`
	Duration   int64          `json:"duration_ms,omitempty"` // 耗时毫秒
}

// ToolCallInfo 工具调用信息
type ToolCallInfo struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
	Result    interface{} `json:"result,omitempty"`
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
}

// TaskNode 任务节点（支持递归子任务）
type TaskNode struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"` // 父节点ID
	RootID   string `json:"root_id"`             // 根任务ID
	Depth    int    `json:"depth"`               // 节点深度（0=根节点）
	Account  string `json:"account"`             // 所属账户

	// 任务描述
	Title       string `json:"title"`
	Description string `json:"description"`
	Goal        string `json:"goal"` // 期望目标

	// 执行配置
	ExecutionMode ExecutionMode `json:"execution_mode"`       // 子节点执行模式
	ToolCalls     []string      `json:"tool_calls,omitempty"` // 需要调用的工具
	MaxRetries    int           `json:"max_retries"`          // 最大重试次数
	RetryCount    int           `json:"retry_count"`          // 当前重试次数
	CanDecompose  bool          `json:"can_decompose"`        // 是否可以进一步拆解
	DependsOn     []string      `json:"depends_on,omitempty"` // 依赖的节点ID

	// 子节点
	Children []*TaskNode `json:"children,omitempty"`
	ChildIDs []string    `json:"child_ids,omitempty"` // 用于存储

	// 状态与进度
	Status   NodeStatus `json:"status"`
	Progress float64    `json:"progress"` // 0-100

	// 上下文与结果
	Context *TaskContext `json:"context"`
	Result  *TaskResult  `json:"result,omitempty"`

	// 时间信息
	CreatedAt  time.Time     `json:"created_at"`
	StartedAt  *time.Time    `json:"started_at,omitempty"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`

	// 日志
	Logs []ExecutionLog `json:"logs"`

	// LLM交互历史（用于问题追踪）
	LLMHistory []LLMInteraction `json:"llm_history,omitempty"`

	// 用户输入等待
	PendingInput  *InputRequest  `json:"pending_input,omitempty"`
	InputResponse *InputResponse `json:"input_response,omitempty"`

	// 内部控制（不序列化）
	mu       sync.RWMutex        `json:"-"`
	pauseCh  chan struct{}       `json:"-"`
	cancelCh chan struct{}       `json:"-"`
	inputCh  chan *InputResponse `json:"-"` // 等待用户输入通道
}

// NewTaskNode 创建新任务节点
func NewTaskNode(account, title, description string) *TaskNode {
	id := generateNodeID()
	return &TaskNode{
		ID:            id,
		RootID:        id, // 根节点时 RootID = ID
		Depth:         0,
		Account:       account,
		Title:         title,
		Description:   description,
		ExecutionMode: ModeSequential,
		MaxRetries:    DefaultMaxRetries,
		CanDecompose:  true,
		Status:        NodePending,
		Progress:      0,
		Context:       NewTaskContext(description),
		Logs:          []ExecutionLog{},
		CreatedAt:     time.Now(),
		pauseCh:       make(chan struct{}, 1),
		cancelCh:      make(chan struct{}, 1),
		inputCh:       make(chan *InputResponse, 1),
	}
}

// NewChildNode 创建子节点
func (n *TaskNode) NewChildNode(title, description, goal string) *TaskNode {
	child := &TaskNode{
		ID:            generateNodeID(),
		ParentID:      n.ID,
		RootID:        n.RootID,
		Depth:         n.Depth + 1,
		Account:       n.Account,
		Title:         title,
		Description:   description,
		Goal:          goal,
		ExecutionMode: ModeSequential,
		MaxRetries:    DefaultMaxRetries,
		CanDecompose:  true,
		Status:        NodePending,
		Progress:      0,
		Context:       NewTaskContext(description),
		Logs:          []ExecutionLog{},
		CreatedAt:     time.Now(),
		pauseCh:       make(chan struct{}, 1),
		cancelCh:      make(chan struct{}, 1),
		inputCh:       make(chan *InputResponse, 1),
	}

	// 继承父节点的用户输入
	child.Context.UserInput = n.Context.UserInput

	n.mu.Lock()
	n.Children = append(n.Children, child)
	n.ChildIDs = append(n.ChildIDs, child.ID)
	n.mu.Unlock()

	return child
}

// AddLog 添加执行日志
func (n *TaskNode) AddLog(level LogLevel, phase, message string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Logs = append(n.Logs, ExecutionLog{
		Time:    time.Now(),
		Level:   level,
		Phase:   phase,
		Message: message,
		NodeID:  n.ID,
	})
}

// AddLLMInteraction 添加 LLM 交互记录
func (n *TaskNode) AddLLMInteraction(phase, request, response string, tokensUsed int, duration int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.LLMHistory = append(n.LLMHistory, LLMInteraction{
		Timestamp:  time.Now(),
		Phase:      phase,
		Request:    request,
		Response:   response,
		TokensUsed: tokensUsed,
		Duration:   duration,
	})
}

// AddLLMInteractionWithTools 添加带工具调用的 LLM 交互记录
func (n *TaskNode) AddLLMInteractionWithTools(phase, request, response string, toolCalls []ToolCallInfo, tokensUsed int, duration int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.LLMHistory = append(n.LLMHistory, LLMInteraction{
		Timestamp:  time.Now(),
		Phase:      phase,
		Request:    request,
		Response:   response,
		ToolCalls:  toolCalls,
		TokensUsed: tokensUsed,
		Duration:   duration,
	})
}

// AddLogWithData 添加带数据的执行日志
func (n *TaskNode) AddLogWithData(level LogLevel, phase, message string, data interface{}) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Logs = append(n.Logs, ExecutionLog{
		Time:    time.Now(),
		Level:   level,
		Phase:   phase,
		Message: message,
		NodeID:  n.ID,
		Data:    data,
	})
}

// SetStatus 设置状态
func (n *TaskNode) SetStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
	if status == NodeRunning && n.StartedAt == nil {
		now := time.Now()
		n.StartedAt = &now
	}
	if status == NodeDone || status == NodeFailed || status == NodeCanceled {
		now := time.Now()
		n.FinishedAt = &now
		if n.StartedAt != nil {
			n.Duration = now.Sub(*n.StartedAt)
		}
	}
}

// SetProgress 设置进度
func (n *TaskNode) SetProgress(progress float64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Progress = progress
}

// GetStatus 获取状态（线程安全）
func (n *TaskNode) GetStatus() NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Status
}

// GetProgress 获取进度（线程安全）
func (n *TaskNode) GetProgress() float64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Progress
}

// IsCanceled 检查是否已取消
func (n *TaskNode) IsCanceled() bool {
	select {
	case <-n.cancelCh:
		return true
	default:
		return false
	}
}

// IsPaused 检查是否已暂停
func (n *TaskNode) IsPaused() bool {
	select {
	case <-n.pauseCh:
		return true
	default:
		return false
	}
}

// Pause 暂停节点
func (n *TaskNode) Pause() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.Status == NodeRunning {
		n.Status = NodePaused
		select {
		case n.pauseCh <- struct{}{}:
		default:
		}
	}
}

// Resume 恢复节点
func (n *TaskNode) Resume() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.Status == NodePaused {
		n.Status = NodeRunning
		select {
		case <-n.pauseCh:
		default:
		}
	}
}

// Cancel 取消节点
func (n *TaskNode) Cancel() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.Status == NodePending || n.Status == NodeRunning || n.Status == NodePaused {
		n.Status = NodeCanceled
		close(n.cancelCh)
		// 递归取消子节点
		for _, child := range n.Children {
			child.Cancel()
		}
	}
}

// CanRetry 检查是否可以重试
func (n *TaskNode) CanRetry() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.RetryCount < n.MaxRetries
}

// IncrementRetry 增加重试计数
func (n *TaskNode) IncrementRetry() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.RetryCount++
}

// CalculateProgress 根据子节点计算进度
func (n *TaskNode) CalculateProgress() float64 {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if len(n.Children) == 0 {
		return n.Progress
	}

	var total float64
	for _, child := range n.Children {
		total += child.CalculateProgress()
	}
	return total / float64(len(n.Children))
}

// ============================================================================
// 用户输入等待方法
// ============================================================================

// WaitForInput 设置等待用户输入并阻塞直到收到响应
// 如果用户取消，返回 nil 和 true（表示跳过）
func (n *TaskNode) WaitForInput(req *InputRequest) (*InputResponse, bool) {
	n.mu.Lock()
	n.Status = NodeWaitingInput
	n.PendingInput = req
	n.mu.Unlock()

	n.AddLog(LogInfo, "waiting_input", "等待用户输入: "+req.Title)

	// 阻塞等待用户响应（无超时）
	resp := <-n.inputCh

	n.mu.Lock()
	n.PendingInput = nil
	n.InputResponse = resp
	if resp.Cancelled {
		n.Status = NodeRunning // 恢复执行但会跳过该步骤
	} else {
		n.Status = NodeRunning
	}
	n.mu.Unlock()

	n.AddLog(LogInfo, "input_received", "收到用户输入")

	return resp, resp.Cancelled
}

// ReceiveInput 接收用户输入响应
func (n *TaskNode) ReceiveInput(resp *InputResponse) {
	select {
	case n.inputCh <- resp:
		// 成功发送
	default:
		// 通道已满，忽略
	}
}

// HasPendingInput 检查是否有待处理的输入请求
func (n *TaskNode) HasPendingInput() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.PendingInput != nil
}

// GetPendingInput 获取待处理的输入请求
func (n *TaskNode) GetPendingInput() *InputRequest {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.PendingInput
}

// IsWaitingInput 检查是否正在等待输入
func (n *TaskNode) IsWaitingInput() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Status == NodeWaitingInput
}

// ClearInput 清除输入状态
func (n *TaskNode) ClearInput() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.PendingInput = nil
	n.InputResponse = nil
}

// generateNodeID 生成节点ID
func generateNodeID() string {
	return uuid.New().String()[:8]
}

// ============================================================================
// TaskContext - 任务上下文
// ============================================================================

// TaskContext 任务上下文
type TaskContext struct {
	// 原始用户输入
	UserInput string `json:"user_input"`

	// 聊天历史（精简版）
	ChatHistory []ChatMessage `json:"chat_history,omitempty"`

	// 父任务结果（用于构建 LLM prompt）
	ParentResults []ParentResult `json:"parent_results,omitempty"`

	// 兄弟任务结果（已完成的同级任务）
	SiblingResults []SiblingResult `json:"sibling_results,omitempty"`

	// 变量存储（任务间数据传递）
	Variables map[string]interface{} `json:"variables,omitempty"`

	// 压缩配置
	MaxContextLen int  `json:"max_context_len"`
	IsCompressed  bool `json:"is_compressed"`

	// Token 统计
	TokensUsed int `json:"tokens_used"`
	MaxTokens  int `json:"max_tokens"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string    `json:"role"` // user/assistant/system
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// ParentResult 父任务结果摘要
type ParentResult struct {
	NodeID  string `json:"node_id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// SiblingResult 兄弟任务结果
type SiblingResult struct {
	NodeID  string     `json:"node_id"`
	Title   string     `json:"title"`
	Status  NodeStatus `json:"status"`
	Summary string     `json:"summary"`
}

// NewTaskContext 创建任务上下文
func NewTaskContext(userInput string) *TaskContext {
	return &TaskContext{
		UserInput:      userInput,
		ChatHistory:    []ChatMessage{},
		ParentResults:  []ParentResult{},
		SiblingResults: []SiblingResult{},
		Variables:      make(map[string]interface{}),
		MaxContextLen:  DefaultMaxContextLen,
	}
}

// AddChatMessage 添加聊天消息
func (c *TaskContext) AddChatMessage(role, content string) {
	c.ChatHistory = append(c.ChatHistory, ChatMessage{
		Role:    role,
		Content: content,
		Time:    time.Now(),
	})
	// 检查是否需要压缩
	c.compressIfNeeded()
}

// AddParentResult 添加父任务结果
func (c *TaskContext) AddParentResult(nodeID, title, summary string) {
	c.ParentResults = append(c.ParentResults, ParentResult{
		NodeID:  nodeID,
		Title:   title,
		Summary: summary,
	})
}

// AddSiblingResult 添加兄弟任务结果
func (c *TaskContext) AddSiblingResult(nodeID, title string, status NodeStatus, summary string) {
	c.SiblingResults = append(c.SiblingResults, SiblingResult{
		NodeID:  nodeID,
		Title:   title,
		Status:  status,
		Summary: summary,
	})
}

// SetVariable 设置变量
func (c *TaskContext) SetVariable(key string, value interface{}) {
	c.Variables[key] = value
}

// GetVariable 获取变量
func (c *TaskContext) GetVariable(key string) (interface{}, bool) {
	val, ok := c.Variables[key]
	return val, ok
}

// GetContextLength 获取当前上下文长度（字符数）
func (c *TaskContext) GetContextLength() int {
	length := len(c.UserInput)
	for _, msg := range c.ChatHistory {
		length += len(msg.Content)
	}
	for _, pr := range c.ParentResults {
		length += len(pr.Summary)
	}
	for _, sr := range c.SiblingResults {
		length += len(sr.Summary)
	}
	return length
}

// compressIfNeeded 如果需要则压缩上下文
func (c *TaskContext) compressIfNeeded() {
	if c.GetContextLength() <= c.MaxContextLen {
		return
	}

	// 压缩策略：
	// 1. 保留最近的聊天记录
	// 2. 截断较长的消息
	// 3. 压缩父/兄弟任务摘要

	// 压缩聊天历史：只保留最近 5 条
	if len(c.ChatHistory) > 5 {
		c.ChatHistory = c.ChatHistory[len(c.ChatHistory)-5:]
	}

	// 截断每条消息的长度
	maxMsgLen := 500
	for i := range c.ChatHistory {
		if len(c.ChatHistory[i].Content) > maxMsgLen {
			c.ChatHistory[i].Content = c.ChatHistory[i].Content[:maxMsgLen] + "..."
		}
	}

	// 压缩父任务摘要
	maxSummaryLen := 200
	for i := range c.ParentResults {
		if len(c.ParentResults[i].Summary) > maxSummaryLen {
			c.ParentResults[i].Summary = c.ParentResults[i].Summary[:maxSummaryLen] + "..."
		}
	}

	// 压缩兄弟任务摘要
	for i := range c.SiblingResults {
		if len(c.SiblingResults[i].Summary) > maxSummaryLen {
			c.SiblingResults[i].Summary = c.SiblingResults[i].Summary[:maxSummaryLen] + "..."
		}
	}

	c.IsCompressed = true
}

// BuildLLMContext 构建 LLM 请求的上下文字符串
func (c *TaskContext) BuildLLMContext() string {
	// 预估容量以减少内存分配
	estimatedSize := len(c.UserInput) + 50
	for _, pr := range c.ParentResults {
		estimatedSize += len(pr.Title) + len(pr.Summary) + 20
	}
	for _, sr := range c.SiblingResults {
		estimatedSize += len(sr.Title) + len(sr.Summary) + 30
	}

	var sb strings.Builder
	sb.Grow(estimatedSize)

	// 用户输入
	sb.WriteString("## 原始用户请求\n")
	sb.WriteString(c.UserInput)
	sb.WriteString("\n\n")

	// 父任务结果
	if len(c.ParentResults) > 0 {
		sb.WriteString("## 父任务执行结果\n")
		for _, pr := range c.ParentResults {
			sb.WriteString("- ")
			sb.WriteString(pr.Title)
			sb.WriteString(": ")
			sb.WriteString(pr.Summary)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// 兄弟任务结果
	if len(c.SiblingResults) > 0 {
		sb.WriteString("## 已完成的同级任务\n")
		for _, sr := range c.SiblingResults {
			sb.WriteString("- ")
			sb.WriteString(sr.Title)
			sb.WriteString(" [")
			sb.WriteString(string(sr.Status))
			sb.WriteString("]: ")
			sb.WriteString(sr.Summary)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ============================================================================
// TaskResult - 任务结果
// ============================================================================

// TaskResult 任务执行结果
type TaskResult struct {
	Success     bool                   `json:"success"`
	Output      string                 `json:"output"`                // 主要输出
	Summary     string                 `json:"summary"`               // 结果摘要（LLM整合后）
	RawSummary  string                 `json:"raw_summary,omitempty"` // 原始摘要（整合前）
	Data        map[string]interface{} `json:"data,omitempty"`        // 结构化数据
	Error       string                 `json:"error,omitempty"`
	ToolResults []ToolCallResult       `json:"tool_results,omitempty"`
	Artifacts   []string               `json:"artifacts,omitempty"` // 保存的博客链接
}

// ToolCallResult 工具调用结果
type ToolCallResult struct {
	ToolName string        `json:"tool_name"`
	Args     interface{}   `json:"args"`
	Result   interface{}   `json:"result"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// NewTaskResult 创建成功结果
func NewTaskResult(output, summary string) *TaskResult {
	return &TaskResult{
		Success: true,
		Output:  output,
		Summary: summary,
		Data:    make(map[string]interface{}),
	}
}

// NewTaskResultError 创建失败结果
func NewTaskResultError(err string) *TaskResult {
	return &TaskResult{
		Success: false,
		Error:   err,
	}
}

// AddToolResult 添加工具调用结果
func (r *TaskResult) AddToolResult(toolName string, args, result interface{}, success bool, err string, duration time.Duration) {
	r.ToolResults = append(r.ToolResults, ToolCallResult{
		ToolName: toolName,
		Args:     args,
		Result:   result,
		Success:  success,
		Error:    err,
		Duration: duration,
	})
}

// ToJSON 转换为 JSON 字符串
func (r *TaskResult) ToJSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}

// ============================================================================
// ExecutionLog - 执行日志
// ============================================================================

// ExecutionLog 执行日志
type ExecutionLog struct {
	Time    time.Time   `json:"time"`
	Level   LogLevel    `json:"level"`
	Phase   string      `json:"phase"` // planning/executing/tool_call/completed
	Message string      `json:"message"`
	NodeID  string      `json:"node_id"`
	Data    interface{} `json:"data,omitempty"`
}

// ToJSON 转换为 JSON 字符串
func (l *ExecutionLog) ToJSON() string {
	data, _ := json.Marshal(l)
	return string(data)
}

// ============================================================================
// 执行配置
// ============================================================================

// ExecutionConfig 执行配置
type ExecutionConfig struct {
	MaxDepth         int           `json:"max_depth"`         // 最大递归深度
	MaxContextLen    int           `json:"max_context_len"`   // 最大上下文长度
	MaxRetries       int           `json:"max_retries"`       // 最大重试次数
	ExecutionTimeout time.Duration `json:"execution_timeout"` // 执行超时
	EnableLogging    bool          `json:"enable_logging"`    // 启用详细日志
}

// DefaultExecutionConfig 默认执行配置
func DefaultExecutionConfig() *ExecutionConfig {
	return &ExecutionConfig{
		MaxDepth:         DefaultMaxDepth,
		MaxContextLen:    DefaultMaxContextLen,
		MaxRetries:       DefaultMaxRetries,
		ExecutionTimeout: 60 * time.Minute, // 1小时
		EnableLogging:    true,
	}
}

// ============================================================================
// 用户输入请求
// ============================================================================

// InputType 输入类型
type InputType string

const (
	InputTypeText     InputType = "text"     // 单行文本
	InputTypeTextarea InputType = "textarea" // 多行文本
	InputTypeSelect   InputType = "select"   // 单选
	InputTypeMulti    InputType = "multi"    // 多选
	InputTypeConfirm  InputType = "confirm"  // 确认（是/否）
	InputTypePassword InputType = "password" // 密码
	InputTypeNumber   InputType = "number"   // 数字
	InputTypeDate     InputType = "date"     // 日期
)

// InputRequest 输入请求
type InputRequest struct {
	ID          string           `json:"id"`                    // 请求ID
	NodeID      string           `json:"node_id"`               // 触发节点
	TaskID      string           `json:"task_id"`               // 根任务ID
	Account     string           `json:"account"`               // 所属账户
	Title       string           `json:"title"`                 // 标题
	Message     string           `json:"message"`               // 提示消息
	InputType   InputType        `json:"input_type"`            // 输入类型
	Options     []InputOption    `json:"options,omitempty"`     // 选项（多选用）
	Placeholder string           `json:"placeholder,omitempty"` // 占位符
	Required    bool             `json:"required"`              // 是否必填
	Default     string           `json:"default,omitempty"`     // 默认值
	Validation  *InputValidation `json:"validation,omitempty"`  // 验证规则
	CreatedAt   time.Time        `json:"created_at"`
}

// InputOption 输入选项
type InputOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
}

// InputValidation 输入验证规则
type InputValidation struct {
	MinLength int    `json:"min_length,omitempty"`
	MaxLength int    `json:"max_length,omitempty"`
	Pattern   string `json:"pattern,omitempty"` // 正则表达式
	Min       *int   `json:"min,omitempty"`     // 数字最小值
	Max       *int   `json:"max,omitempty"`     // 数字最大值
}

// InputResponse 输入响应
type InputResponse struct {
	RequestID string      `json:"request_id"`
	NodeID    string      `json:"node_id"`
	TaskID    string      `json:"task_id"`
	Value     interface{} `json:"value"`     // 用户输入的值
	Cancelled bool        `json:"cancelled"` // 用户取消
	CreatedAt time.Time   `json:"created_at"`
}

// NewInputRequest 创建输入请求
func NewInputRequest(nodeID, taskID, account, title, message string, inputType InputType) *InputRequest {
	return &InputRequest{
		ID:        uuid.New().String()[:8],
		NodeID:    nodeID,
		TaskID:    taskID,
		Account:   account,
		Title:     title,
		Message:   message,
		InputType: inputType,
		Required:  true,
		CreatedAt: time.Now(),
	}
}

// WithOptions 添加选项
func (r *InputRequest) WithOptions(options []InputOption) *InputRequest {
	r.Options = options
	return r
}

// WithPlaceholder 设置占位符
func (r *InputRequest) WithPlaceholder(placeholder string) *InputRequest {
	r.Placeholder = placeholder
	return r
}

// WithDefault 设置默认值
func (r *InputRequest) WithDefault(defaultVal string) *InputRequest {
	r.Default = defaultVal
	return r
}

// WithValidation 设置验证规则
func (r *InputRequest) WithValidation(v *InputValidation) *InputRequest {
	r.Validation = v
	return r
}

// NewInputResponse 创建输入响应
func NewInputResponse(requestID, nodeID, taskID string, value interface{}, cancelled bool) *InputResponse {
	return &InputResponse{
		RequestID: requestID,
		NodeID:    nodeID,
		TaskID:    taskID,
		Value:     value,
		Cancelled: cancelled,
		CreatedAt: time.Now(),
	}
}

// ToJSON 转换为 JSON
func (r *InputRequest) ToJSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}
