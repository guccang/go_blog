package http

import (
	"codegen"
	"encoding/json"
	"fmt"
	log "mylog"
	h "net/http"
	"time"
	"view"

	"github.com/gorilla/websocket"
)

// HandleCodeGen 编码助手页面
func HandleCodeGen(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleCodeGen", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	view.PageCodeGen(w)
}

// HandleCodeGenProjects GET: 获取项目列表
func HandleCodeGenProjects(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		jsonError(w, "Unauthorized")
		return
	}

	switch r.Method {
	case h.MethodGet:
		projects, err := codegen.ListProjects()
		if err != nil {
			jsonError(w, err.Error())
			return
		}
		jsonOK(w, map[string]interface{}{
			"projects":  projects,
			"workspace": codegen.GetWorkspace(),
		})

	case h.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request")
			return
		}
		if err := codegen.CreateProject(req.Name); err != nil {
			jsonError(w, err.Error())
			return
		}
		jsonOK(w, map[string]interface{}{"name": req.Name})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleCodeGenRun POST: 启动编码会话
func HandleCodeGenRun(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 || r.Method != h.MethodPost {
		jsonError(w, "Unauthorized")
		return
	}

	var req struct {
		Project string `json:"project"`
		Prompt  string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request")
		return
	}

	if req.Project == "" || req.Prompt == "" {
		jsonError(w, "project and prompt are required")
		return
	}

	session, err := codegen.StartSession(req.Project, req.Prompt)
	if err != nil {
		jsonError(w, err.Error())
		return
	}

	jsonOK(w, map[string]interface{}{
		"session_id": session.ID,
		"project":    session.Project,
		"status":     session.Status,
	})
}

// HandleCodeGenMessage POST: 向已有会话发送消息
func HandleCodeGenMessage(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 || r.Method != h.MethodPost {
		jsonError(w, "Unauthorized")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request")
		return
	}

	if err := codegen.SendMessage(req.SessionID, req.Prompt); err != nil {
		jsonError(w, err.Error())
		return
	}

	jsonOK(w, map[string]interface{}{"status": "ok"})
}

// HandleCodeGenSessions GET: 获取会话列表
func HandleCodeGenSessions(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		jsonError(w, "Unauthorized")
		return
	}

	sessions := codegen.GetSessions()
	jsonOK(w, map[string]interface{}{
		"sessions": sessions,
	})
}

// HandleCodeGenStop POST: 停止会话
func HandleCodeGenStop(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 || r.Method != h.MethodPost {
		jsonError(w, "Unauthorized")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request")
		return
	}

	if err := codegen.StopSession(req.SessionID); err != nil {
		jsonError(w, err.Error())
		return
	}

	jsonOK(w, map[string]interface{}{"status": "stopped"})
}

// HandleCodeGenTree GET: 获取项目目录树
func HandleCodeGenTree(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		jsonError(w, "Unauthorized")
		return
	}

	project := r.URL.Query().Get("project")
	if project == "" {
		jsonError(w, "project is required")
		return
	}

	tree, err := codegen.GetProjectTree(project, 5)
	if err != nil {
		jsonError(w, err.Error())
		return
	}

	jsonOK(w, tree)
}

// HandleCodeGenFile GET: 读取项目文件
func HandleCodeGenFile(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		jsonError(w, "Unauthorized")
		return
	}

	project := r.URL.Query().Get("project")
	filePath := r.URL.Query().Get("path")
	if project == "" || filePath == "" {
		jsonError(w, "project and path are required")
		return
	}

	content, err := codegen.ReadProjectFile(project, filePath)
	if err != nil {
		jsonError(w, err.Error())
		return
	}

	jsonOK(w, map[string]interface{}{
		"path":    filePath,
		"content": content,
	})
}

// HandleCodeGenWS WebSocket: 实时输出
func HandleCodeGenWS(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleCodeGenWS", r)
	if checkLogin(r) != 0 {
		jsonError(w, "Unauthorized")
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	session := codegen.GetSession(sessionID)
	if session == nil {
		h.Error(w, "Session not found", h.StatusNotFound)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *h.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.ErrorF(log.ModuleHandler, "CodeGen WS upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// 先发送已有消息历史
	for _, msg := range session.Messages {
		data, _ := json.Marshal(map[string]interface{}{
			"type":       msg.Role,
			"text":       msg.Content,
			"tool_name":  msg.ToolName,
			"tool_input": msg.ToolInput,
			"time":       msg.Time.Format("15:04:05"),
		})
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// 如果已完成，发送 done 事件后退出
	if session.Status != codegen.StatusRunning {
		data, _ := json.Marshal(codegen.StreamEvent{
			Type:    "result",
			Text:    fmt.Sprintf("会话已结束 (状态: %s)", session.Status),
			CostUSD: session.CostUSD,
			Done:    true,
		})
		conn.WriteMessage(websocket.TextMessage, data)
		return
	}

	// 订阅实时事件
	ch := session.Subscribe()
	defer session.Unsubscribe(ch)

	// 心跳
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// 转发事件到 WebSocket
	for event := range ch {
		data, _ := json.Marshal(event)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			break
		}
		if event.Done {
			break
		}
	}
}

// jsonOK 返回成功 JSON 响应
func jsonOK(w h.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	result := map[string]interface{}{
		"success": true,
		"data":    data,
	}
	json.NewEncoder(w).Encode(result)
}

// jsonError 返回错误 JSON 响应
func jsonError(w h.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	result := map[string]interface{}{
		"success": false,
		"error":   errMsg,
		"time":    time.Now().Format("15:04:05"),
	}
	json.NewEncoder(w).Encode(result)
}
