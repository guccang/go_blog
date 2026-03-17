package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"agentbase"
	"uap"
)

// ACPMessage 定义 ACP 协议消息结构
type ACPMessage struct {
	Type      string      `json:"type"`
	SessionID string      `json:"session_id,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// ACPClient 表示一个 ACP 客户端连接
type ACPClient struct {
	Conn     net.Conn
	ID       string
	LastSeen time.Time
}

// ACPAgent 主结构体
type ACPAgent struct {
	Config     *Config
	Clients    map[string]*ACPClient
	Listener   net.Listener
	StopChan   chan bool
	AgentBase  *agentbase.AgentBase
}

func main() {
	configPath := flag.String("config", "acp-agent.json", "配置文件路径")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	agentBase, err := agentbase.NewAgentBase(config.AgentBaseConfig)
	if err != nil {
		log.Fatalf("创建 AgentBase 失败: %v", err)
	}

	agent := &ACPAgent{
		Config:    config,
		Clients:   make(map[string]*ACPClient),
		StopChan:  make(chan bool),
		AgentBase: agentBase,
	}

	if err := agent.Start(); err != nil {
		log.Fatalf("启动 ACP Agent 失败: %v", err)
	}

	waitForStop(agent)
}

func (a *ACPAgent) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.Config.Port))
	if err != nil {
		return fmt.Errorf("启动监听失败: %v", err)
	}
	a.Listener = listener

	log.Printf("ACP Agent 启动，监听端口: %d", a.Config.Port)
	go a.handleConnections()
	go a.heartbeatCheck()

	return nil
}

func (a *ACPAgent) handleConnections() {
	for {
		conn, err := a.Listener.Accept()
		if err != nil {
			select {
			case <-a.StopChan:
				return
			default:
				log.Printf("接受连接失败: %v", err)
				continue
			}
		}
		go a.handleClient(conn)
	}
}

func (a *ACPAgent) handleClient(conn net.Conn) {
	clientID := fmt.Sprintf("%s-%d", conn.RemoteAddr().String(), time.Now().UnixNano())
	client := &ACPClient{
		Conn:     conn,
		ID:       clientID,
		LastSeen: time.Now(),
	}

	a.Clients[clientID] = client
	defer func() {
		delete(a.Clients, clientID)
		conn.Close()
		log.Printf("客户端断开: %s", clientID)
	}()

	log.Printf("新客户端连接: %s", clientID)

	welcomeMsg := ACPMessage{
		Type:      "welcome",
		SessionID: clientID,
		Content: map[string]interface{}{
			"message": "欢迎连接到 ACP Agent",
			"version": "1.0.0",
			"time":    time.Now().Unix(),
		},
		Timestamp: time.Now().Unix(),
	}
	a.sendMessage(conn, welcomeMsg)

	decoder := json.NewDecoder(conn)
	for {
		var msg ACPMessage
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("解码消息失败 (客户端 %s): %v", clientID, err)
			return
		}

		client.LastSeen = time.Now()
		a.handleMessage(client, msg)
	}
}

func (a *ACPAgent) handleMessage(client *ACPClient, msg ACPMessage) {
	log.Printf("收到消息 (客户端 %s, 类型: %s)", client.ID, msg.Type)

	switch msg.Type {
	case "ping":
		response := ACPMessage{
			Type:      "pong",
			SessionID: client.ID,
			RequestID: msg.RequestID,
			Timestamp: time.Now().Unix(),
		}
		a.sendMessage(client.Conn, response)

	case "code_execution":
		a.handleCodeExecution(client, msg)

	case "tool_call":
		a.handleToolCall(client, msg)

	case "session_management":
		a.handleSessionManagement(client, msg)

	case "cloud_code_request":
		a.HandleCloudCodeRequest(client, msg)

	case "client_code_x_request":
		a.HandleClientCodeXRequest(client, msg)

	default:
		response := ACPMessage{
			Type:      "error",
			SessionID: client.ID,
			RequestID: msg.RequestID,
			Error:     fmt.Sprintf("未知消息类型: %s", msg.Type),
			Timestamp: time.Now().Unix(),
		}
		a.sendMessage(client.Conn, response)
	}
}

func (a *ACPAgent) handleCodeExecution(client *ACPClient, msg ACPMessage) {
	response := ACPMessage{
		Type:      "code_execution_result",
		SessionID: client.ID,
		RequestID: msg.RequestID,
		Content: map[string]interface{}{
			"status":  "received",
			"message": "代码执行请求已接收，处理中...",
		},
		Timestamp: time.Now().Unix(),
	}
	a.sendMessage(client.Conn, response)
}

func (a *ACPAgent) handleToolCall(client *ACPClient, msg ACPMessage) {
	response := ACPMessage{
		Type:      "tool_call_result",
		SessionID: client.ID,
		RequestID: msg.RequestID,
		Content: map[string]interface{}{
			"status":  "received",
			"message": "工具调用请求已接收",
		},
		Timestamp: time.Now().Unix(),
	}
	a.sendMessage(client.Conn, response)
}

func (a *ACPAgent) handleSessionManagement(client *ACPClient, msg ACPMessage) {
	response := ACPMessage{
		Type:      "session_management_result",
		SessionID: client.ID,
		RequestID: msg.RequestID,
		Content: map[string]interface{}{
			"status":  "success",
			"message": "会话管理操作完成",
		},
		Timestamp: time.Now().Unix(),
	}
	a.sendMessage(client.Conn, response)
}

func (a *ACPAgent) sendMessage(conn net.Conn, msg ACPMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("编码消息失败: %v", err)
		return
	}

	data = append(data, '
')
	if _, err := conn.Write(data); err != nil {
		log.Printf("发送消息失败: %v", err)
	}
}

func (a *ACPAgent) heartbeatCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.checkClientHeartbeats()
		case <-a.StopChan:
			return
		}
	}
}

func (a *ACPAgent) checkClientHeartbeats() {
	now := time.Now()
	for clientID, client := range a.Clients {
		if now.Sub(client.LastSeen) > 120*time.Second {
			log.Printf("客户端 %s 心跳超时，断开连接", clientID)
			client.Conn.Close()
			delete(a.Clients, clientID)
		}
	}
}

func (a *ACPAgent) Stop() {
	close(a.StopChan)
	if a.Listener != nil {
		a.Listener.Close()
	}

	for _, client := range a.Clients {
		client.Conn.Close()
	}

	log.Println("ACP Agent 已停止")
}

func waitForStop(agent *ACPAgent) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("收到停止信号，正在停止...")
	agent.Stop()
}
