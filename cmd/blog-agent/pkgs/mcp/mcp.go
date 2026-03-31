package mcp

import (
	"encoding/json"
	"fmt"
	"sync"

	"delegation"
	log "mylog"
	"strings"
)

var mcp_version = "Version3.0"
var toolNameMapping = make(map[string]string)
var toolNameMutex sync.RWMutex // 保护 toolNameMapping 的并发访问

// Delegation token 上下文
var delegationTokenContext = &delegationTokenCtx{
	tokens: make(map[string]*delegation.DelegationToken),
}

type delegationTokenCtx struct {
	mu     sync.RWMutex
	tokens map[string]*delegation.DelegationToken
}

// SetDelegationToken 设置当前请求的 delegation token
func SetDelegationToken(requestID string, token *delegation.DelegationToken) {
	delegationTokenContext.mu.Lock()
	defer delegationTokenContext.mu.Unlock()
	delegationTokenContext.tokens[requestID] = token
}

// GetDelegationToken 获取当前请求的 delegation token
func GetDelegationToken(requestID string) *delegation.DelegationToken {
	delegationTokenContext.mu.RLock()
	defer delegationTokenContext.mu.RUnlock()
	return delegationTokenContext.tokens[requestID]
}

// ClearDelegationToken 清除当前请求的 delegation token
func ClearDelegationToken(requestID string) {
	delegationTokenContext.mu.Lock()
	defer delegationTokenContext.mu.Unlock()
	delete(delegationTokenContext.tokens, requestID)
}

// GetDelegationManager 获取 delegation 管理器
func GetDelegationManager() *delegation.Manager {
	return delegation.GetManager()
}

// InitDelegationManager 初始化 delegation 管理器
func InitDelegationManager() {
	delegation.InitManager()
	log.InfoF(log.ModuleMCP, "Delegation manager initialized")
}

// VerifyDelegationToken 验证 delegation token 并返回授权的账户
// 如果验证成功，返回授权的账户；如果验证失败，返回错误
func VerifyDelegationToken(token *delegation.DelegationToken) (string, error) {
	if token == nil {
		return "", delegation.ErrInvalidToken
	}

	mgr := delegation.GetManager()
	if err := mgr.Verify(token); err != nil {
		return "", err
	}

	return token.TargetAccount, nil
}

// ParseDelegationTokenFromHeader 从 header 中解析 delegation token
func ParseDelegationTokenFromHeader(header string) (*delegation.DelegationToken, error) {
	if header == "" {
		return nil, delegation.ErrInvalidToken
	}

	token, err := delegation.Decode(header)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// ValidateAccountAccess 验证账户访问权限
// 如果存在有效的 delegation token，检查请求的 account 是否与 token 中的 target account 匹配
// requestID 用于获取当前请求的 delegation token
func ValidateAccountAccess(requestID string, requestedAccount string) (string, error) {
	token := GetDelegationToken(requestID)
	if token == nil {
		// 没有 delegation token，使用原始 account（session cookie 验证已在 HTTP 层完成）
		return requestedAccount, nil
	}

	// 验证 token
	authorizedAccount, err := VerifyDelegationToken(token)
	if err != nil {
		return "", err
	}

	// 如果请求的 account 与授权账户不符，拒绝访问
	// 委托令牌只能访问其声明的目标账户，不允许通配符权限
	if requestedAccount != "" && requestedAccount != authorizedAccount {
		return "", delegation.NewDelegationError("ACCOUNT_MISMATCH",
			fmt.Sprintf("token authorizes account %s but requested %s", authorizedAccount, requestedAccount))
	}

	// 返回授权的账户
	return authorizedAccount, nil
}

// ToolCall represents a function call
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function call details
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// LLMTool represents a tool available to the LLM
type LLMTool struct {
	Type     string      `json:"type"`
	Function LLMFunction `json:"function"`
}

// LLMFunction represents the function definition for LLM
type LLMFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// MCPTool represents an MCP tool that can be called
type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputschema"`
}

// MCPToolCall represents a tool call request
type MCPToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPToolResponse represents the response from a tool call
type MCPToolResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result"`
	Error   string      `json:"error,omitempty"`
}

func Info() {
	log.InfoF(log.ModuleMCP, "info mcp v3.0 - Internal Tools Only")
}

func extractFunctionName(s string) string {
	lastDot := strings.LastIndex(s, ".")
	if lastDot == -1 {
		return s // 如果没有 `.`，返回整个字符串
	}
	var toolName = s[lastDot+1:]
	toolNameMutex.Lock()
	toolNameMapping[toolName] = s
	toolNameMutex.Unlock()
	return toolName
}

func Init() {
	log.Debug(log.ModuleMCP, "=== MCP Module Initialization Started ===")
	log.DebugF(log.ModuleMCP, "MCP Version: %s", mcp_version)

	RegisterInnerTools()

	tools := GetInnerMCPToolsProcessed()
	log.DebugF(log.ModuleMCP, "MCP module initialized with %d internal tools", len(tools))
	log.Debug(log.ModuleMCP, "=== MCP Module Initialization Completed ===")
}

// GetAvailableLLMTools 返回 LLM 格式的工具列表
// 如果 selectedTools 非空，则只返回选中的工具；否则返回全部内部工具
func GetAvailableLLMTools(selectedTools []string) []LLMTool {
	allTools := GetInnerMCPToolsProcessed()

	if len(selectedTools) == 0 {
		return allTools
	}

	// 预构建选中工具的 map 用于 O(1) 查找
	selectedMap := make(map[string]bool, len(selectedTools))
	for _, t := range selectedTools {
		selectedMap[t] = true
	}

	llmTools := make([]LLMTool, 0, len(selectedTools))
	for _, tool := range allTools {
		if selectedMap[tool.Function.Name] {
			llmTools = append(llmTools, tool)
		}
	}

	return llmTools
}

// CallMCPTool 调用内部工具并返回结果
func CallMCPTool(toolName string, arguments map[string]interface{}) MCPToolResponse {
	log.DebugF(log.ModuleMCP, "toolcall CallMCPTool: %s, arguments: %v", toolName, arguments)

	// 尝试通过映射表找到完整名称
	toolNameMutex.RLock()
	mappedName := toolNameMapping[toolName]
	toolNameMutex.RUnlock()

	// 解析工具名，提取 Inner_blog.xxx 中的 xxx 部分
	callName := toolName
	if mappedName != "" {
		callName = mappedName
	}

	parts := splitToolName(callName)
	if len(parts) == 2 {
		callName = parts[1]
	}

	data := CallInnerTools(callName, arguments)
	// 解析统一信封
	var envelope struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data,omitempty"`
		Error string          `json:"error,omitempty"`
		Hint  string          `json:"hint,omitempty"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return MCPToolResponse{Success: false, Error: fmt.Sprintf("invalid tool response format: %s", data)}
	}
	if !envelope.OK {
		return MCPToolResponse{Success: false, Error: envelope.Error}
	}
	result := string(envelope.Data)
	if envelope.Hint != "" {
		result = result + "\n\n" + envelope.Hint
	}
	return MCPToolResponse{Success: true, Result: result}
}

// CallToolForAPI 供 HTTP API 调用的工具执行入口
// requestID 用于 delegation token 上下文
func CallToolForAPI(toolCall MCPToolCall, requestID string) MCPToolResponse {
	log.DebugF(log.ModuleMCP, "=== Calling MCP Tool: %s ===", toolCall.Name)
	log.DebugF(log.ModuleMCP, "Tool arguments: %v", toolCall.Arguments)

	// 设置当前请求 ID
	SetCurrentRequestID(requestID)

	// 解析工具名
	callName := toolCall.Name
	parts := splitToolName(toolCall.Name)
	if len(parts) == 2 && parts[0] == "Inner_blog" {
		callName = parts[1]
	}

	data := CallInnerToolsWithRequestID(callName, toolCall.Arguments, requestID)
	// 解析统一信封
	var envelope struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data,omitempty"`
		Error string          `json:"error,omitempty"`
		Hint  string          `json:"hint,omitempty"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return MCPToolResponse{Success: false, Error: fmt.Sprintf("invalid tool response format: %s", data)}
	}
	if !envelope.OK {
		return MCPToolResponse{Success: false, Error: envelope.Error}
	}
	result := string(envelope.Data)
	if envelope.Hint != "" {
		result = result + "\n\n" + envelope.Hint
	}
	return MCPToolResponse{Success: true, Result: result}
}

// GetAvailableTools 返回所有可用的内部工具列表（MCPTool 格式）
func GetAvailableTools() []MCPTool {
	innerTools := GetInnerMCPTools(nil)
	tools := make([]MCPTool, 0, len(innerTools))
	for _, tool := range innerTools {
		tools = append(tools, MCPTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}
	return tools
}

func GetVersion() string {
	return mcp_version
}

// splitToolName 分割工具名为 server 和 tool 两部分
func splitToolName(toolName string) []string {
	if idx := findFirstDot(toolName); idx != -1 {
		return []string{toolName[:idx], toolName[idx+1:]}
	}
	return []string{toolName}
}

// findFirstDot 找到字符串中的第一个点号
func findFirstDot(s string) int {
	for i, ch := range s {
		if ch == '.' {
			return i
		}
	}
	return -1
}
