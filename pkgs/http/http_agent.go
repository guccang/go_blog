package http

import (
	"agent"
	"encoding/json"
	"fmt"
	log "mylog"
	h "net/http"
	"view"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *h.Request) bool {
		return true
	},
}

// HandleAgentPage renders the agent task panel page
func HandleAgentPage(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentPage", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageAgent(w)
}

// HandleAgentTasks handles agent task API
func HandleAgentTasks(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentTasks", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	account := getAccountFromRequest(r)

	switch r.Method {
	case h.MethodGet:
		// 获取所有任务
		tasks := agent.GetTasks(account)
		reminders := make(map[string]interface{})
		for _, t := range tasks {
			if t.LinkedReminderID != "" {
				if r := agent.GetReminderInfo(t.LinkedReminderID); r != nil {
					reminders[t.ID] = r
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"tasks":     tasks,
			"reminders": reminders,
		})

	case h.MethodPost:
		// 创建新任务
		var req struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid request body",
			})
			return
		}

		if req.Description == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Description is required",
			})
			return
		}

		if req.Priority <= 0 {
			req.Priority = 5
		}

		task := agent.CreateTask(account, req.Title, req.Description, req.Priority)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"task":    task,
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleAgentTask handles single task operations
func HandleAgentTask(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentTask", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 从 URL 获取 task ID
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Task ID is required",
		})
		return
	}

	switch r.Method {
	case h.MethodGet:
		// 获取任务详情
		task := agent.GetTask(taskID)
		if task == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Task not found",
			})
			return
		}
		// 如果有关联的提醒，获取提醒信息
		var reminderInfo interface{}
		if task.LinkedReminderID != "" {
			reminderInfo = agent.GetReminderInfo(task.LinkedReminderID)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"task":     task,
			"reminder": reminderInfo,
		})

	case h.MethodDelete:
		// 删除任务
		if agent.DeleteTask(taskID) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Task deleted",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to delete task",
			})
		}

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleAgentReminder handles reminder info API
func HandleAgentReminder(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentReminder", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	reminderID := r.URL.Query().Get("id")
	if reminderID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Reminder ID is required",
		})
		return
	}

	reminder := agent.GetReminderInfo(reminderID)
	if reminder == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Reminder not found",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"reminder": reminder,
	})
}

// HandleAgentTaskAction handles task actions (pause/resume)
func HandleAgentTaskAction(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentTaskAction", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
		Action string `json:"action"` // pause/resume/cancel
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	var success bool
	var message string

	switch req.Action {
	case "pause":
		success = agent.PauseTask(req.TaskID)
		message = "Task paused"
	case "resume":
		success = agent.ResumeTask(req.TaskID)
		message = "Task resumed"
	case "cancel":
		success = agent.CancelTask(req.TaskID)
		message = "Task canceled"
	default:
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid action",
		})
		return
	}

	if success {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": message,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to %s task", req.Action),
		})
	}
}

// HandleAgentWebSocket handles WebSocket connection for real-time notifications
func HandleAgentWebSocket(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentWebSocket", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	account := getAccountFromRequest(r)

	// 升级为 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.ErrorF(log.ModuleHandler, "WebSocket upgrade failed: %v", err)
		return
	}

	log.InfoF(log.ModuleHandler, "WebSocket connected for account: %s", account)

	// 注册到通知中心
	hub := agent.GetHub()
	if hub == nil {
		log.Error(log.ModuleHandler, "Notification hub not initialized")
		conn.Close()
		return
	}

	client := &agent.ClientConnection{
		Account: account,
		Conn:    conn,
	}
	hub.Register(client)

	// Sync reminders after connection is established
	hub.SyncReminders(account)

	// 保持连接并处理心跳
	defer func() {
		hub.Unregister(client)
		log.InfoF(log.ModuleHandler, "WebSocket disconnected for account: %s", account)
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.WarnF(log.ModuleHandler, "WebSocket error: %v", err)
			}
			break
		}
	}
}

// HandleAgentStatus handles agent status API
func HandleAgentStatus(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentStatus", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	hub := agent.GetHub()
	pool := agent.GetPool()

	status := map[string]interface{}{
		"hub_connected":     hub != nil,
		"pool_running":      pool != nil,
		"connected_clients": 0,
	}

	if hub != nil {
		status["connected_clients"] = hub.GetTotalConnections()
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  status,
	})
}
