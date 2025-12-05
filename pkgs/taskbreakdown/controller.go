package taskbreakdown

import (
	"encoding/json"
	"net/http"
	"strings"

	"auth"
)

// Controller 任务控制器
type Controller struct {
	manager *TaskManager
}

// NewController 创建新的任务控制器
func NewController(manager *TaskManager) *Controller {
	return &Controller{
		manager: manager,
	}
}

// getsession 从请求cookie中提取session值
func getsession(r *http.Request) string {
	session, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return session.Value
}

// getAccountFromRequest 通过解析session cookie提取账户
func getAccountFromRequest(r *http.Request) string {
	s := getsession(r)
	if s == "" {
		return ""
	}
	return auth.GetAccountBySession(s)
}

// setCommonHeaders 设置通用响应头
func setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// sendErrorResponse 发送错误响应
func sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(TaskResponse{
		Success: false,
		Error:   message,
	})
}

// sendSuccessResponse 发送成功响应
func sendSuccessResponse(w http.ResponseWriter, data interface{}) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TaskResponse{
		Success: true,
		Data:    data,
	})
}

// HandleGetTasks 处理获取任务列表请求
func (c *Controller) HandleGetTasks(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 检查是否请求特定任务
	taskID := r.URL.Query().Get("id")
	if taskID != "" {
		c.HandleGetTask(w, r)
		return
	}

	// 检查是否请求根任务
	parentID := r.URL.Query().Get("parent_id")
	if parentID != "" {
		c.HandleGetSubtasks(w, r)
		return
	}

	// 获取所有任务
	tasks, err := c.manager.ListTasks(account)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to get tasks: "+err.Error())
		return
	}

	sendSuccessResponse(w, tasks)
}

// HandleGetTask 处理获取单个任务请求
func (c *Controller) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		// 尝试从URL路径提取
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/tasks/") {
			taskID = strings.TrimPrefix(path, "/api/tasks/")
		}
	}

	if taskID == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	task, err := c.manager.GetTask(account, taskID)
	if err != nil {
		sendErrorResponse(w, http.StatusNotFound, "Task not found: "+err.Error())
		return
	}

	sendSuccessResponse(w, task)
}

// HandleCreateTask 处理创建任务请求
func (c *Controller) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	// 检查内容类型
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
		sendErrorResponse(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req TaskCreateRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	task, err := c.manager.CreateTask(account, &req)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to create task: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(TaskResponse{
		Success: true,
		Data:    task,
	})
}

// HandleUpdateTask 处理更新任务请求
func (c *Controller) HandleUpdateTask(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	// 检查内容类型
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
		sendErrorResponse(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 从URL路径提取任务ID
	path := r.URL.Path
	var taskID string
	if strings.HasPrefix(path, "/api/tasks/") {
		// 移除/api/tasks/前缀
		taskID = strings.TrimPrefix(path, "/api/tasks/")
		// 移除可能的后续路径（如/progress）
		if idx := strings.Index(taskID, "/"); idx != -1 {
			taskID = taskID[:idx]
		}
	}

	if taskID == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	var updates TaskUpdateRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updates); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	task, err := c.manager.UpdateTask(account, taskID, &updates)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to update task: "+err.Error())
		return
	}

	sendSuccessResponse(w, task)
}

// HandleDeleteTask 处理删除任务请求
func (c *Controller) HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 从URL路径提取任务ID
	path := r.URL.Path
	var taskID string
	if strings.HasPrefix(path, "/api/tasks/") {
		taskID = strings.TrimPrefix(path, "/api/tasks/")
	}

	if taskID == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	if err := c.manager.DeleteTask(account, taskID); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to delete task: "+err.Error())
		return
	}

	sendSuccessResponse(w, map[string]string{
		"message": "Task deleted successfully",
	})
}

// HandleGetSubtasks 处理获取子任务请求
func (c *Controller) HandleGetSubtasks(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	parentID := r.URL.Query().Get("parent_id")
	if parentID == "" {
		// 如果没有指定parent_id，获取根任务
		rootTasks, err := c.manager.GetRootTasks(account)
		if err != nil {
			sendErrorResponse(w, http.StatusInternalServerError, "Failed to get root tasks: "+err.Error())
			return
		}
		sendSuccessResponse(w, rootTasks)
		return
	}

	// 获取任务树
	taskTree, err := c.manager.GetTaskTree(account, parentID)
	if err != nil {
		sendErrorResponse(w, http.StatusNotFound, "Task not found: "+err.Error())
		return
	}

	sendSuccessResponse(w, taskTree)
}

// HandleAddSubtask 处理添加子任务请求
func (c *Controller) HandleAddSubtask(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	// 检查内容类型
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
		sendErrorResponse(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 从URL路径提取父任务ID
	path := r.URL.Path
	var parentID string
	if strings.HasPrefix(path, "/api/tasks/") {
		// 移除/api/tasks/前缀
		parentID = strings.TrimPrefix(path, "/api/tasks/")
		// 移除/subtasks后缀
		parentID = strings.TrimSuffix(parentID, "/subtasks")
	}

	if parentID == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Parent task ID is required")
		return
	}

	var req TaskCreateRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	subtask, err := c.manager.AddSubtask(account, parentID, &req)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to add subtask: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(TaskResponse{
		Success: true,
		Data:    subtask,
	})
}

// HandleUpdateTaskProgress 处理更新任务进度请求
func (c *Controller) HandleUpdateTaskProgress(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	// 检查内容类型
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
		sendErrorResponse(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 从URL路径提取任务ID
	path := r.URL.Path
	var taskID string
	if strings.HasPrefix(path, "/api/tasks/") && strings.HasSuffix(path, "/progress") {
		// 移除/api/tasks/前缀和/progress后缀
		taskID = strings.TrimPrefix(path, "/api/tasks/")
		taskID = strings.TrimSuffix(taskID, "/progress")
	}

	if taskID == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	var request struct {
		Progress int `json:"progress"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	// 验证进度值
	if request.Progress < 0 || request.Progress > 100 {
		sendErrorResponse(w, http.StatusBadRequest, "Progress must be between 0 and 100")
		return
	}

	updates := &TaskUpdateRequest{
		Progress: &request.Progress,
	}

	task, err := c.manager.UpdateTask(account, taskID, updates)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to update task progress: "+err.Error())
		return
	}

	sendSuccessResponse(w, task)
}

// HandleUpdateTaskOrder 处理更新任务顺序请求
func (c *Controller) HandleUpdateTaskOrder(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	// 检查内容类型
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
		sendErrorResponse(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var request struct {
		TaskID string `json:"task_id"`
		Order  int    `json:"order"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	if request.TaskID == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	if err := c.manager.UpdateTaskOrder(account, request.TaskID, request.Order); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to update task order: "+err.Error())
		return
	}

	sendSuccessResponse(w, map[string]string{
		"message": "Task order updated successfully",
	})
}

// HandleGetTimeline 处理获取时间线数据请求
func (c *Controller) HandleGetTimeline(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 获取根任务ID参数（可选）
	rootID := r.URL.Query().Get("root")

	timelineData, err := c.manager.GetTimelineData(account, rootID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to get timeline data: "+err.Error())
		return
	}

	sendSuccessResponse(w, timelineData)
}

// HandleGetStatistics 处理获取统计信息请求
func (c *Controller) HandleGetStatistics(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 获取根任务ID参数（可选）
	rootID := r.URL.Query().Get("root")

	stats, err := c.manager.GetStatistics(account, rootID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to get statistics: "+err.Error())
		return
	}

	sendSuccessResponse(w, stats)
}

// HandleSearchTasks 处理搜索任务请求
func (c *Controller) HandleSearchTasks(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Search query is required")
		return
	}

	results, err := c.manager.SearchTasks(account, query)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to search tasks: "+err.Error())
		return
	}

	sendSuccessResponse(w, results)
}

// HandleGetTaskGraph 处理获取任务网络图数据请求
func (c *Controller) HandleGetTaskGraph(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 获取根任务ID参数（可选）
	rootID := r.URL.Query().Get("root")

	graphData, err := c.manager.GetTaskGraph(account, rootID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to get task graph data: "+err.Error())
		return
	}

	sendSuccessResponse(w, graphData)
}

// HandleGetTimeTrends 处理获取时间趋势数据请求
func (c *Controller) HandleGetTimeTrends(w http.ResponseWriter, r *http.Request) {
	setCommonHeaders(w)

	account := getAccountFromRequest(r)
	if account == "" {
		sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// 获取根任务ID参数（可选）
	rootID := r.URL.Query().Get("root")
	// 获取时间范围参数（可选，默认"30d"）
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "30d"
	}

	// 验证时间范围
	validRanges := map[string]bool{"7d": true, "30d": true, "90d": true, "1y": true}
	if !validRanges[timeRange] {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid time range. Valid values: 7d, 30d, 90d, 1y")
		return
	}

	trendData, err := c.manager.GetTimeTrends(account, rootID, timeRange)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to get time trends data: "+err.Error())
		return
	}

	sendSuccessResponse(w, trendData)
}