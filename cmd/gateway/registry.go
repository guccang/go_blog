package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"uap"
)

// Registry Agent 注册表
type Registry struct {
	server *uap.Server
	mu     sync.RWMutex
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{}
}

// SetServer 绑定 UAP server
func (r *Registry) SetServer(server *uap.Server) {
	r.server = server

	// 注册回调
	server.OnAgentOnline = func(agent *uap.AgentConn) {
		log.Printf("[Registry] agent online: %s (type=%s, name=%s)", agent.ID, agent.AgentType, agent.Name)
	}
	server.OnAgentOffline = func(agent *uap.AgentConn) {
		log.Printf("[Registry] agent offline: %s (type=%s, name=%s)", agent.ID, agent.AgentType, agent.Name)
		// 广播 agent_offline 通知给所有其他在线 agent，使其立即移除离线 agent
		r.broadcastAgentOffline(agent)
	}
}

// GetAllAgents 获取所有在线 agent 信息
func (r *Registry) GetAllAgents() []map[string]any {
	if r.server == nil {
		return nil
	}
	return r.server.GetAllAgents()
}

// GetAllTools 获取所有在线 agent 的完整工具定义
func (r *Registry) GetAllTools() []map[string]any {
	if r.server == nil {
		return nil
	}
	return r.server.GetAllTools()
}

// OnlineCount 在线 agent 数量
func (r *Registry) OnlineCount() int {
	agents := r.GetAllAgents()
	return len(agents)
}

// GetAgent 获取指定 agent
func (r *Registry) GetAgent(agentID string) *uap.AgentConn {
	if r.server == nil {
		return nil
	}
	return r.server.GetAgent(agentID)
}

// GetAgentsByType 按类型获取 agent
func (r *Registry) GetAgentsByType(agentType string) []*uap.AgentConn {
	if r.server == nil {
		return nil
	}
	return r.server.GetAgentsByType(agentType)
}

// StartHealthCheck 启动健康检查
func (r *Registry) StartHealthCheck(timeout time.Duration) {
	if r.server != nil {
		r.server.StartHealthCheck(timeout)
	}
}

// broadcastAgentOffline 向所有其他在线 agent 广播某 agent 离线通知
func (r *Registry) broadcastAgentOffline(offlineAgent *uap.AgentConn) {
	if r.server == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"event":      "agent_offline",
		"agent_id":   offlineAgent.ID,
		"agent_type": offlineAgent.AgentType,
		"agent_name": offlineAgent.Name,
	})
	agents := r.server.GetAllAgents()
	for _, a := range agents {
		id, _ := a["agent_id"].(string)
		if id == "" || id == offlineAgent.ID {
			continue
		}
		err := r.server.SendToAgent(id, &uap.Message{
			Type:    uap.MsgNotify,
			From:    "gateway",
			To:      id,
			Payload: payload,
		})
		if err != nil {
			log.Printf("[Registry] failed to notify %s about agent_offline: %v", id, err)
		}
	}
}
