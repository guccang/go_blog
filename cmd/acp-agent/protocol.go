package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// ACPProtocol ACP 协议处理器
type ACPProtocol struct {
	agent *ACPAgent
}

// NewACPProtocol 创建新的 ACP 协议处理器
func NewACPProtocol(agent *ACPAgent) *ACPProtocol {
	return &ACPProtocol{
		agent: agent,
	}
}

// ParseMessage 解析原始消息为 ACPMessage
func (p *ACPProtocol) ParseMessage(data []byte) (*ACPMessage, error) {
	var msg ACPMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("解析 ACP 消息失败: %v", err)
	}

	// 验证必需字段
	if msg.Type == "" {
		return nil, fmt.Errorf("消息类型不能为空")
	}

	// 设置时间戳（如果未提供）
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().Unix()
	}

	return &msg, nil
}

// SerializeMessage 序列化 ACPMessage 为 JSON
func (p *ACPProtocol) SerializeMessage(msg *ACPMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("序列化 ACP 消息失败: %v", err)
	}
	return data, nil
}

// ValidateMessage 验证消息的有效性
func (p *ACPProtocol) ValidateMessage(msg *ACPMessage) error {
	// 检查消息类型是否有效
	validTypes := map[string]bool{
		"ping":                true,
		"pong":                true,
		"code_execution":      true,
		"code_execution_result": true,
		"tool_call":           true,
		"tool_call_result":    true,
		"session_management":  true,
		"session_management_result": true,
		"error":               true,
		"welcome":             true,
		"cloud_code_request":  true,
		"cloud_code_response": true,
		"client_code_x_request": true,
		"client_code_x_response": true,
	}

	if !validTypes[msg.Type] {
		return fmt.Errorf("无效的消息类型: %s", msg.Type)
	}

	// 根据消息类型进行额外验证
	switch msg.Type {
	case "code_execution":
		if msg.Content == nil {
			return fmt.Errorf("code_execution 消息必须包含 content 字段")
		}
	case "tool_call":
		if msg.Content == nil {
			return fmt.Errorf("tool_call 消息必须包含 content 字段")
		}
	case "session_management":
		if msg.Content == nil {
			return fmt.Errorf("session_management 消息必须包含 content 字段")
		}
	}

	return nil
}

// CreateResponse 创建响应消息
func (p *ACPProtocol) CreateResponse(originalMsg *ACPMessage, content interface{}, errMsg string) *ACPMessage {
	response := &ACPMessage{
		Type:      originalMsg.Type + "_result",
		SessionID: originalMsg.SessionID,
		RequestID: originalMsg.RequestID,
		Content:   content,
		Timestamp: time.Now().Unix(),
	}

	if errMsg != "" {
		response.Type = "error"
		response.Error = errMsg
	}

	return response
}

// CreateWelcomeMessage 创建欢迎消息
func (p *ACPProtocol) CreateWelcomeMessage(sessionID string) *ACPMessage {
	return &ACPMessage{
		Type:      "welcome",
		SessionID: sessionID,
		Content: map[string]interface{}{
			"message":     "欢迎连接到 ACP Agent",
			"version":     "1.0.0",
			"protocol":    "ACP/1.0",
			"server_time": time.Now().Unix(),
			"capabilities": []string{
				"code_execution",
				"tool_call",
				"session_management",
				"cloud_code_integration",
				"client_code_x_integration",
			},
		},
		Timestamp: time.Now().Unix(),
	}
}

// CreatePingMessage 创建心跳消息
func (p *ACPProtocol) CreatePingMessage() *ACPMessage {
	return &ACPMessage{
		Type:      "ping",
		Timestamp: time.Now().Unix(),
	}
}

// CreatePongMessage 创建心跳响应
func (p *ACPProtocol) CreatePongMessage(requestID string) *ACPMessage {
	return &ACPMessage{
		Type:      "pong",
		RequestID: requestID,
		Timestamp: time.Now().Unix(),
	}
}

// CreateErrorMessage 创建错误消息
func (p *ACPProtocol) CreateErrorMessage(sessionID, requestID, errorMsg string) *ACPMessage {
	return &ACPMessage{
		Type:      "error",
		SessionID: sessionID,
		RequestID: requestID,
		Error:     errorMsg,
		Timestamp: time.Now().Unix(),
	}
}
