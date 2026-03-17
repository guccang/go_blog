package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// ClientCodeXClient Client Code X 客户端
type ClientCodeXClient struct {
	config    ClientCodeXConfig
	conn      *websocket.Conn
	connected bool
}

// NewClientCodeXClient 创建新的 Client Code X 客户端
func NewClientCodeXClient(config ClientCodeXConfig) *ClientCodeXClient {
	return &ClientCodeXClient{
		config:    config,
		connected: false,
	}
}

// Connect 连接到 Client Code X
func (c *ClientCodeXClient) Connect() error {
	if !c.config.Enabled {
		return fmt.Errorf("Client Code X 集成未启用")
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := make(map[string][]string)
	if c.config.AuthToken != "" {
		headers["Authorization"] = []string{"Bearer " + c.config.AuthToken}
	}

	conn, _, err := dialer.Dial(c.config.Endpoint, headers)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}

	c.conn = conn
	c.connected = true

	return nil
}

// Disconnect 断开连接
func (c *ClientCodeXClient) Disconnect() error {
	if c.connected && c.conn != nil {
		err := c.conn.Close()
		c.connected = false
		return err
	}
	return nil
}

// SendMessage 发送消息到 Client Code X
func (c *ClientCodeXClient) SendMessage(message map[string]interface{}) error {
	if !c.connected {
		return fmt.Errorf("未连接到 Client Code X")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("编码消息失败: %v", err)
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// ReceiveMessage 接收消息
func (c *ClientCodeXClient) ReceiveMessage() (map[string]interface{}, error) {
	if !c.connected {
		return nil, fmt.Errorf("未连接到 Client Code X")
	}

	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("接收消息失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(message, &result); err != nil {
		return nil, fmt.Errorf("解析消息失败: %v", err)
	}

	return result, nil
}

// ExecuteCommand 执行命令
func (c *ClientCodeXClient) ExecuteCommand(command string, args map[string]interface{}) (map[string]interface{}, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	message := map[string]interface{}{
		"type":    "command",
		"command": command,
		"args":    args,
		"time":    time.Now().Unix(),
	}

	if err := c.SendMessage(message); err != nil {
		return nil, err
	}

	return c.ReceiveMessage()
}

// HandleClientCodeXRequest 处理 Client Code X 请求
func (a *ACPAgent) HandleClientCodeXRequest(client *ACPClient, msg ACPMessage) {
	if !a.Config.ClientCodeX.Enabled {
		errorMsg := ACPMessage{
			Type:      "error",
			SessionID: client.ID,
			RequestID: msg.RequestID,
			Error:     "Client Code X 集成未启用",
			Timestamp: time.Now().Unix(),
		}
		a.sendMessage(client.Conn, errorMsg)
		return
	}

	clientCodeX := NewClientCodeXClient(a.Config.ClientCodeX)
	
	// 根据请求内容处理
	content, ok := msg.Content.(map[string]interface{})
	if !ok {
		errorMsg := ACPMessage{
			Type:      "error",
			SessionID: client.ID,
			RequestID: msg.RequestID,
			Error:     "无效的请求内容",
			Timestamp: time.Now().Unix(),
		}
		a.sendMessage(client.Conn, errorMsg)
		return
	}

	action, _ := content["action"].(string)
	command, _ := content["command"].(string)
	args, _ := content["args"].(map[string]interface{})
	
	var result map[string]interface{}
	var err error

	switch action {
	case "execute_command":
		if command == "" {
			err = fmt.Errorf("命令不能为空")
		} else {
			result, err = clientCodeX.ExecuteCommand(command, args)
		}
		
	case "send_message":
		message, _ := content["message"].(map[string]interface{})
		if message == nil {
			err = fmt.Errorf("消息不能为空")
		} else {
			err = clientCodeX.SendMessage(message)
			if err == nil {
				result = map[string]interface{}{
					"status":  "success",
					"message": "消息发送成功",
				}
			}
		}
		
	case "receive_message":
		result, err = clientCodeX.ReceiveMessage()
		
	default:
		err = fmt.Errorf("未知的 Client Code X 操作: %s", action)
	}

	// 确保断开连接
	defer clientCodeX.Disconnect()

	if err != nil {
		errorMsg := ACPMessage{
			Type:      "error",
			SessionID: client.ID,
			RequestID: msg.RequestID,
			Error:     err.Error(),
			Timestamp: time.Now().Unix(),
		}
		a.sendMessage(client.Conn, errorMsg)
		return
	}

	response := ACPMessage{
		Type:      "client_code_x_response",
		SessionID: client.ID,
		RequestID: msg.RequestID,
		Content:   result,
		Timestamp: time.Now().Unix(),
	}
	a.sendMessage(client.Conn, response)
}
