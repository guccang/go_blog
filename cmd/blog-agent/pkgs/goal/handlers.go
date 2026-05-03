package goal

import (
	"blog"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// getAccount extracts account from session cookie
func getAccount(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return blog.GetAccountFromSession(cookie.Value)
}

// ============================================================================
// HTTP Handlers
// ============================================================================

// HandleGetGoal returns a specific goal as JSON
func HandleGetGoal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	level := r.URL.Query().Get("level")
	period := r.URL.Query().Get("period")

	if level == "" || period == "" {
		// Use current periods if not specified
		daily, weekly, monthly, yearly := CurrentPeriods()
		if level == "" {
			level = LevelDaily
		}
		switch level {
		case LevelDaily:
			period = daily
		case LevelWeekly:
			period = weekly
		case LevelMonthly:
			period = monthly
		case LevelYearly:
			period = yearly
		}
	}

	account := getAccount(r)
	goal, err := GetGoal(account, level, period)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	resp := map[string]interface{}{
		"success": true,
		"data":    goal,
	}

	// Provide period navigation data when requested
	if r.URL.Query().Get("nav") == "true" {
		prev, errPrev := PrevPeriod(level, period)
		next, errNext := NextPeriod(level, period)
		if errPrev == nil && errNext == nil {
			resp["nav"] = map[string]string{
				"prev":    prev,
				"current": period,
				"next":    next,
			}
		}
	}

	json.NewEncoder(w).Encode(resp)
}

// HandleSaveGoal saves goal overview
func HandleSaveGoal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var req struct {
		Level    string  `json:"level"`
		Period   string  `json:"period"`
		Overview *string `json:"overview"`
		Status   *string `json:"status"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON: " + err.Error(),
		})
		return
	}

	if req.Level == "" || req.Period == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "level and period are required",
		})
		return
	}

	account := getAccount(r)
	goal, err := GetGoal(account, req.Level, req.Period)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	if req.Overview != nil {
		goal.Overview = *req.Overview
	}
	if req.Status != nil {
		goal.Status = *req.Status
	}

	if err := SaveGoal(account, goal); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Goal saved successfully",
	})
}

// HandleAddGoalTask adds a task to a goal
func HandleAddGoalTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var req struct {
		Level  string `json:"level"`
		Period string `json:"period"`
		Task   Task   `json:"task"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON: " + err.Error(),
		})
		return
	}

	if req.Task.ID == "" {
		req.Task.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	account := getAccount(r)
	if err := AddTask(account, req.Level, req.Period, req.Task); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Task added successfully",
		"task_id": req.Task.ID,
	})
}

// HandleUpdateGoalTask updates a task in a goal
func HandleUpdateGoalTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var req struct {
		Level  string `json:"level"`
		Period string `json:"period"`
		TaskID string `json:"task_id"`
		Task   Task   `json:"task"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON: " + err.Error(),
		})
		return
	}

	account := getAccount(r)
	if err := UpdateTask(account, req.Level, req.Period, req.TaskID, req.Task); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Task updated successfully",
	})
}

// HandleDeleteGoalTask deletes a task from a goal
func HandleDeleteGoalTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var req struct {
		Level  string `json:"level"`
		Period string `json:"period"`
		TaskID string `json:"task_id"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON: " + err.Error(),
		})
		return
	}

	account := getAccount(r)
	if err := DeleteTask(account, req.Level, req.Period, req.TaskID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Task deleted successfully",
	})
}

// HandleDeleteGoal deletes an entire goal (blog entry)
func HandleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	var req struct {
		Level  string `json:"level"`
		Period string `json:"period"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to parse JSON: " + err.Error(),
		})
		return
	}

	if req.Level == "" || req.Period == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "level and period are required",
		})
		return
	}

	account := getAccount(r)
	if err := DeleteGoal(account, req.Level, req.Period); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Goal deleted successfully",
	})
}

// HandleGetCurrentGoals returns all current active goals (for cortana-friendly access)
func HandleGetCurrentGoals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	account := getAccount(r)
	goals, err := GetCurrentGoals(account)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    goals,
	})
}

// HandleListGoals lists all goals by level
func HandleListGoals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	level := r.URL.Query().Get("level")
	if level == "" {
		level = LevelMonthly
	}

	year := time.Now().Year()
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		if yearStr == "all" {
			year = 0 // disable year filter
		} else if parsed, err := fmt.Sscanf(yearStr, "%d", &year); err == nil && parsed == 1 {
			// parsed successfully
		}
	}

	account := getAccount(r)
	goals, err := ListGoalsByLevel(account, level, year)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    goals,
		"level":   level,
	})
}
