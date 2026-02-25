package statistics

import (
	"encoding/json"
	"exercise"
	"fmt"
	"reading"
	"taskbreakdown"
	"time"
	"todolist"
	"yearplan"
)

// =================================== TodoList Raw 接口 =========================================

// RawGetTodosByDate 获取指定日期的待办列表
func RawGetTodosByDate(account, date string) string {
	mgr := todolist.NewTodoManager()
	list, err := mgr.GetTodosByDate(account, date)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(list)
	return string(data)
}

// RawGetTodosRange 获取日期范围内的待办
func RawGetTodosRange(account, startDate, endDate string) string {
	mgr := todolist.NewTodoManager()
	all, err := mgr.GetHistoricalTodos(account, startDate, endDate)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(all)
	return string(data)
}

// RawAddTodo 添加待办事项
func RawAddTodo(account, date, content string, hours, minutes, urgency, importance int) string {
	mgr := todolist.NewTodoManager()
	item, err := mgr.AddTodo(account, date, content, hours, minutes, urgency, importance)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(item)
	return string(data)
}

// RawToggleTodo 切换待办完成状态
func RawToggleTodo(account, date, id string) string {
	mgr := todolist.NewTodoManager()
	err := mgr.ToggleTodo(account, date, id)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return `{"success": true}`
}

// RawDeleteTodo 删除待办
func RawDeleteTodo(account, date, id string) string {
	mgr := todolist.NewTodoManager()
	err := mgr.DeleteTodo(account, date, id)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return `{"success": true}`
}

// =================================== Exercise Raw 接口 =========================================

// RawGetExerciseByDate 获取指定日期运动记录
func RawGetExerciseByDate(account, date string) string {
	list, err := exercise.GetExercisesByDate(account, date)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(list)
	return string(data)
}

// RawGetExerciseRange 获取日期范围运动记录
func RawGetExerciseRange(account, startDate, endDate string) string {
	all, err := exercise.GetAllExercises(account)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Sprintf(`{"error": "invalid start date: %s"}`, err.Error())
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return fmt.Sprintf(`{"error": "invalid end date: %s"}`, err.Error())
	}

	filtered := make(map[string]exercise.ExerciseList)
	for dateKey, list := range all {
		d, err := time.Parse("2006-01-02", dateKey)
		if err != nil {
			continue
		}
		if (d.Equal(start) || d.After(start)) && (d.Equal(end) || d.Before(end)) {
			filtered[dateKey] = list
		}
	}
	data, _ := json.Marshal(filtered)
	return string(data)
}

// RawAddExercise 添加运动记录
func RawAddExercise(account, date, name, exerciseType string, duration int, intensity string, calories int, notes string) string {
	item, err := exercise.AddExercise(account, date, name, exerciseType, duration, intensity, calories, notes, 0, nil)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(item)
	return string(data)
}

// RawGetExerciseStats 获取运动统计（周期性）
func RawGetExerciseStats(account string, days int) string {
	all, err := exercise.GetAllExercises(account)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	now := time.Now()
	cutoff := now.AddDate(0, 0, -days)

	totalMinutes := 0
	totalCalories := 0
	totalSessions := 0
	typeCount := make(map[string]int)
	activeDays := 0

	for dateKey, list := range all {
		d, err := time.Parse("2006-01-02", dateKey)
		if err != nil {
			continue
		}
		if d.After(cutoff) || d.Equal(cutoff) {
			if len(list.Items) > 0 {
				activeDays++
			}
			for _, item := range list.Items {
				totalMinutes += item.Duration
				totalCalories += item.Calories
				totalSessions++
				typeCount[item.Type]++
			}
		}
	}

	result := map[string]interface{}{
		"period":         fmt.Sprintf("最近%d天", days),
		"total_sessions": totalSessions,
		"total_minutes":  totalMinutes,
		"total_hours":    fmt.Sprintf("%.1f", float64(totalMinutes)/60.0),
		"total_calories": totalCalories,
		"active_days":    activeDays,
		"type_breakdown": typeCount,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// =================================== Reading Raw 接口 =========================================

// RawGetAllBooks 获取所有书籍
func RawGetAllBooks(account string) string {
	books := reading.GetAllBooksWithAccount(account)
	if books == nil || len(books) == 0 {
		return `{"books": [], "total": 0}`
	}

	type BookSummary struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Author     string `json:"author"`
		Status     string `json:"status"`
		TotalPages int    `json:"total_pages"`
		Category   string `json:"category"`
	}
	summaries := make([]BookSummary, 0, len(books))
	for _, b := range books {
		cat := ""
		if len(b.Category) > 0 {
			cat = b.Category[0]
		}
		summaries = append(summaries, BookSummary{
			ID:         b.ID,
			Title:      b.Title,
			Author:     b.Author,
			Status:     b.Status,
			TotalPages: b.TotalPages,
			Category:   cat,
		})
	}
	result := map[string]interface{}{
		"books": summaries,
		"total": len(summaries),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// RawGetBooksByStatus 按状态筛选书籍
func RawGetBooksByStatus(account, status string) string {
	books := reading.FilterBooksByStatusWithAccount(account, status)
	if books == nil || len(books) == 0 {
		return fmt.Sprintf(`{"books": [], "status": "%s", "total": 0}`, status)
	}

	type BookSummary struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Author     string `json:"author"`
		TotalPages int    `json:"total_pages"`
	}
	summaries := make([]BookSummary, 0, len(books))
	for _, b := range books {
		summaries = append(summaries, BookSummary{
			ID:         b.ID,
			Title:      b.Title,
			Author:     b.Author,
			TotalPages: b.TotalPages,
		})
	}
	result := map[string]interface{}{
		"books":  summaries,
		"status": status,
		"total":  len(summaries),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// RawGetReadingStats 获取阅读统计
func RawGetReadingStats(account string) string {
	stats := reading.GetReadingStatisticsWithAccount(account)
	if stats == nil {
		return `{"error": "获取阅读统计失败"}`
	}
	data, _ := json.Marshal(stats)
	return string(data)
}

// RawUpdateReadingProgress 更新阅读进度
func RawUpdateReadingProgress(account, bookID string, currentPage int, notes string) string {
	err := reading.UpdateReadingProgressWithAccount(account, bookID, currentPage, notes)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return `{"success": true}`
}

// RawGetBookNotes 获取读书笔记
func RawGetBookNotes(account, bookID string) string {
	notes := reading.GetBookNotesWithAccount(account, bookID)
	if notes == nil || len(notes) == 0 {
		return `{"notes": [], "total": 0}`
	}
	result := map[string]interface{}{
		"notes": notes,
		"total": len(notes),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// =================================== YearPlan Raw 接口 =========================================

// RawGetMonthGoal 获取月度目标
func RawGetMonthGoal(account string, year, month int) string {
	goal, err := yearplan.GetMonthGoalWithAccount(account, year, month)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(goal)
	return string(data)
}

// RawGetYearGoals 获取年度所有月目标
func RawGetYearGoals(account string, year int) string {
	goals, err := yearplan.GetMonthGoalsWithAccount(account, year)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(goals)
	return string(data)
}

// RawAddYearTask 添加计划任务
func RawAddYearTask(account string, year, month int, title, description, priority, dueDate string) string {
	task := yearplan.Task{
		Title:       title,
		Description: description,
		Status:      "planning",
		Priority:    priority,
		DueDate:     dueDate,
		CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}
	err := yearplan.AddTaskWithAccount(account, year, month, task)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return `{"success": true}`
}

// RawUpdateYearTask 更新任务状态
func RawUpdateYearTask(account string, year, month int, taskID, status string) string {
	goal, err := yearplan.GetMonthGoalWithAccount(account, year, month)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	var updatedTask yearplan.Task
	found := false
	for _, t := range goal.Tasks {
		if t.ID == taskID {
			updatedTask = t
			updatedTask.Status = status
			updatedTask.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
			found = true
			break
		}
	}

	if !found {
		return fmt.Sprintf(`{"error": "任务不存在: %s"}`, taskID)
	}

	err = yearplan.UpdateTaskWithAccount(account, year, month, taskID, updatedTask)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return `{"success": true}`
}

// =================================== TaskBreakdown Raw 接口 =========================================

// RawGetAllComplexTasks 获取所有任务
func RawGetAllComplexTasks(account string) string {
	mgr := taskbreakdown.NewTaskManager()
	tasks, err := mgr.ListTasks(account)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	type TaskSummary struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Priority int    `json:"priority"`
		Progress int    `json:"progress"`
	}
	summaries := make([]TaskSummary, 0, len(tasks))
	for _, t := range tasks {
		summaries = append(summaries, TaskSummary{
			ID:       t.ID,
			Title:    t.Title,
			Status:   t.Status,
			Priority: t.Priority,
			Progress: t.Progress,
		})
	}
	result := map[string]interface{}{
		"tasks": summaries,
		"total": len(summaries),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// RawGetComplexTasksByStatus 按状态筛选任务
func RawGetComplexTasksByStatus(account, status string) string {
	mgr := taskbreakdown.NewTaskManager()
	tasks, err := mgr.ListTasks(account)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	type TaskSummary struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Priority int    `json:"priority"`
		Progress int    `json:"progress"`
	}
	filtered := make([]TaskSummary, 0)
	for _, t := range tasks {
		if t.Status == status {
			filtered = append(filtered, TaskSummary{
				ID:       t.ID,
				Title:    t.Title,
				Priority: t.Priority,
				Progress: t.Progress,
			})
		}
	}
	result := map[string]interface{}{
		"tasks":  filtered,
		"status": status,
		"total":  len(filtered),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// RawGetComplexTaskStats 任务统计
func RawGetComplexTaskStats(account string) string {
	mgr := taskbreakdown.NewTaskManager()
	// GetStatistics 需要 account 和 rootID，空 rootID 获取总体统计
	tasks, err := mgr.ListTasks(account)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	totalTasks := len(tasks)
	completedTasks := 0
	inProgressTasks := 0
	planningTasks := 0

	for _, t := range tasks {
		switch t.Status {
		case "completed":
			completedTasks++
		case "in-progress":
			inProgressTasks++
		case "planning":
			planningTasks++
		}
	}

	completionRate := 0.0
	if totalTasks > 0 {
		completionRate = float64(completedTasks) / float64(totalTasks) * 100
	}

	result := map[string]interface{}{
		"total_tasks":       totalTasks,
		"completed_tasks":   completedTasks,
		"in_progress_tasks": inProgressTasks,
		"planning_tasks":    planningTasks,
		"completion_rate":   fmt.Sprintf("%.1f%%", completionRate),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// RawCreateComplexTask 创建任务
func RawCreateComplexTask(account, title, description, priority, startDate, endDate string) string {
	mgr := taskbreakdown.NewTaskManager()

	pri := 3 // 默认中等优先级
	switch priority {
	case "highest":
		pri = 1
	case "high":
		pri = 2
	case "medium":
		pri = 3
	case "low":
		pri = 4
	case "lowest":
		pri = 5
	}

	req := &taskbreakdown.TaskCreateRequest{
		Title:       title,
		Description: description,
		Priority:    pri,
		StartDate:   startDate,
		EndDate:     endDate,
		Status:      "planning",
	}

	task, err := mgr.CreateTask(account, req)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	data, _ := json.Marshal(task)
	return string(data)
}
