package main

import "encoding/json"

// ExecutionResult Python 代码执行结果
type ExecutionResult struct {
	Success    bool             `json:"success"`
	Stdout     string           `json:"stdout"`
	Stderr     string           `json:"stderr,omitempty"`
	ExitCode   int              `json:"exit_code"`
	DurationMs int64            `json:"duration_ms"`
	ErrorType  string           `json:"error_type,omitempty"` // "syntax"|"runtime"|"timeout"|"output_truncated"
	ToolCalls  []ToolCallRecord `json:"tool_calls"`
	Truncated  bool             `json:"truncated,omitempty"`
}

// ToolCallRecord 单次工具调用记录
type ToolCallRecord struct {
	Tool     string `json:"tool"`
	AgentID  string `json:"agent_id,omitempty"`
	Success  bool   `json:"success"`
	Duration int64  `json:"duration_ms"`
	Error    string `json:"error,omitempty"`
}

// toolCallRequest Python 沙箱发送的工具调用请求
type toolCallRequest struct {
	Type string          `json:"type"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

// toolCallResponse 返回给 Python 沙箱的工具调用结果
type toolCallResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// mustMarshalJSON 将值序列化为 JSON，失败时返回空对象
func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
