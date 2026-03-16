package agentbase

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// ToolCatalog 工具目录管理
// 从 gateway HTTP API 获取所有在线 agent 的工具列表
type ToolCatalog struct {
	gatewayHTTP string // Gateway HTTP 地址（如 http://127.0.0.1:10086）

	// 工具目录: tool_name → agent_id
	catalog map[string]string
	mu      sync.RWMutex

	stopCh chan struct{}
}

// NewToolCatalog 创建工具目录管理器
func NewToolCatalog(gatewayHTTP string) *ToolCatalog {
	return &ToolCatalog{
		gatewayHTTP: gatewayHTTP,
		catalog:     make(map[string]string),
		stopCh:      make(chan struct{}),
	}
}

// Discover 从 gateway 获取工具目录
// excludeAgentID: 排除指定 agent 的工具（通常是自己）
func (tc *ToolCatalog) Discover(excludeAgentID string) error {
	url := fmt.Sprintf("%s/api/gateway/tools", tc.gatewayHTTP)

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %v", err)
	}

	var result struct {
		Success bool              `json:"success"`
		Tools   []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %v", err)
	}

	catalog := make(map[string]string)
	for _, raw := range result.Tools {
		var tool struct {
			AgentID string `json:"agent_id"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal(raw, &tool); err != nil {
			continue
		}
		// 排除指定 agent 的工具
		if tool.AgentID == excludeAgentID {
			continue
		}
		catalog[tool.Name] = tool.AgentID
	}

	tc.mu.Lock()
	tc.catalog = catalog
	tc.mu.Unlock()

	log.Printf("[ToolCatalog] discovered %d tools from gateway", len(catalog))
	return nil
}

// StartRefreshLoop 后台定时刷新工具目录
// interval: 刷新间隔（建议 60s）
// excludeAgentID: 排除指定 agent 的工具
func (tc *ToolCatalog) StartRefreshLoop(interval time.Duration, excludeAgentID string) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-tc.stopCh:
				return
			case <-ticker.C:
				if err := tc.Discover(excludeAgentID); err != nil {
					log.Printf("[ToolCatalog] refresh failed: %v", err)
				}
			}
		}
	}()
}

// GetAgentID 根据工具名获取对应的 agent ID
func (tc *ToolCatalog) GetAgentID(toolName string) (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	agentID, exists := tc.catalog[toolName]
	return agentID, exists
}

// GetAll 获取所有工具目录（副本）
func (tc *ToolCatalog) GetAll() map[string]string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	result := make(map[string]string, len(tc.catalog))
	for k, v := range tc.catalog {
		result[k] = v
	}
	return result
}

// Stop 停止刷新循环
func (tc *ToolCatalog) Stop() {
	close(tc.stopCh)
}
