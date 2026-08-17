package http

import (
	"config"
	"encoding/json"
	"errors"
	goalpkg "goal"
	log "mylog"
	h "net/http"
	"persistence"
	"piagent"
	"sort"
	"strings"
	"time"
)

// HandleGoalTaskDrafts creates editable task drafts from the explicitly aligned goal.
func HandleGoalTaskDrafts(w h.ResponseWriter, r *h.Request) {
	if r.Method != h.MethodPost {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Level       string `json:"level"`
		Period      string `json:"period"`
		Instruction string `json:"instruction"`
		Provider    string `json:"provider"`
	}
	if err := json.NewDecoder(h.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		h.Error(w, "invalid JSON", h.StatusBadRequest)
		return
	}
	request.Level = strings.TrimSpace(request.Level)
	request.Period = strings.TrimSpace(request.Period)
	if request.Level == "" || request.Period == "" {
		h.Error(w, "level and period are required", h.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	current, err := goalpkg.GetGoal(account, request.Level, request.Period)
	if err != nil {
		h.Error(w, err.Error(), h.StatusInternalServerError)
		return
	}
	parentLevel, parentPeriod, err := parseGoalParentID(current.Level, current.ParentID)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}
	parent, err := goalpkg.FindGoal(account, parentLevel, parentPeriod)
	if err != nil {
		h.Error(w, err.Error(), h.StatusInternalServerError)
		return
	}
	if parent == nil || parent.Status == "archived" {
		h.Error(w, "aligned goal is unavailable", h.StatusBadRequest)
		return
	}

	context := piagent.GoalTaskContext{
		CurrentLevel:    current.Level,
		CurrentPeriod:   current.Period,
		CurrentOverview: current.Overview,
		ParentLevel:     parent.Level,
		ParentPeriod:    parent.Period,
		ParentOverview:  parent.Overview,
		ParentJudge:     parent.Judge,
		Instruction:     request.Instruction,
	}
	todayWeekday := 0
	if current.Level == goalpkg.LevelDaily && parent.Level == goalpkg.LevelWeekly {
		date, parseErr := time.Parse("2006-01-02", current.Period)
		if parseErr != nil {
			h.Error(w, "invalid daily period", h.StatusBadRequest)
			return
		}
		todayWeekday = int(date.Weekday())
		if todayWeekday == 0 {
			todayWeekday = 7
		}
	}
	for _, task := range parent.Tasks {
		if task.Status == "cancelled" || task.Status == "completed" {
			continue
		}
		schedules := make([]piagent.GoalExecutionSlot, 0, len(task.EffectiveSchedules()))
		for _, schedule := range task.EffectiveSchedules() {
			if todayWeekday == 0 || schedule.Weekday == todayWeekday {
				schedules = append(schedules, piagent.GoalExecutionSlot{Weekday: schedule.Weekday, TimeSlot: schedule.TimeSlot})
			}
		}
		if todayWeekday != 0 && len(schedules) == 0 {
			continue
		}
		context.ParentTasks = append(context.ParentTasks, piagent.GoalTaskReference{
			ID:          task.ID,
			Title:       task.Title,
			Description: task.Description,
			Importance:  task.Importance,
			Schedules:   schedules,
		})
	}
	sort.SliceStable(context.ParentTasks, func(i, j int) bool {
		return context.ParentTasks[i].Importance > context.ParentTasks[j].Importance
	})
	parentTaskLimit := 0
	switch current.Level {
	case goalpkg.LevelDaily:
		parentTaskLimit = 2
	case goalpkg.LevelWeekly:
		parentTaskLimit = 5
	case goalpkg.LevelMonthly:
		parentTaskLimit = 3
	}
	if parentTaskLimit > 0 && len(context.ParentTasks) > parentTaskLimit {
		context.ParentTasks = context.ParentTasks[:parentTaskLimit]
	}
	if todayWeekday != 0 && len(context.ParentTasks) == 0 {
		h.Error(w, "对齐的周目标今天没有排期任务，请先在周目标中设置星期和时段", h.StatusBadRequest)
		return
	}
	if todayWeekday != 0 {
		// 日计划只向模型暴露今天已排期的周任务，避免模型再次拆解整周目标。
		context.ParentOverview = ""
		context.ParentJudge = ""
	}
	for _, task := range current.Tasks {
		schedules := make([]piagent.GoalExecutionSlot, 0, len(task.EffectiveSchedules()))
		for _, schedule := range task.EffectiveSchedules() {
			schedules = append(schedules, piagent.GoalExecutionSlot{Weekday: schedule.Weekday, TimeSlot: schedule.TimeSlot})
		}
		context.ExistingTasks = append(context.ExistingTasks, piagent.GoalTaskReference{
			ID:          task.ID,
			Title:       task.Title,
			Description: task.Description,
			Importance:  task.Importance,
			Schedules:   schedules,
		})
	}

	migratePIProvidersToJSON(account)
	config.ReloadConfigFromSQLite(account)
	result, err := piagent.GenerateGoalTasks(account, request.Provider, context)
	if err != nil {
		log.ErrorF(log.ModuleBlog, "goal task generation failed account=%s goal=%s|%s: %v", account, request.Level, request.Period, err)
		h.Error(w, err.Error(), h.StatusBadGateway)
		return
	}
	if err := persistence.RecordPIUsage(account, result.Provider, result.Model, result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens, result.DurationMs, "goal_task_fill"); err != nil {
		log.ErrorF(log.ModuleBlog, "record goal task generation usage failed account=%s: %v", account, err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": result.Tasks})
}

func parseGoalParentID(currentLevel, parentID string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(parentID), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", errors.New("align the current goal before generating tasks")
	}
	parentLevel := strings.TrimSpace(parts[0])
	allowed := map[string]map[string]bool{
		goalpkg.LevelDaily:   {goalpkg.LevelWeekly: true, goalpkg.LevelMonthly: true},
		goalpkg.LevelWeekly:  {goalpkg.LevelMonthly: true},
		goalpkg.LevelMonthly: {goalpkg.LevelYearly: true},
	}
	if !allowed[currentLevel][parentLevel] {
		return "", "", errors.New("aligned goal level is invalid")
	}
	return parentLevel, strings.TrimSpace(parts[1]), nil
}
