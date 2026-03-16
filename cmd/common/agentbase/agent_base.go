package agentbase

import (
	"sync"

	"uap"
)

// MessageHandler 消息处理器函数签名
type MessageHandler func(msg *uap.Message)

// Config AgentBase 配置
type Config struct {
	ServerURL   string         // Gateway WebSocket URL
	AgentID     string         // Agent 唯一标识
	AgentType   string         // Agent 类型
	AgentName   string         // 人类可读名称
	Description string         // Agent 能力简述
	AuthToken   string         // 认证令牌
	Capacity    int            // 最大并发数
	Tools       []uap.ToolDef  // 注册的工具列表
	Meta        map[string]any // 扩展字段
}

// AgentBase Agent 基础连接管理
// 提供 UAP 客户端包装和消息分发能力
type AgentBase struct {
	Client *uap.Client // UAP 客户端（公开以便直接访问）

	// 配置
	AgentID   string
	AgentType string
	AgentName string

	// 消息处理器注册表
	handlers map[string]MessageHandler
	handlerMu sync.RWMutex

	// 协议层（可选）
	protocolLayer *ProtocolLayer
}

// NewAgentBase 创建 AgentBase 实例
func NewAgentBase(cfg *Config) *AgentBase {
	client := uap.NewClient(cfg.ServerURL, cfg.AgentID, cfg.AgentType, cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Description = cfg.Description
	client.Capacity = cfg.Capacity
	client.Tools = cfg.Tools
	client.Meta = cfg.Meta

	ab := &AgentBase{
		Client:    client,
		AgentID:   cfg.AgentID,
		AgentType: cfg.AgentType,
		AgentName: cfg.AgentName,
		handlers:  make(map[string]MessageHandler),
	}

	// 设置 UAP 消息回调
	client.OnMessage = ab.dispatch

	return ab
}

// RegisterHandler 注册消息处理器
func (ab *AgentBase) RegisterHandler(msgType string, handler MessageHandler) {
	ab.handlerMu.Lock()
	defer ab.handlerMu.Unlock()
	ab.handlers[msgType] = handler
}

// dispatch 消息分发（内部使用）
func (ab *AgentBase) dispatch(msg *uap.Message) {
	ab.handlerMu.RLock()
	handler, exists := ab.handlers[msg.Type]
	ab.handlerMu.RUnlock()

	if exists {
		handler(msg)
	}
}

// Run 启动连接（阻塞，自动重连）
func (ab *AgentBase) Run() {
	ab.Client.Run()
}

// Stop 停止连接
func (ab *AgentBase) Stop() {
	ab.Client.Stop()
}

// IsConnected 是否已连接到 gateway
func (ab *AgentBase) IsConnected() bool {
	return ab.Client.IsConnected()
}

// SendMsg 发送消息到指定 agent
func (ab *AgentBase) SendMsg(toAgentID, msgType string, payload any) error {
	return ab.Client.SendTo(toAgentID, msgType, payload)
}
