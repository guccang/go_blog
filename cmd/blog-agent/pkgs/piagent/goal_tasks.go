package piagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	maxDailyGoalTasks   = 2
	maxWeeklyGoalTasks  = 5
	maxWeeklyTimeSlots  = 14
	maxDefaultGoalTasks = 3
)

type GoalExecutionSlot struct {
	Weekday  int    `json:"weekday,omitempty"`
	TimeSlot string `json:"time_slot"`
}

// GoalTaskReference keeps execution scheduling visible to the model.
type GoalTaskReference struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Importance  int                 `json:"importance"`
	Schedules   []GoalExecutionSlot `json:"schedules,omitempty"`
}

// GoalTaskContext contains only the goal data needed to draft lower-level tasks.
type GoalTaskContext struct {
	CurrentLevel    string
	CurrentPeriod   string
	CurrentOverview string
	ParentLevel     string
	ParentPeriod    string
	ParentOverview  string
	ParentJudge     string
	ParentTasks     []GoalTaskReference
	ExistingTasks   []GoalTaskReference
	Instruction     string
}

// GoalTaskDraft is an editable task draft returned to the goal page.
type GoalTaskDraft struct {
	Title           string              `json:"title"`
	Description     string              `json:"description,omitempty"`
	SourceTaskID    string              `json:"source_task_id,omitempty"`
	SourceTaskTitle string              `json:"source_task_title,omitempty"`
	Importance      int                 `json:"importance"`
	Priority        string              `json:"priority"`
	EstimateHours   float64             `json:"estimate_hours,omitempty"`
	Schedules       []GoalExecutionSlot `json:"schedules,omitempty"`
}

// GoalTaskDraftResult includes model metadata for usage accounting.
type GoalTaskDraftResult struct {
	Tasks      []GoalTaskDraft `json:"tasks"`
	Provider   string          `json:"provider"`
	Model      string          `json:"model"`
	Usage      TokenUsage      `json:"usage"`
	DurationMs int64           `json:"duration_ms"`
}

// GenerateGoalTasks drafts tasks from the current goal overview or an explicitly aligned parent goal.
func GenerateGoalTasks(account, provider string, context GoalTaskContext) (GoalTaskDraftResult, error) {
	if strings.TrimSpace(context.ParentOverview) == "" && len(context.ParentTasks) == 0 && strings.TrimSpace(context.CurrentOverview) == "" {
		return GoalTaskDraftResult{}, errors.New("目标没有可拆解的内容：请先填写目标概述，或对齐一个有内容的上层目标")
	}
	cfg, err := loadProvider(account, provider)
	if err != nil {
		return GoalTaskDraftResult{}, err
	}

	prompt := buildGoalTaskPrompt(context)
	startedAt := time.Now()
	result, err := chat(cfg, prompt)
	if err != nil {
		return GoalTaskDraftResult{}, err
	}
	tasks, err := parseGoalTaskDrafts(result.Content, context)
	if err != nil {
		return GoalTaskDraftResult{}, err
	}
	return GoalTaskDraftResult{
		Tasks:      tasks,
		Provider:   cfg.Name,
		Model:      cfg.Model,
		Usage:      result.Usage,
		DurationMs: time.Since(startedAt).Milliseconds(),
	}, nil
}

func buildGoalTaskPrompt(context GoalTaskContext) string {
	payload, _ := json.Marshal(map[string]any{
		"current_goal": map[string]any{
			"level":          context.CurrentLevel,
			"period":         context.CurrentPeriod,
			"overview":       context.CurrentOverview,
			"existing_tasks": context.ExistingTasks,
		},
		"aligned_goal": map[string]any{
			"level":    context.ParentLevel,
			"period":   context.ParentPeriod,
			"overview": context.ParentOverview,
			"judge":    context.ParentJudge,
			"tasks":    context.ParentTasks,
		},
		"user_instruction": strings.TrimSpace(context.Instruction),
	})
	scheduleInstruction := fmt.Sprintf("最多生成%d项。", maxTasksForLevel(context.CurrentLevel))
	switch context.CurrentLevel {
	case "daily":
		scheduleInstruction += "一天最多两项，只承接今天的工作，不要把整个周目标或月目标一次拆完。"
	case "weekly":
		scheduleInstruction += "只生成少量持续推进的任务，不要为每个时段复制任务；系统会依据用户定义的重要性分配多个执行时段。"
	}
	hasParentTasks := len(context.ParentTasks) > 0
	hasParent := hasParentTasks || strings.TrimSpace(context.ParentOverview) != ""
	var prompt string
	if hasParent {
		prompt = "你是目标页面的任务排期助手。用户已明确选择上层目标，请把上层目标转成当前周期内可以持续执行的任务草稿。"
	} else {
		prompt = "你是目标页面的任务排期助手。当前目标没有对齐上层目标，请把当前目标的概述拆解成当前周期内可以持续执行的任务草稿。"
	}
	prompt += "不要评价目标，不要主动扩展范围，不要重复已有任务。每项应具体、简短、可执行。" + scheduleInstruction
	format := `{"tasks":[{"title":"","description":"","estimate_hours":1}]}`
	if hasParentTasks {
		prompt += "每项必须通过 source_task_id 原样填写它所承接的上层任务 id，并通过 source_task_title 原样填写标题。不要判断或修改 importance 和 priority，系统会从上层任务继承。"
		format = `{"tasks":[{"title":"","description":"","source_task_id":"上层任务ID","source_task_title":"上层任务标题","estimate_hours":1}]}`
	}
	prompt += "estimate_hours 应为合理的正数。" +
		"只返回 JSON，不要使用 Markdown。格式：" + format + "。\n\n目标数据：" + string(payload)
	return prompt
}

func parseGoalTaskDrafts(content string, context GoalTaskContext) ([]GoalTaskDraft, error) {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	}
	var response struct {
		Tasks []GoalTaskDraft `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(content), &response); err != nil {
		return nil, fmt.Errorf("模型返回的内容不是有效 JSON：%v；原始返回开头：%.160s", err, content)
	}

	seen := make(map[string]struct{}, len(context.ExistingTasks)+len(response.Tasks))
	occupied := make(map[string]struct{}, len(context.ExistingTasks))
	for _, task := range context.ExistingTasks {
		seen[normalizeTaskTitle(task.Title)] = struct{}{}
		for _, schedule := range task.Schedules {
			if key := scheduleKey(context.CurrentLevel, schedule); key != "" {
				occupied[key] = struct{}{}
			}
		}
	}
	maxTasks := maxTasksForLevel(context.CurrentLevel)
	tasks := make([]GoalTaskDraft, 0, maxTasks)
	seenSources := make(map[string]struct{}, maxTasks)
	duplicateCount := 0
	for _, task := range response.Tasks {
		task.Title = strings.TrimSpace(task.Title)
		task.Description = strings.TrimSpace(task.Description)
		key := normalizeTaskTitle(task.Title)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			duplicateCount++
			continue
		}
		task.SourceTaskID = resolveSourceTaskID(task.SourceTaskID, task.SourceTaskTitle, context.ParentTasks)
		sourceKey := strings.TrimSpace(task.SourceTaskID)
		if sourceKey == "" {
			sourceKey = normalizeTaskTitle(task.SourceTaskTitle)
		}
		if context.CurrentLevel == "daily" && sourceKey != "" {
			if _, exists := seenSources[sourceKey]; exists {
				continue
			}
			seenSources[sourceKey] = struct{}{}
		}
		seen[key] = struct{}{}
		task.Importance = inheritedImportance(task.SourceTaskID, task.SourceTaskTitle, context.ParentTasks)
		task.Priority = priorityForGoalImportance(task.Importance)
		if task.EstimateHours < 0 {
			task.EstimateHours = 0
		}
		tasks = append(tasks, task)
		if len(tasks) == maxTasks {
			break
		}
	}
	if context.CurrentLevel == "weekly" {
		sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].Importance > tasks[j].Importance })
	}
	weeklyBudgets := weeklySlotBudgets(tasks, maxWeeklyTimeSlots-len(occupied))
	scheduledTasks := make([]GoalTaskDraft, 0, len(tasks))
	for i := range tasks {
		if context.CurrentLevel != "daily" && context.CurrentLevel != "weekly" {
			scheduledTasks = append(scheduledTasks, tasks[i])
			continue
		}
		desired := 1
		if context.CurrentLevel == "weekly" {
			desired = weeklyBudgets[i]
		}
		preferred := inheritedSchedules(tasks[i].SourceTaskID, tasks[i].SourceTaskTitle, context.ParentTasks)
		tasks[i].Schedules = allocateGoalTaskSchedules(context.CurrentLevel, desired, tasks[i].Importance, preferred, occupied)
		if len(tasks[i].Schedules) > 0 {
			scheduledTasks = append(scheduledTasks, tasks[i])
		}
	}
	kept := len(tasks)
	tasks = scheduledTasks
	if len(tasks) == 0 {
		switch {
		case len(response.Tasks) == 0:
			return nil, errors.New("模型没有返回任何任务，请补充目标概述或调整指令后重试")
		case kept == 0 && duplicateCount > 0:
			return nil, fmt.Errorf("模型返回了 %d 项任务，但都与现有任务重复，没有可添加的新任务", len(response.Tasks))
		case kept == 0:
			return nil, fmt.Errorf("模型返回了 %d 项任务，但均被过滤（标题为空或与现有任务重复），没有可添加的新任务", len(response.Tasks))
		default:
			return nil, errors.New("模型返回了任务，但当前周期已没有可分配的空闲时段，请先调整现有任务排期")
		}
	}
	return tasks, nil
}

func resolveSourceTaskID(sourceID, sourceTitle string, parents []GoalTaskReference) string {
	for _, parent := range parents {
		if sourceID != "" && parent.ID == sourceID {
			return parent.ID
		}
	}
	key := normalizeTaskTitle(sourceTitle)
	for _, parent := range parents {
		if key != "" && normalizeTaskTitle(parent.Title) == key {
			return parent.ID
		}
	}
	return ""
}

func weeklySlotBudgets(tasks []GoalTaskDraft, available int) []int {
	budgets := make([]int, len(tasks))
	if len(tasks) == 0 || available <= 0 {
		return budgets
	}
	totalDesired := 0
	for _, task := range tasks {
		totalDesired += task.Importance
	}
	if totalDesired <= available {
		for i, task := range tasks {
			budgets[i] = task.Importance
		}
		return budgets
	}
	if available < len(tasks) {
		for i := 0; i < available; i++ {
			budgets[i] = 1
		}
		return budgets
	}
	for i := range budgets {
		budgets[i] = 1
	}
	remaining := available - len(tasks)
	extraWeight := totalDesired - len(tasks)
	remainders := make([]int, len(tasks))
	used := 0
	for i, task := range tasks {
		weight := task.Importance - 1
		extra := remaining * weight / extraWeight
		budgets[i] += extra
		used += extra
		remainders[i] = remaining * weight % extraWeight
	}
	for left := remaining - used; left > 0; left-- {
		best := 0
		for i := 1; i < len(tasks); i++ {
			if remainders[i] > remainders[best] {
				best = i
			}
		}
		budgets[best]++
		remainders[best] = -1
	}
	return budgets
}

func maxTasksForLevel(level string) int {
	switch level {
	case "daily":
		return maxDailyGoalTasks
	case "weekly":
		return maxWeeklyGoalTasks
	default:
		return maxDefaultGoalTasks
	}
}

func scheduleKey(level string, schedule GoalExecutionSlot) string {
	if schedule.TimeSlot != "morning" && schedule.TimeSlot != "afternoon" {
		return ""
	}
	if level == "daily" {
		return schedule.TimeSlot
	}
	if level == "weekly" && schedule.Weekday >= 1 && schedule.Weekday <= 7 {
		return fmt.Sprintf("%d:%s", schedule.Weekday, schedule.TimeSlot)
	}
	return ""
}

func inheritedImportance(sourceID, sourceTitle string, parents []GoalTaskReference) int {
	key := normalizeTaskTitle(sourceTitle)
	importance := 3
	for _, parent := range parents {
		value := parent.Importance
		if value < 1 || value > 5 {
			value = 3
		}
		if (sourceID != "" && parent.ID == sourceID) || (normalizeTaskTitle(parent.Title) == key && key != "") {
			return value
		}
		if value > importance {
			importance = value
		}
	}
	return importance
}

func inheritedSchedules(sourceID, sourceTitle string, parents []GoalTaskReference) []GoalExecutionSlot {
	key := normalizeTaskTitle(sourceTitle)
	for _, parent := range parents {
		if (sourceID != "" && parent.ID == sourceID) || (normalizeTaskTitle(parent.Title) == key && key != "") {
			return parent.Schedules
		}
	}
	return nil
}

func priorityForGoalImportance(importance int) string {
	if importance >= 4 {
		return "high"
	}
	if importance <= 2 {
		return "low"
	}
	return "medium"
}

func allocateGoalTaskSchedules(level string, desired, importance int, preferred []GoalExecutionSlot, occupied map[string]struct{}) []GoalExecutionSlot {
	result := make([]GoalExecutionSlot, 0, desired)
	if level == "daily" {
		for _, schedule := range preferred {
			candidate := GoalExecutionSlot{TimeSlot: schedule.TimeSlot}
			if key := scheduleKey(level, candidate); key != "" {
				if _, exists := occupied[key]; !exists {
					occupied[key] = struct{}{}
					return []GoalExecutionSlot{candidate}
				}
			}
		}
		if len(preferred) > 0 {
			return nil
		}
	}
	weekdays := []int{0}
	if level == "weekly" {
		weekdays = []int{1, 2, 3, 4, 5, 6, 7}
	}
	slots := []string{"afternoon", "morning"}
	if importance >= 4 {
		slots = []string{"morning", "afternoon"}
	}
	for _, slot := range slots {
		for _, weekday := range weekdays {
			candidate := GoalExecutionSlot{Weekday: weekday, TimeSlot: slot}
			key := scheduleKey(level, candidate)
			if _, exists := occupied[key]; !exists {
				occupied[key] = struct{}{}
				result = append(result, candidate)
				if len(result) == desired {
					return result
				}
			}
		}
	}
	return result
}

func normalizeTaskTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), ""))
}
