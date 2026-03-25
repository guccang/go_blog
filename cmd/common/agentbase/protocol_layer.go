package agentbase

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"uap"
)

// ProtocolLayerConfig 协议层配置
type ProtocolLayerConfig struct {
	TargetAgentID  string                           // 目标 agent ID（如 blog-agent backend）
	BuildRegister  func() interface{}               // 自定义注册消息构建器
	BuildHeartbeat func() interface{}               // 自定义心跳消息构建器（可选）
	OnRegisterAck  func(success bool, error string) // 注册确认回调（可选）
}

// ProtocolLayer 协议层管理
// 负责向目标 agent 注册和发送心跳
type ProtocolLayer struct {
	agent      *AgentBase
	cfg        *ProtocolLayerConfig
	registered bool
	regMu      sync.Mutex
	stopCh     chan struct{}
}

// EnableProtocolLayer 启用协议层
func (ab *AgentBase) EnableProtocolLayer(cfg *ProtocolLayerConfig) {
	pl := &ProtocolLayer{
		agent:  ab,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
	ab.protocolLayer = pl

	// 注册 register_ack 处理器
	ab.RegisterHandler(uap.MsgRegisterAck, pl.handleRegisterAck)
}

// StartProtocolLayer 启动协议层（等待连接、注册、心跳循环）
// 应在单独的 goroutine 中调用
func (ab *AgentBase) StartProtocolLayer() {
	if ab.protocolLayer == nil {
		log.Printf("[AgentBase] protocol layer not enabled")
		return
	}
	ab.protocolLayer.start()
}

// start 启动协议层主循环
func (pl *ProtocolLayer) start() {
	// 等待 UAP 连接就绪
	for !pl.agent.IsConnected() {
		time.Sleep(100 * time.Millisecond)
	}

	// 发送初始注册
	pl.sendRegister()

	// 启动心跳循环
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pl.stopCh:
			return
		case <-ticker.C:
			// 检查连接状态
			if !pl.agent.IsConnected() {
				// 断线后等待重连
				pl.regMu.Lock()
				pl.registered = false
				pl.regMu.Unlock()

				for !pl.agent.IsConnected() {
					select {
					case <-pl.stopCh:
						return
					case <-time.After(1 * time.Second):
					}
				}

				// 重连后重新注册
				pl.sendRegister()
				continue
			}

			// 检查注册状态
			pl.regMu.Lock()
			registered := pl.registered
			pl.regMu.Unlock()

			if !registered {
				// 未注册成功（目标 agent 可能晚启动），重试注册
				log.Printf("[ProtocolLayer] target agent not registered yet, retrying...")
				pl.sendRegister()
				continue
			}

			// 发送心跳
			pl.sendHeartbeat()
		}
	}
}

// sendRegister 发送注册消息
func (pl *ProtocolLayer) sendRegister() {
	payload := pl.cfg.BuildRegister()
	if err := pl.agent.SendMsg(pl.cfg.TargetAgentID, uap.MsgRegister, payload); err != nil {
		log.Printf("[ProtocolLayer] send register failed: %v", err)
	}
}

// sendHeartbeat 发送心跳消息
func (pl *ProtocolLayer) sendHeartbeat() {
	var payload interface{}
	if pl.cfg.BuildHeartbeat != nil {
		payload = pl.cfg.BuildHeartbeat()
	} else {
		// 默认心跳载荷
		payload = uap.HeartbeatPayload{
			AgentID: pl.agent.AgentID,
		}
	}
	if err := pl.agent.SendMsg(pl.cfg.TargetAgentID, uap.MsgHeartbeat, payload); err != nil {
		log.Printf("[ProtocolLayer] send heartbeat failed: %v", err)
	}
}

// handleRegisterAck 处理注册确认
func (pl *ProtocolLayer) handleRegisterAck(msg *uap.Message) {
	// 只处理来自目标 agent 的 ack
	if msg.From != pl.cfg.TargetAgentID {
		return
	}

	var ack struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	json.Unmarshal(msg.Payload, &ack)

	pl.regMu.Lock()
	pl.registered = ack.Success
	pl.regMu.Unlock()

	if ack.Success {
		log.Printf("[ProtocolLayer] registered with target agent: %s", pl.cfg.TargetAgentID)
	} else {
		log.Printf("[ProtocolLayer] register rejected: %s", ack.Error)
	}

	// 调用自定义回调
	if pl.cfg.OnRegisterAck != nil {
		pl.cfg.OnRegisterAck(ack.Success, ack.Error)
	}
}

// Stop 停止协议层
func (pl *ProtocolLayer) Stop() {
	close(pl.stopCh)
}

// IsRegistered 是否已注册成功
func (pl *ProtocolLayer) IsRegistered() bool {
	pl.regMu.Lock()
	defer pl.regMu.Unlock()
	return pl.registered
}
