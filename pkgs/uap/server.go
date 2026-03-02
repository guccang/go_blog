package uap

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AgentConn 表示一个已连接的 agent
type AgentConn struct {
	ID        string
	AgentType string
	Name      string
	Tools     []ToolDef
	Capacity  int
	Meta      map[string]any
	Conn      *websocket.Conn
	mu        sync.Mutex
	LastHB    time.Time
	Online    bool
}

// Send 向此 agent 发送消息
func (a *AgentConn) Send(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Conn == nil {
		return fmt.Errorf("agent %s not connected", a.ID)
	}
	return a.Conn.WriteMessage(websocket.TextMessage, data)
}

// Close 关闭连接
func (a *AgentConn) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Online = false
	if a.Conn != nil {
		a.Conn.Close()
	}
}

// Server UAP 网关服务端
type Server struct {
	agents   map[string]*AgentConn // agent_id -> AgentConn
	mu       sync.RWMutex
	upgrader websocket.Upgrader

	// AuthToken 验证 token（为空则不验证）
	AuthToken string

	// OnAgentOnline/Offline 回调
	OnAgentOnline  func(agent *AgentConn)
	OnAgentOffline func(agent *AgentConn)

	// OnMessage 处理无法路由的消息（如 To 为空或目标不在线）
	OnMessage func(from *AgentConn, msg *Message)
}

// NewServer 创建 UAP 网关服务
func NewServer() *Server {
	return &Server{
		agents: make(map[string]*AgentConn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// HandleWebSocket HTTP handler，用于接受 agent 的 WebSocket 连接
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[UAP] upgrade error: %v", err)
		return
	}

	log.Printf("[UAP] new WebSocket connection from %s", r.RemoteAddr)

	// 等待 register 消息
	s.handleConn(conn)
}

// handleConn 处理单个 WebSocket 连接的消息循环
func (s *Server) handleConn(conn *websocket.Conn) {
	var agent *AgentConn

	defer func() {
		if agent != nil {
			s.removeAgent(agent)
		}
		conn.Close()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if agent != nil {
				log.Printf("[UAP] agent %s (%s) disconnected: %v", agent.Name, agent.ID, err)
			} else {
				log.Printf("[UAP] connection closed before registration: %v", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[UAP] parse error: %v", err)
			continue
		}

		switch msg.Type {
		case MsgRegister:
			// 如果 To 非空，说明是应用层的 register 消息（如 codegen 协议），应路由而非拦截
			if msg.To != "" && agent != nil {
				msg.From = agent.ID
				s.routeMessage(agent, &msg)
				continue
			}
			if agent != nil {
				log.Printf("[UAP] duplicate register from %s, ignoring", agent.ID)
				continue
			}
			agent = s.handleRegister(conn, &msg)

		case MsgHeartbeat:
			// 如果 To 非空，说明是应用层的 heartbeat 消息，应路由而非拦截
			if msg.To != "" && agent != nil {
				msg.From = agent.ID
				s.routeMessage(agent, &msg)
				continue
			}
			if agent != nil {
				agent.LastHB = time.Now()
				s.sendTo(agent, &Message{
					Type: MsgHeartbeatAck,
					Ts:   time.Now().UnixMilli(),
				})
			}

		default:
			if agent == nil {
				log.Printf("[UAP] message before registration, type=%s, dropping", msg.Type)
				continue
			}
			// 填充 From 字段（gateway 保证）
			msg.From = agent.ID
			s.routeMessage(agent, &msg)
		}
	}
}

// handleRegister 处理 agent 注册
func (s *Server) handleRegister(conn *websocket.Conn, msg *Message) *AgentConn {
	var payload RegisterPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[UAP] invalid register payload: %v", err)
		sendDirect(conn, &Message{
			Type: MsgRegisterAck,
			Payload: mustMarshal(RegisterAckPayload{
				Success: false,
				Error:   "invalid register payload",
			}),
			Ts: time.Now().UnixMilli(),
		})
		return nil
	}

	// 验证 token
	if s.AuthToken != "" && payload.AuthToken != s.AuthToken {
		log.Printf("[UAP] register rejected: invalid token from %s", payload.AgentID)
		sendDirect(conn, &Message{
			Type: MsgRegisterAck,
			Payload: mustMarshal(RegisterAckPayload{
				Success: false,
				Error:   "invalid auth token",
			}),
			Ts: time.Now().UnixMilli(),
		})
		return nil
	}

	// 检查重名
	s.mu.Lock()
	if existing, ok := s.agents[payload.AgentID]; ok && existing.Online {
		s.mu.Unlock()
		log.Printf("[UAP] register rejected: agent %s already online", payload.AgentID)
		sendDirect(conn, &Message{
			Type: MsgRegisterAck,
			Payload: mustMarshal(RegisterAckPayload{
				Success: false,
				Error:   fmt.Sprintf("agent %s already online", payload.AgentID),
			}),
			Ts: time.Now().UnixMilli(),
		})
		return nil
	}

	agent := &AgentConn{
		ID:        payload.AgentID,
		AgentType: payload.AgentType,
		Name:      payload.Name,
		Tools:     payload.Tools,
		Capacity:  payload.Capacity,
		Meta:      payload.Meta,
		Conn:      conn,
		LastHB:    time.Now(),
		Online:    true,
	}
	s.agents[payload.AgentID] = agent
	s.mu.Unlock()

	log.Printf("[UAP] agent registered: %s (type=%s, name=%s, tools=%d)",
		payload.AgentID, payload.AgentType, payload.Name, len(payload.Tools))

	// 发送注册确认
	agent.Send(&Message{
		Type: MsgRegisterAck,
		Payload: mustMarshal(RegisterAckPayload{
			Success: true,
		}),
		Ts: time.Now().UnixMilli(),
	})

	if s.OnAgentOnline != nil {
		s.OnAgentOnline(agent)
	}

	return agent
}

// routeMessage 路由消息：按 To 字段转发
func (s *Server) routeMessage(from *AgentConn, msg *Message) {
	if msg.To == "" {
		// To 为空，交给 OnMessage 回调处理
		if s.OnMessage != nil {
			s.OnMessage(from, msg)
		} else {
			log.Printf("[UAP] message from %s has empty To, dropping (type=%s)", from.ID, msg.Type)
		}
		return
	}

	s.mu.RLock()
	target, ok := s.agents[msg.To]
	s.mu.RUnlock()

	if !ok || !target.Online {
		// 目标 agent 不在线，返回错误给发送方
		log.Printf("[UAP] target agent %s not online, returning error to %s", msg.To, from.ID)
		from.Send(&Message{
			Type: MsgError,
			ID:   msg.ID,
			From: "gateway",
			To:   from.ID,
			Payload: mustMarshal(ErrorPayload{
				Code:    "agent_offline",
				Message: fmt.Sprintf("target agent %s is not online", msg.To),
			}),
			Ts: time.Now().UnixMilli(),
		})
		return
	}

	// 转发给目标 agent
	if err := target.Send(msg); err != nil {
		log.Printf("[UAP] forward to %s failed: %v", msg.To, err)
	}
}

// sendTo 发送消息给指定 agent
func (s *Server) sendTo(agent *AgentConn, msg *Message) {
	if err := agent.Send(msg); err != nil {
		log.Printf("[UAP] send to %s failed: %v", agent.ID, err)
	}
}

// removeAgent 移除断线 agent
func (s *Server) removeAgent(agent *AgentConn) {
	s.mu.Lock()
	if existing, ok := s.agents[agent.ID]; ok && existing == agent {
		delete(s.agents, agent.ID)
	}
	s.mu.Unlock()

	agent.Close()
	log.Printf("[UAP] agent %s (%s) removed", agent.Name, agent.ID)

	if s.OnAgentOffline != nil {
		s.OnAgentOffline(agent)
	}
}

// GetAgent 获取在线 agent
func (s *Server) GetAgent(agentID string) *AgentConn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[agentID]
	if ok && agent.Online {
		return agent
	}
	return nil
}

// GetAgentsByType 按类型获取在线 agent 列表
func (s *Server) GetAgentsByType(agentType string) []*AgentConn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*AgentConn
	for _, a := range s.agents {
		if a.Online && a.AgentType == agentType {
			result = append(result, a)
		}
	}
	return result
}

// GetAllAgents 获取所有在线 agent 信息
func (s *Server) GetAllAgents() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []map[string]any
	for _, a := range s.agents {
		if a.Online {
			tools := make([]string, 0, len(a.Tools))
			for _, t := range a.Tools {
				tools = append(tools, t.Name)
			}
			result = append(result, map[string]any{
				"agent_id":   a.ID,
				"agent_type": a.AgentType,
				"name":       a.Name,
				"tools":      tools,
				"capacity":   a.Capacity,
				"last_hb":    a.LastHB.Format(time.RFC3339),
			})
		}
	}
	return result
}

// SendToAgent 从外部向指定 agent 发送消息
func (s *Server) SendToAgent(agentID string, msg *Message) error {
	agent := s.GetAgent(agentID)
	if agent == nil {
		return fmt.Errorf("agent %s not online", agentID)
	}
	return agent.Send(msg)
}

// StartHealthCheck 启动心跳检测（定期清理超时 agent）
func (s *Server) StartHealthCheck(timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.mu.RLock()
			var expired []*AgentConn
			for _, a := range s.agents {
				if a.Online && time.Since(a.LastHB) > timeout {
					expired = append(expired, a)
				}
			}
			s.mu.RUnlock()

			for _, a := range expired {
				log.Printf("[UAP] agent %s heartbeat timeout, removing", a.ID)
				s.removeAgent(a)
			}
		}
	}()
}

// ========================= 工具函数 =========================

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func sendDirect(conn *websocket.Conn, msg *Message) {
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}
