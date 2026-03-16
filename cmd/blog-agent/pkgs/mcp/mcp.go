package mcp

import (
	"fmt"
	log "mylog"
	"strings"
	"sync"
)

var mcp_version = "Version3.0"
var toolNameMapping = make(map[string]string)
var toolNameMutex sync.RWMutex // 保护 toolNameMapping 的并发访问

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
	if data == "" {
		return MCPToolResponse{
			Success: false,
			Error:   "Error NOT find tool: " + toolName,
		}
	}
	return MCPToolResponse{
		Success: true,
		Result:  data,
	}
}

// CallToolForAPI 供 HTTP API 调用的工具执行入口
func CallToolForAPI(toolCall MCPToolCall) MCPToolResponse {
	log.DebugF(log.ModuleMCP, "=== Calling MCP Tool: %s ===", toolCall.Name)
	log.DebugF(log.ModuleMCP, "Tool arguments: %v", toolCall.Arguments)

	parts := splitToolName(toolCall.Name)
	if len(parts) == 2 && parts[0] == "Inner_blog" {
		data := CallInnerTools(parts[1], toolCall.Arguments)
		if data == "" {
			return MCPToolResponse{
				Success: false,
				Error:   "Error NOT find tool: " + toolCall.Name,
			}
		}
		return MCPToolResponse{
			Success: true,
			Result:  data,
		}
	}

	// 尝试直接调用
	data := CallInnerTools(toolCall.Name, toolCall.Arguments)
	if data != "" && !strings.HasPrefix(data, "Error NOT find callback:") {
		return MCPToolResponse{
			Success: true,
			Result:  data,
		}
	}

	return MCPToolResponse{
		Success: false,
		Error:   fmt.Sprintf("Tool '%s' not found", toolCall.Name),
	}
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
