package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CloudCodeClient Cloud Code 客户端
type CloudCodeClient struct {
	config   CloudCodeConfig
	client   *http.Client
	baseURL  string
}

// NewCloudCodeClient 创建新的 Cloud Code 客户端
func NewCloudCodeClient(config CloudCodeConfig) *CloudCodeClient {
	baseURL := fmt.Sprintf("http://%s:%d", config.Host, config.Port)
	
	return &CloudCodeClient{
		config:  config,
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		baseURL: baseURL,
	}
}

// ExecuteCode 执行代码
func (c *CloudCodeClient) ExecuteCode(code string, language string) (map[string]interface{}, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("Cloud Code 集成未启用")
	}

	requestBody := map[string]interface{}{
		"code":     code,
		"language": language,
		"timeout":  30,
	}

	return c.makeRequest("/api/execute", requestBody)
}

// CreateSession 创建会话
func (c *CloudCodeClient) CreateSession(sessionType string) (map[string]interface{}, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("Cloud Code 集成未启用")
	}

	requestBody := map[string]interface{}{
		"type": sessionType,
	}

	return c.makeRequest("/api/session/create", requestBody)
}

// GetSessionStatus 获取会话状态
func (c *CloudCodeClient) GetSessionStatus(sessionID string) (map[string]interface{}, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("Cloud Code 集成未启用")
	}

	return c.makeRequest(fmt.Sprintf("/api/session/%s/status", sessionID), nil)
}

// CloseSession 关闭会话
func (c *CloudCodeClient) CloseSession(sessionID string) (map[string]interface{}, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("Cloud Code 集成未启用")
	}

	return c.makeRequest(fmt.Sprintf("/api/session/%s/close", sessionID), nil)
}

// makeRequest 发送 HTTP 请求
func (c *CloudCodeClient) makeRequest(endpoint string, body interface{}) (map[string]interface{}, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("编码请求体失败: %v", err)
		}
	}

	url := c.baseURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败: %s - %s", resp.Status, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return result, nil
}

// HandleCloudCodeRequest 处理 Cloud Code 请求
func (a *ACPAgent) HandleCloudCodeRequest(client *ACPClient, msg ACPMessage) {
	if !a.Config.CloudCode.Enabled {
		errorMsg := ACPMessage{
			Type:      "error",
			SessionID: client.ID,
			RequestID: msg.RequestID,
			Error:     "Cloud Code 集成未启用",
			Timestamp: time.Now().Unix(),
		}
		a.sendMessage(client.Conn, errorMsg)
		return
	}

	cloudCodeClient := NewCloudCodeClient(a.Config.CloudCode)
	
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
	
	var result map[string]interface{}
	var err error

	switch action {
	case "execute_code":
		code, _ := content["code"].(string)
		language, _ := content["language"].(string)
		if language == "" {
			language = "python"
		}
		result, err = cloudCodeClient.ExecuteCode(code, language)
		
	case "create_session":
		sessionType, _ := content["session_type"].(string)
		if sessionType == "" {
			sessionType = "default"
		}
		result, err = cloudCodeClient.CreateSession(sessionType)
		
	case "get_session_status":
		sessionID, _ := content["session_id"].(string)
		result, err = cloudCodeClient.GetSessionStatus(sessionID)
		
	case "close_session":
		sessionID, _ := content["session_id"].(string)
		result, err = cloudCodeClient.CloseSession(sessionID)
		
	default:
		err = fmt.Errorf("未知的 Cloud Code 操作: %s", action)
	}

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
		Type:      "cloud_code_response",
		SessionID: client.ID,
		RequestID: msg.RequestID,
		Content:   result,
		Timestamp: time.Now().Unix(),
	}
	a.sendMessage(client.Conn, response)
}
