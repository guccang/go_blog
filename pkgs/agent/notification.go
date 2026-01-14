package agent

import (
	log "mylog"
	"sync"

	"github.com/gorilla/websocket"
)

// TaskNotification 任务通知
type TaskNotification struct {
	TaskID   string      `json:"task_id"`
	Type     string      `json:"type"` // submitted/started/progress/paused/resumed/canceled/done/error
	Progress float64     `json:"progress,omitempty"`
	Message  string      `json:"message,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

// ClientConnection WebSocket 客户端连接
type ClientConnection struct {
	Account string
	Conn    *websocket.Conn
}

// NotificationHub WebSocket 推送中心
type NotificationHub struct {
	clients    map[string][]*websocket.Conn // account -> connections
	broadcast  chan TaskNotification
	register   chan *ClientConnection
	unregister chan *ClientConnection
	mu         sync.RWMutex
}

// NewNotificationHub 创建通知中心
func NewNotificationHub() *NotificationHub {
	return &NotificationHub{
		clients:    make(map[string][]*websocket.Conn),
		broadcast:  make(chan TaskNotification, 100),
		register:   make(chan *ClientConnection, 10),
		unregister: make(chan *ClientConnection, 10),
	}
}

// Start 启动通知中心
func (h *NotificationHub) Start() {
	log.Message(log.ModuleAgent, "Starting notification hub")
	go h.run()
}

// run 主循环
func (h *NotificationHub) run() {
	for {
		select {
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case notification := <-h.broadcast:
			h.broadcastToAll(notification)
		}
	}
}

// Register 注册客户端
func (h *NotificationHub) Register(client *ClientConnection) {
	h.register <- client
}

// Unregister 注销客户端
func (h *NotificationHub) Unregister(client *ClientConnection) {
	h.unregister <- client
}

// Broadcast 广播通知
func (h *NotificationHub) Broadcast(notification TaskNotification) {
	select {
	case h.broadcast <- notification:
	default:
		log.Warn(log.ModuleAgent, "Notification channel full, dropping message")
	}
}

// BroadcastToAccount 向特定账户广播
func (h *NotificationHub) BroadcastToAccount(account string, notification TaskNotification) {
	h.mu.RLock()
	conns, ok := h.clients[account]
	h.mu.RUnlock()

	if !ok {
		return
	}

	for _, conn := range conns {
		if err := conn.WriteJSON(notification); err != nil {
			log.WarnF(log.ModuleAgent, "Failed to send notification: %v", err)
		}
	}
}

// addClient 添加客户端
func (h *NotificationHub) addClient(client *ClientConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client.Account] = append(h.clients[client.Account], client.Conn)
	log.MessageF(log.ModuleAgent, "Client registered for account: %s", client.Account)
}

// removeClient 移除客户端
func (h *NotificationHub) removeClient(client *ClientConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.clients[client.Account]
	if !ok {
		return
	}

	// 查找并移除连接
	for i, conn := range conns {
		if conn == client.Conn {
			h.clients[client.Account] = append(conns[:i], conns[i+1:]...)
			conn.Close()
			break
		}
	}

	// 如果没有连接了，删除账户条目
	if len(h.clients[client.Account]) == 0 {
		delete(h.clients, client.Account)
	}

	log.MessageF(log.ModuleAgent, "Client unregistered for account: %s", client.Account)
}

// broadcastToAll 向所有客户端广播
func (h *NotificationHub) broadcastToAll(notification TaskNotification) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.clients {
		for _, conn := range conns {
			if err := conn.WriteJSON(notification); err != nil {
				log.WarnF(log.ModuleAgent, "Failed to broadcast: %v", err)
			}
		}
	}
}

// GetConnectedAccounts 获取已连接的账户数
func (h *NotificationHub) GetConnectedAccounts() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetTotalConnections 获取总连接数
func (h *NotificationHub) GetTotalConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, conns := range h.clients {
		total += len(conns)
	}
	return total
}

// SyncReminders 同步提醒
func (h *NotificationHub) SyncReminders(account string) {
	if globalScheduler == nil {
		return
	}
	reminders := globalScheduler.GetReminders(account)
	if len(reminders) == 0 {
		return
	}

	var activeReminders []*Reminder
	for _, r := range reminders {
		if r.Enabled {
			activeReminders = append(activeReminders, r)
		}
	}

	if len(activeReminders) == 0 {
		return
	}

	notification := TaskNotification{
		Type: "reminder_sync",
		Data: activeReminders,
	}
	h.BroadcastToAccount(account, notification)
}
