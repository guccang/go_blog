package agentbase

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"uap"
)

// RemoteCaller 远程工具调用组件
// 封装 pending channel 模式，提供请求-响应关联能力
// 不自动注册 handler，由各 agent 在自己的 handler 中调用 DispatchToolResult/DispatchError
type RemoteCaller struct {
	ab      *AgentBase
	catalog *ToolCatalog

	pending map[string]chan *uap.ToolResultPayload
	pendMu  sync.Mutex
}

// NewRemoteCaller 创建远程调用组件
func NewRemoteCaller(ab *AgentBase, catalog *ToolCatalog) *RemoteCaller {
	return &RemoteCaller{
		ab:      ab,
		catalog: catalog,
		pending: make(map[string]chan *uap.ToolResultPayload),
	}
}

// CallTool 发送工具调用并等待响应
func (rc *RemoteCaller) CallTool(toolName string, args json.RawMessage, timeout time.Duration) (result string, agentID string, err error) {
	agentID, ok := rc.catalog.GetAgentID(toolName)
	if !ok {
		return "", "", fmt.Errorf("tool %s not found in catalog", toolName)
	}

	msgID := uap.NewMsgID()
	ch := make(chan *uap.ToolResultPayload, 1)

	rc.pendMu.Lock()
	rc.pending[msgID] = ch
	rc.pendMu.Unlock()

	defer func() {
		rc.pendMu.Lock()
		delete(rc.pending, msgID)
		rc.pendMu.Unlock()
	}()

	log.Printf("[RemoteCaller] tool_call → agent=%s tool=%s", agentID, toolName)

	// 发送 tool_call
	sendErr := rc.ab.Client.Send(&uap.Message{
		Type: uap.MsgToolCall,
		ID:   msgID,
		From: rc.ab.AgentID,
		To:   agentID,
		Payload: mustMarshal(uap.ToolCallPayload{
			ToolName:  toolName,
			Arguments: args,
		}),
		Ts: time.Now().UnixMilli(),
	})
	if sendErr != nil {
		return "", agentID, fmt.Errorf("send tool_call: %v", sendErr)
	}

	// 等待结果
	select {
	case res := <-ch:
		if !res.Success {
			return "", agentID, fmt.Errorf("tool error: %s", res.Error)
		}
		log.Printf("[RemoteCaller] tool_result ← agent=%s tool=%s resultLen=%d", agentID, toolName, len(res.Result))
		return res.Result, agentID, nil
	case <-time.After(timeout):
		return "", agentID, fmt.Errorf("tool %s timeout after %v", toolName, timeout)
	}
}

// CallToolWithRetry 带瞬态错误重试的工具调用（重试一次）
func (rc *RemoteCaller) CallToolWithRetry(toolName string, args json.RawMessage, timeout time.Duration) (string, string, error) {
	result, agentID, err := rc.CallTool(toolName, args, timeout)
	if err != nil && isTransientError(err) {
		log.Printf("[RemoteCaller] tool %s transient error, retrying: %v", toolName, err)
		time.Sleep(1 * time.Second)
		result, agentID, err = rc.CallTool(toolName, args, timeout)
	}
	return result, agentID, err
}

// DispatchToolResult 分发 tool_result 到 pending channel
// 返回 true 表示匹配到了 pending 请求
func (rc *RemoteCaller) DispatchToolResult(payload *uap.ToolResultPayload) bool {
	rc.pendMu.Lock()
	ch, ok := rc.pending[payload.RequestID]
	rc.pendMu.Unlock()
	if ok {
		ch <- payload
		return true
	}
	return false
}

// DispatchError 分发 error 到 pending channel
// gateway 使用原始 msg.ID 作为错误消息的 ID
// 返回 true 表示匹配到了 pending 请求
func (rc *RemoteCaller) DispatchError(msgID, errMsg string) bool {
	rc.pendMu.Lock()
	ch, ok := rc.pending[msgID]
	rc.pendMu.Unlock()
	if ok {
		ch <- &uap.ToolResultPayload{
			RequestID: msgID,
			Success:   false,
			Error:     errMsg,
		}
		return true
	}
	return false
}

// isTransientError 判断是否是瞬态网络错误（值得重试）
func isTransientError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "not connected")
}

// mustMarshal JSON 序列化，失败返回空对象
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
