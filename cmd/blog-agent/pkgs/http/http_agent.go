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
		// 获取任务摘要列表（轻量级）
		summaries := agent.GetTaskSummaries(account)
		activeIds := agent.GetActiveTaskIDs()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"tasks":     summaries,
			"activeIds": activeIds,
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
		// 获取任务图详情
		graph := agent.GetTaskGraph(taskID)
		if graph == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Task not found",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"task":    graph,
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
		Action string `json:"action"` // pause/resume/cancel/retry
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
	case "retry":
		success = agent.RetryTask(req.TaskID)
		message = "Task retrying"
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

// HandleAgentTaskGraph 获取任务图可视化数据
func HandleAgentTaskGraph(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentTaskGraph", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 获取 taskId/graphId 参数
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Task ID is required",
		})
		return
	}

	// 获取图数据
	result := agent.GetTaskGraphData(taskID)
	json.NewEncoder(w).Encode(result)
}

// HandleAgentTaskInput 处理任务用户输入提交
func HandleAgentTaskInput(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentTaskInput", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	var req struct {
		RequestID string      `json:"request_id"`
		TaskID    string      `json:"task_id"`
		NodeID    string      `json:"node_id"`
		Value     interface{} `json:"value"`
		Cancelled bool        `json:"cancelled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.TaskID == "" || req.NodeID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Task ID and Node ID are required",
		})
		return
	}

	// 创建响应
	resp := agent.NewInputResponse(req.RequestID, req.NodeID, req.TaskID, req.Value, req.Cancelled)

	// 提交输入
	if err := agent.SubmitTaskInput(req.TaskID, req.NodeID, resp); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Input submitted, task resumed",
	})
}

// HandleAgentPendingInputs 获取待处理的输入请求
func HandleAgentPendingInputs(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAgentPendingInputs", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Task ID is required",
		})
		return
	}

	inputs := agent.GetPendingInputs(taskID)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"inputs":  inputs,
	})
}
