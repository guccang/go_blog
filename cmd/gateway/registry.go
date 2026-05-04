package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
func (r *Registry) SetServer(server *uap.Server, tracker *Tracker) {
	r.server = server

	// 注册回调
	server.OnAgentOnline = func(agent *uap.AgentConn) {
		log.Printf("[Registry] agent online: %s (type=%s, name=%s)", agent.ID, agent.AgentType, agent.Name)
		if tracker != nil {
			tracker.RecordLifecycle(EventKindAgentOn, agent, fmt.Sprintf("type=%s", agent.AgentType))
		}
		r.broadcastAgentLifecycle("agent_online", agent)
	}
	server.OnAgentOffline = func(agent *uap.AgentConn) {
		log.Printf("[Registry] agent offline: %s (type=%s, name=%s)", agent.ID, agent.AgentType, agent.Name)
		if tracker != nil {
			tracker.RecordLifecycle(EventKindAgentOff, agent, fmt.Sprintf("type=%s", agent.AgentType))
		}
		r.broadcastAgentLifecycle("agent_offline", agent)
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

// broadcastAgentLifecycle 向所有其他在线 agent 广播 agent 生命周期通知。
func (r *Registry) broadcastAgentLifecycle(event string, agent *uap.AgentConn) {
	if r.server == nil {
		return
	}
	event = strings.TrimSpace(event)
	if event == "" || agent == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"event":      event,
		"agent_id":   agent.ID,
		"agent_type": agent.AgentType,
		"agent_name": agent.Name,
	})
	agents := r.server.GetAllAgents()
	for _, a := range agents {
		id, _ := a["agent_id"].(string)
		if id == "" || id == agent.ID {
			continue
		}
		err := r.server.SendToAgent(id, &uap.Message{
			Type:    uap.MsgNotify,
			From:    "gateway",
			To:      id,
			Payload: payload,
		})
		if err != nil {
			log.Printf("[Registry] failed to notify %s about %s: %v", id, event, err)
		}
	}
}
