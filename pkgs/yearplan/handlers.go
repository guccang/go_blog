package yearplan

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"blog"
	"io/ioutil"
	log "mylog"
	"time"
)

// getAccountFromRequest extracts account from session cookie
func getAccountFromRequest(r *http.Request) string {
	sessionCookie, err := r.Cookie("session")
	if err != nil {
		log.DebugF("No session cookie found: %v", err)
		return ""
	}
	
	return blog.GetAccountFromSession(sessionCookie.Value)
}

// HandleYearPlan renders the year plan page
func HandleYearPlan(w http.ResponseWriter, r *http.Request) {
	// This function should render the yearplan.template, which will be handled in the http package
}

// HandleGetPlan handles the API request to get a year plan
func HandleGetPlan(w http.ResponseWriter, r *http.Request) {
	// Set response header
	w.Header().Set("Content-Type", "application/json")

	// Parse year from query parameters
	yearStr := r.URL.Query().Get("year")
	if yearStr == "" {
		// Try to get from title parameter
		title := r.URL.Query().Get("title")
		if title != "" {
			// Extract year from title (format: 年计划_2023)
			_, err := fmt.Sscanf(title, "年计划_%s", &yearStr)
			if err != nil {
				http.Error(w, "Invalid title format", http.StatusBadRequest)
				return
			}
		} else {
			http.Error(w, "Year parameter is required", http.StatusBadRequest)
			return
		}
	}

	// Convert year to int
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "Invalid year format", http.StatusBadRequest)
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	// Get plan data
	planData, err := blog.GetYearPlanWithAccount(account, year)
	if err != nil {
		// If not found, return empty data structure with 404
		if err.Error() == "未找到年份 "+yearStr+" 的计划" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"yearOverview": "",
				"monthPlans":   make([]string, 12),
				"year":         year,
			})
			return
		}
		
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return plan data as JSON
	json.NewEncoder(w).Encode(planData)
}

// HandleSavePlan handles the API request to save a year plan
func HandleSavePlan(w http.ResponseWriter, r *http.Request) {
	// Set response header
	w.Header().Set("Content-Type", "application/json")

	// Check request method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 记录收到的原始数据，便于调试
	log.DebugF("接收到的年计划数据: %s", string(body))

	// 先解析为map，以便检查任务数据的存在
	var rawData map[string]interface{}
	err = json.Unmarshal(body, &rawData)
	if err != nil {
		http.Error(w, "Failed to parse JSON data: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 检查是否存在tasks字段
	tasks, hasTasks := rawData["tasks"]
	if hasTasks {
		log.DebugF("任务数据存在: %v", tasks)
	} else {
		log.DebugF("任务数据不存在")
	}

	// 解析为YearPlanData
	var planData blog.YearPlanData
	err = json.Unmarshal(body, &planData)
	if err != nil {
		http.Error(w, "Failed to parse JSON data: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 检查任务数据是否正确解析
	log.DebugF("解析后的年计划: Year=%d, TasksMap大小=%d", planData.Year, len(planData.Tasks))

	// 确保Tasks字段不为空
	if planData.Tasks == nil {
		planData.Tasks = make(map[string]interface{})
	}

	// 如果tasks在原始数据中存在，但在planData.Tasks中丢失，手动添加
	if hasTasks && len(planData.Tasks) == 0 {
		planData.Tasks = rawData["tasks"].(map[string]interface{})
		log.DebugF("手动添加任务数据，大小=%d", len(planData.Tasks))
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	// Save plan
	err = blog.SaveYearPlanWithAccount(account, &planData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Year plan saved successfully",
	})
}

// HandleGetMonthGoal handles the API request to get a month goal
func HandleGetMonthGoal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse parameters
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")

	if yearStr == "" || monthStr == "" {
		// Use current month if not specified
		year, month := GetCurrentMonth()
		yearStr = strconv.Itoa(year)
		monthStr = strconv.Itoa(month)
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid year format",
		})
		return
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid month format",
		})
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	// Get month goal
	goal, err := GetMonthGoalWithAccount(account, year, month)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Create response data structure with content field
	responseData := map[string]interface{}{
		"content": goal.Overview,
		"year":    goal.Year,
		"month":   goal.Month,
		"weeks":   goal.Weeks,
		"tasks":   goal.Tasks,
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    responseData,
	})
}

// HandleSaveMonthGoal handles the API request to save a month goal
func HandleSaveMonthGoal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	// Parse request data
	var requestData struct {
		Year    int    `json:"year"`
		Month   int    `json:"month"`
		Content string `json:"content"`
	}

	err = json.Unmarshal(body, &requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON data: " + err.Error(),
		})
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	// Get existing month goal or create new one
	goal, err := GetMonthGoalWithAccount(account, requestData.Year, requestData.Month)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Update overview content
	goal.Overview = requestData.Content

	err = SaveMonthGoalWithAccount(account, goal)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Month goal saved successfully",
	})
}

// HandleGetWeekGoal handles the API request to get a week goal
func HandleGetWeekGoal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse parameters
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	weekStr := r.URL.Query().Get("week")

	if yearStr == "" || monthStr == "" || weekStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Year, month and week parameters are required",
		})
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid year format",
		})
		return
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid month format",
		})
		return
	}

	week, err := strconv.Atoi(weekStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid week format",
		})
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	// Get week goal
	goal, err := GetWeekGoalWithAccount(account, year, month, week)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Create response data structure with content field
	responseData := map[string]interface{}{
		"content": goal.Overview,
		"year":    goal.Year,
		"month":   goal.Month,
		"week":    goal.Week,
		"tasks":   goal.Tasks,
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    responseData,
	})
}

// HandleSaveWeekGoal handles the API request to save a week goal
func HandleSaveWeekGoal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	// Parse request data
	var requestData struct {
		Year    int    `json:"year"`
		Month   int    `json:"month"`
		Week    int    `json:"week"`
		Content string `json:"content"`
	}

	err = json.Unmarshal(body, &requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON data: " + err.Error(),
		})
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	// Get existing week goal or create new one
	goal, err := GetWeekGoalWithAccount(account, requestData.Year, requestData.Month, requestData.Week)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Update overview content
	goal.Overview = requestData.Content

	err = SaveWeekGoalWithAccount(account, goal)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Week goal saved successfully",
	})
}

// HandleAddTask handles the API request to add a task
func HandleAddTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var requestData struct {
		Year int  `json:"year"`
		Month int `json:"month"`
		Task  Task `json:"task"`
	}

	err = json.Unmarshal(body, &requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON data: " + err.Error(),
		})
		return
	}

	// Generate task ID if not provided
	if requestData.Task.ID == "" {
		requestData.Task.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	err = AddTaskWithAccount(account, requestData.Year, requestData.Month, requestData.Task)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task added successfully",
		"task_id": requestData.Task.ID,
	})
}

// HandleUpdateTask handles the API request to update a task
func HandleUpdateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var requestData struct {
		Year   int    `json:"year"`
		Month  int    `json:"month"`
		TaskID string `json:"task_id"`
		Task   Task   `json:"task"`
	}

	err = json.Unmarshal(body, &requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON data: " + err.Error(),
		})
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	err = UpdateTaskWithAccount(account, requestData.Year, requestData.Month, requestData.TaskID, requestData.Task)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task updated successfully",
	})
}

// HandleDeleteTask handles the API request to delete a task
func HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var requestData struct {
		Year   int    `json:"year"`
		Month  int    `json:"month"`
		TaskID string `json:"task_id"`
	}

	err = json.Unmarshal(body, &requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON data: " + err.Error(),
		})
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	err = DeleteTaskWithAccount(account, requestData.Year, requestData.Month, requestData.TaskID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task deleted successfully",
	})
}

// HandleGetMonthGoals handles the API request to get all month goals for a year
func HandleGetMonthGoals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	yearStr := r.URL.Query().Get("year")
	if yearStr == "" {
		year, _ := GetCurrentMonth()
		yearStr = strconv.Itoa(year)
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid year format",
		})
		return
	}

	// Get account from session
	account := getAccountFromRequest(r)
	
	goalsMap, err := GetMonthGoalsWithAccount(account, year)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Convert map to array format for frontend consumption
	var goalsArray []interface{}
	for month := 1; month <= 12; month++ {
		if goal, exists := goalsMap[month]; exists {
			goalsArray = append(goalsArray, map[string]interface{}{
				"year":  goal.Year,
				"month": goal.Month,
				"overview": goal.Overview,
				"weeks": goal.Weeks,
				"tasks": goal.Tasks,
			})
		} else {
			// Add placeholder for months without goals
			goalsArray = append(goalsArray, map[string]interface{}{
				"year":  year,
				"month": month,
				"overview": "",
				"weeks": make(map[string]interface{}),
				"tasks": []interface{}{},
			})
		}
	}

	json.NewEncoder(w).Encode(goalsArray)
}

// InitYearPlan initializes the year plan module
func InitYearPlan() error {
	// This function would perform any necessary initialization
	return nil
} 