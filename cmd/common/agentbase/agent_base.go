package agentbase

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

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
// 提供 UAP 客户端包装、消息分发和生命周期管理能力
type AgentBase struct {
	Client *uap.Client // UAP 客户端（公开以便直接访问）

	// 配置
	AgentID   string
	AgentType string
	AgentName string
	Capacity  int

	// 生命周期
	lifecycle *Lifecycle
	startTime time.Time

	// 可选回调（Agent 自行设置）
	ActiveTaskCounter func() int // 返回活跃任务数（drain 轮询用）
	OnShutdown        func()     // shutdown 时的自定义回调（如通知业务层停止接收）

	// 消息处理器注册表
	handlers  map[string]MessageHandler
	handlerMu sync.RWMutex

	// 协议层（可选）
	protocolLayer *ProtocolLayer

	// shutdown 内部通道
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
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
		Client:     client,
		AgentID:    cfg.AgentID,
		AgentType:  cfg.AgentType,
		AgentName:  cfg.AgentName,
		Capacity:   cfg.Capacity,
		lifecycle:  NewLifecycle(),
		startTime:  time.Now(),
		handlers:   make(map[string]MessageHandler),
		shutdownCh: make(chan struct{}),
	}

	// 设置 UAP 消息回调
	client.OnMessage = ab.dispatch

	// 设置注册成功回调：starting → running
	client.OnRegistered = func(success bool) {
		if success {
			ab.lifecycle.TransitionTo(StateRunning)
		}
	}

	// 内置注册 ctrl_shutdown 处理器
	ab.handlers[uap.MsgCtrlShutdown] = ab.handleCtrlShutdown

	// 内置注册 ctrl_status 处理器
	ab.handlers[uap.MsgCtrlStatus] = ab.handleCtrlStatus

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
	// ctrl_* 前缀消息始终处理（不受状态限制）
	if strings.HasPrefix(msg.Type, "ctrl_") {
		ab.handlerMu.RLock()
		handler, exists := ab.handlers[msg.Type]
		ab.handlerMu.RUnlock()
		if exists {
			handler(msg)
		}
		return
	}

	// draining 状态下自动拒绝新 task_assign
	if ab.lifecycle.State() == StateDraining && msg.Type == uap.MsgTaskAssign {
		var p struct {
			TaskID string `json:"task_id"`
		}
		json.Unmarshal(msg.Payload, &p)
		log.Printf("[AgentBase] draining: rejecting task_assign task=%s", p.TaskID)
		ab.Client.SendTo(msg.From, uap.MsgTaskRejected, uap.TaskRejectedPayload{
			TaskID: p.TaskID,
			Reason: "agent is shutting down",
		})
		return
	}

	// draining 状态下自动拒绝新 tool_call
	if ab.lifecycle.State() == StateDraining && msg.Type == uap.MsgToolCall {
		log.Printf("[AgentBase] draining: rejecting tool_call from=%s", msg.From)
		ab.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "agent is shutting down",
		})
		return
	}

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

// Lifecycle 返回生命周期状态机
func (ab *AgentBase) Lifecycle() *Lifecycle {
	return ab.lifecycle
}

// InitiateShutdown 发起优雅关闭（供信号处理器或外部调用）
func (ab *AgentBase) InitiateShutdown(reason string) {
	ab.executeShutdown(reason, 30, false)
}

// ========================= 控制协议处理器 =========================

// handleCtrlShutdown 处理 ctrl_shutdown 消息
func (ab *AgentBase) handleCtrlShutdown(msg *uap.Message) {
	var payload uap.CtrlShutdownPayload
	json.Unmarshal(msg.Payload, &payload)

	activeTasks := 0
	if ab.ActiveTaskCounter != nil {
		activeTasks = ab.ActiveTaskCounter()
	}

	log.Printf("[AgentBase] received ctrl_shutdown from=%s reason=%q force=%v timeout=%d active_tasks=%d",
		msg.From, payload.Reason, payload.Force, payload.TimeoutSec, activeTasks)

	// 回复 ack
	ab.Client.SendTo(msg.From, uap.MsgCtrlShutdownAck, uap.CtrlShutdownAckPayload{
		AgentID:      ab.AgentID,
		Accepted:     true,
		CurrentState: ab.lifecycle.State(),
		ActiveTasks:  activeTasks,
	})

	// 执行 shutdown
	timeout := payload.TimeoutSec
	if timeout <= 0 {
		timeout = 30
	}
	go ab.executeShutdown(payload.Reason, timeout, payload.Force)
}

// handleCtrlStatus 处理 ctrl_status 消息
func (ab *AgentBase) handleCtrlStatus(msg *uap.Message) {
	activeTasks := 0
	if ab.ActiveTaskCounter != nil {
		activeTasks = ab.ActiveTaskCounter()
	}

	uptime := int64(time.Since(ab.startTime).Seconds())

	log.Printf("[AgentBase] received ctrl_status from=%s, reporting state=%s tasks=%d",
		msg.From, ab.lifecycle.State(), activeTasks)

	ab.Client.SendTo(msg.From, uap.MsgCtrlStatusReport, uap.CtrlStatusReportPayload{
		AgentID:     ab.AgentID,
		AgentType:   ab.AgentType,
		AgentName:   ab.AgentName,
		State:       ab.lifecycle.State(),
		ActiveTasks: activeTasks,
		Capacity:    ab.Capacity,
		Uptime:      uptime,
	})
}

// executeShutdown 执行关闭流程
func (ab *AgentBase) executeShutdown(reason string, timeoutSec int, force bool) {
	ab.shutdownOnce.Do(func() {
		close(ab.shutdownCh)

		if force {
			log.Printf("[AgentBase] force shutdown: reason=%q", reason)
			ab.lifecycle.TransitionTo(StateStopped)
			ab.Stop()
			return
		}

		// 进入 draining 状态
		if err := ab.lifecycle.TransitionTo(StateDraining); err != nil {
			// 可能已在 draining 或 stopped，直接退出
			log.Printf("[AgentBase] shutdown transition failed: %v, stopping directly", err)
			ab.lifecycle.TransitionTo(StateStopped)
			ab.Stop()
			return
		}

		log.Printf("[AgentBase] entering drain mode: reason=%q timeout=%ds", reason, timeoutSec)

		// 调用自定义 shutdown 回调
		if ab.OnShutdown != nil {
			ab.OnShutdown()
		}

		// 轮询等待活跃任务完成
		deadline := time.After(time.Duration(timeoutSec) * time.Second)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-deadline:
				activeTasks := 0
				if ab.ActiveTaskCounter != nil {
					activeTasks = ab.ActiveTaskCounter()
				}
				log.Printf("[AgentBase] drain timeout (%ds), stopping with %d active tasks", timeoutSec, activeTasks)
				ab.lifecycle.TransitionTo(StateStopped)
				ab.Stop()
				return
			case <-ticker.C:
				activeTasks := 0
				if ab.ActiveTaskCounter != nil {
					activeTasks = ab.ActiveTaskCounter()
				}
				if activeTasks == 0 {
					log.Printf("[AgentBase] all tasks drained, stopping gracefully")
					ab.lifecycle.TransitionTo(StateStopped)
					ab.Stop()
					return
				}
				log.Printf("[AgentBase] draining: %d active tasks remaining", activeTasks)
			}
		}
	})
}
