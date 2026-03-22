package uap

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client UAP agent 侧客户端 SDK
type Client struct {
	// 配置
	GatewayURL   string
	AgentID      string
	AgentType    string
	Name         string
	Description  string // agent 能力简述
	HostPlatform string // 运行平台（macOS/Linux/Windows）
	HostIP       string // 主机 IP 地址
	Workspace    string // 工作目录
	Tools        []ToolDef
	Capacity     int
	Meta         map[string]any
	AuthToken    string

	// 内部状态
	conn       *websocket.Conn
	mu         sync.Mutex
	connected  bool
	stopCh     chan struct{}
	backoffIdx int

	// 消息处理回调
	OnMessage func(msg *Message)

	// 注册成功回调（gateway register_ack 成功时触发）
	OnRegistered func(success bool)
}

// NewClient 创建 UAP 客户端
func NewClient(gatewayURL, agentID, agentType, name string) *Client {
	return &Client{
		GatewayURL: gatewayURL,
		AgentID:    agentID,
		AgentType:  agentType,
		Name:       name,
		Capacity:   1,
		stopCh:     make(chan struct{}),
	}
}

// Run 启动连接（阻塞，自动重连）
func (c *Client) Run() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if err := c.connect(); err != nil {
			log.Printf("[UAP-Client] connect failed: %v", err)
			c.backoffSleep()
			continue
		}

		c.register()
		c.runLoop()
	}
}

// connect 建立 WebSocket 连接
func (c *Client) connect() error {
	log.Printf("[UAP-Client] connecting to %s ...", c.GatewayURL)
	conn, _, err := websocket.DefaultDialer.Dial(c.GatewayURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %v", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.backoffIdx = 0
	c.mu.Unlock()

	log.Printf("[UAP-Client] connected to gateway")
	return nil
}

// register 发送注册消息
func (c *Client) register() {
	payload := RegisterPayload{
		AgentID:      c.AgentID,
		AgentType:    c.AgentType,
		Name:         c.Name,
		Description:  c.Description,
		HostPlatform: c.HostPlatform,
		HostIP:       c.HostIP,
		Workspace:    c.Workspace,
		Tools:        c.Tools,
		Capacity:     c.Capacity,
		Meta:         c.Meta,
		AuthToken:    c.AuthToken,
	}
	c.Send(&Message{
		Type:    MsgRegister,
		ID:      NewMsgID(),
		From:    c.AgentID,
		Payload: mustMarshal(payload),
		Ts:      time.Now().UnixMilli(),
	})
}

// runLoop 消息读取主循环
func (c *Client) runLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	// 启动心跳
	go c.heartbeatLoop()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[UAP-Client] read error: %v, reconnecting...", err)
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[UAP-Client] parse error: %v", err)
			continue
		}

		switch msg.Type {
		case MsgRegisterAck:
			if msg.From != "" {
				// 来自其他 agent（如 go_blog 的 codegen 协议 ack），转给 OnMessage
				if c.OnMessage != nil {
					c.OnMessage(&msg)
				}
			} else {
				// gateway 自身的注册确认
				var ack RegisterAckPayload
				json.Unmarshal(msg.Payload, &ack)
				if ack.Success {
					log.Printf("[UAP-Client] registered as %s (%s)", c.Name, c.AgentID)
				} else {
					log.Printf("[UAP-Client] register rejected: %s", ack.Error)
				}
				if c.OnRegistered != nil {
					c.OnRegistered(ack.Success)
				}
			}

		case MsgHeartbeatAck:
			if msg.From != "" {
				// 来自其他 agent 的心跳回复，转给 OnMessage
				if c.OnMessage != nil {
					c.OnMessage(&msg)
				}
			}
			// else: gateway 自身的心跳确认，忽略

		default:
			if c.OnMessage != nil {
				c.OnMessage(&msg)
			}
		}
	}
}

// heartbeatLoop 定时发送心跳
func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			connected := c.connected
			c.mu.Unlock()
			if !connected {
				log.Printf("[UAP-Client] heartbeat loop exiting: not connected")
				return
			}
			if err := c.Send(&Message{
				Type: MsgHeartbeat,
				From: c.AgentID,
				Payload: mustMarshal(HeartbeatPayload{
					AgentID: c.AgentID,
				}),
				Ts: time.Now().UnixMilli(),
			}); err != nil {
				log.Printf("[UAP-Client] heartbeat send failed: %v", err)
			}
		case <-c.stopCh:
			return
		}
	}
}

// Send 发送消息到 gateway
func (c *Client) Send(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// SendTo 向指定 agent 发送消息（通过 gateway 路由）
func (c *Client) SendTo(toAgentID, msgType string, payload any) error {
	return c.Send(&Message{
		Type:    msgType,
		ID:      NewMsgID(),
		From:    c.AgentID,
		To:      toAgentID,
		Payload: mustMarshal(payload),
		Ts:      time.Now().UnixMilli(),
	})
}

// SendNotify 发送通知消息
func (c *Client) SendNotify(toAgentID string, payload NotifyPayload) error {
	return c.SendTo(toAgentID, MsgNotify, payload)
}

// IsConnected 是否已连接
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Stop 停止客户端
func (c *Client) Stop() {
	close(c.stopCh)
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Unlock()
}

// backoffSleep 指数退避
func (c *Client) backoffSleep() {
	delays := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
		60 * time.Second,
	}

	delay := delays[c.backoffIdx]
	if c.backoffIdx < len(delays)-1 {
		c.backoffIdx++
	}

	select {
	case <-c.stopCh:
		return
	case <-time.After(delay):
	}
}

// ========================= 工具函数 =========================

// NewMsgID 生成消息 ID
func NewMsgID() string {
	return uuid.New().String()[:8]
}
