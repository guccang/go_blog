package taskbreakdown

import (
	"encoding/json"
	"fmt"
	"html/template"
	log "mylog"
	"net/http"
	"strings"
	"time"

	"auth"
)

var controller *Controller

// SetController 设置控制器实例
func SetController(c *Controller) {
	controller = c
}

// GetTaskManager 获取任务管理器实例
func GetTaskManager() *TaskManager {
	if controller == nil {
		return nil
	}
	return controller.manager
}

// TemplateData 模板数据
type TemplateData struct {
	RootTaskID string
}

// HandleTaskBreakdown 处理任务拆解页面请求
func HandleTaskBreakdown(w http.ResponseWriter, r *http.Request) {
	log.DebugF(log.ModuleTaskBreakdown, "HandleTaskBreakdown %s", r.Method)

	// 检查用户是否已登录
	session, err := r.Cookie("session")
	if err != nil || session.Value == "" {
		// 未登录，重定向到登录页面
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 解析模板
	tmpl, err := template.ParseFiles("templates/taskbreakdown.template")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取root查询参数
	rootTaskID := r.URL.Query().Get("root")

	// 准备模板数据
	data := TemplateData{
		RootTaskID: rootTaskID,
	}

	// 必须在Execute之前设置Content-Type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// 执行模板
	if err := tmpl.Execute(w, data); err != nil {
		log.ErrorF(log.ModuleTaskBreakdown, "Template execution error: %v", err)
		return
	}
}

// HandleTasks 处理任务相关请求
func HandleTasks(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.DebugF(log.ModuleTaskBreakdown, "HandleTasks %s %s", r.Method, r.URL.Path)

	// 检查用户是否已登录
	session, err := r.Cookie("session")
	if err != nil || session.Value == "" {
		// 未登录，返回错误
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleTasks: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	// 根据URL路径和方法路由到不同的处理器
	path := r.URL.Path

	// 处理特定任务的操作
	if path == "/api/tasks" {
		// 根路径：列表、创建
		switch r.Method {
		case http.MethodGet:
			controller.HandleGetTasks(w, r)
		case http.MethodPost:
			controller.HandleCreateTask(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		}
		return
	}

	// 处理已完成任务请求
	if path == "/api/tasks/completed" {
		if r.Method == http.MethodGet {
			controller.HandleGetCompletedRootTasks(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		}
		return
	}

	// 处理已删除任务请求
	if path == "/api/tasks/deleted" {
		if r.Method == http.MethodGet {
			controller.HandleGetDeletedTasks(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		}
		return
	}

	// 处理任务恢复请求
	if strings.HasSuffix(path, "/restore") {
		if r.Method == http.MethodPut {
			controller.HandleRestoreTask(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		}
		return
	}

	// 处理带ID的路径
	if len(path) > len("/api/tasks/") {
		// 检查是否是特定操作
		if r.Method == http.MethodGet {
			// GET请求：获取任务
			controller.HandleGetTask(w, r)
		} else if r.Method == http.MethodPut {
			// PUT请求：更新任务
			controller.HandleUpdateTask(w, r)
		} else if r.Method == http.MethodDelete {
			// DELETE请求：删除任务
			controller.HandleDeleteTask(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		}
		return
	}

	// 默认返回方法不允许
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
}

// HandleTaskProgress 处理任务进度更新请求
func HandleTaskProgress(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPut {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleTaskProgress: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	controller.HandleUpdateTaskProgress(w, r)
}

// HandleTaskOrder 处理任务顺序更新请求
func HandleTaskOrder(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPut {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleTaskOrder: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	controller.HandleUpdateTaskOrder(w, r)
}

// HandleSubtasks 处理子任务相关请求
func HandleSubtasks(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleSubtasks: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		controller.HandleGetSubtasks(w, r)
	case http.MethodPost:
		controller.HandleAddSubtask(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// HandleTimeline 处理时间线数据请求
func HandleTimeline(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleTimeline: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	controller.HandleGetTimeline(w, r)
}

// HandleStatistics 处理统计信息请求
func HandleStatistics(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleStatistics: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	controller.HandleGetStatistics(w, r)
}

// HandleSearchTasks 处理搜索任务请求
func HandleSearchTasks(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleSearchTasks: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	controller.HandleSearchTasks(w, r)
}

// HandleTaskGraph 处理任务网络图数据请求
func HandleTaskGraph(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleTaskGraph: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	controller.HandleGetTaskGraph(w, r)
}

// HandleTimeTrends 处理时间趋势数据请求
func HandleTimeTrends(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")

	// 处理CORS预检请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 检查控制器是否已初始化
	if controller == nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleTimeTrends: controller is nil")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
		return
	}

	controller.HandleGetTimeTrends(w, r)
}

// HandleCompletedTasks 处理已完成任务页面请求
func HandleCompletedTasks(w http.ResponseWriter, r *http.Request) {
	log.DebugF(log.ModuleTaskBreakdown, "HandleCompletedTasks %s", r.Method)

	// 检查用户是否已登录
	session, err := r.Cookie("session")
	if err != nil || session.Value == "" {
		// 未登录，重定向到登录页面
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 解析模板
	tmpl, err := template.ParseFiles("templates/taskbreakdown_completed.template")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 执行模板
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// HandleDeletedTasks 处理已删除任务页面请求
func HandleDeletedTasks(w http.ResponseWriter, r *http.Request) {
	log.DebugF(log.ModuleTaskBreakdown, "HandleDeletedTasks %s", r.Method)

	// 检查用户是否已登录
	session, err := r.Cookie("session")
	if err != nil || session.Value == "" {
		// 未登录，重定向到登录页面
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 解析模板
	tmpl, err := template.ParseFiles("templates/taskbreakdown_deleted.template")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 执行模板
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// HandleTaskTimeAnalysis 处理任务时间分析请求
func HandleTaskTimeAnalysis(w http.ResponseWriter, r *http.Request) {
	log.DebugF(log.ModuleTaskBreakdown, "HandleTaskTimeAnalysis %s", r.Method)

	if controller == nil {
		http.Error(w, "Controller not initialized", http.StatusInternalServerError)
		return
	}

	// 只处理GET请求
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	controller.HandleGetTaskTimeAnalysis(w, r)
}

// HandleDailyTimeOverlap 处理每天时间重叠检查请求
func HandleDailyTimeOverlap(w http.ResponseWriter, r *http.Request) {
	log.DebugF(log.ModuleTaskBreakdown, "HandleDailyTimeOverlap %s", r.Method)

	if controller == nil {
		http.Error(w, "Controller not initialized", http.StatusInternalServerError)
		return
	}

	// 只处理GET请求
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	controller.HandleCheckDailyTimeOverlap(w, r)
}

// HandleSyncToTodo 处理同步到待办事项的请求
func HandleSyncToTodo(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// 认证检查（复用现有逻辑）
	session, err := r.Cookie("session")
	if err != nil || session.Value == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// 获取账户
	account := auth.GetAccountBySession(session.Value)
	if account == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid session"})
		return
	}

	// 获取日期参数（默认为今天）
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 调用同步方法
	manager := NewTaskManager()
	syncedCount, err := manager.SyncInProgressTasksToTodoList(account, date)
	if err != nil {
		log.ErrorF(log.ModuleTaskBreakdown, "HandleSyncToTodo failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error": err.Error(),
		})
		return
	}

	// 返回结果
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"synced_count": syncedCount,
		"date": date,
		"message": fmt.Sprintf("成功同步 %d 个任务到 %s", syncedCount, date),
	})
}
