package main

import (
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
	}
}

// GetAllAgents 获取所有在线 agent 信息
func (r *Registry) GetAllAgents() []map[string]any {
	if r.server == nil {
		return nil
	}
	return r.server.GetAllAgents()
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
