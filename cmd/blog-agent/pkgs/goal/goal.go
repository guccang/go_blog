package goal

import (
	"blog"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"strconv"
	"strings"
	"time"
)

// Level constants for goal hierarchy
const (
	LevelDaily   = "daily"
	LevelWeekly  = "weekly"
	LevelMonthly = "monthly"
	LevelYearly  = "yearly"
)

// Goal represents a unified goal at any level (daily/weekly/monthly/yearly)
type Goal struct {
	ID        string `json:"id"`
	Level     string `json:"level"`      // daily, weekly, monthly, yearly
	Period    string `json:"period"`     // 2026-05-02, 2026-W18, 2026-05, 2026
	Overview  string `json:"overview"`   // main goal description
	Tasks     []Task `json:"tasks"`      // sub-tasks
	Progress  int    `json:"progress"`   // 0-100, auto-calculated from tasks
	Status    string `json:"status"`     // active, completed, archived
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Task represents a sub-task within a goal
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`   // pending, in_progress, completed, cancelled
	Priority    string `json:"priority"` // low, medium, high
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// GoalSummary provides a lightweight summary for cortana-friendly access
type GoalSummary struct {
	Level      string `json:"level"`
	Period     string `json:"period"`
	Overview   string `json:"overview"`
	Progress   int    `json:"progress"`
	TotalTasks int    `json:"total_tasks"`
	DoneTasks  int    `json:"done_tasks"`
	PendingTasks int  `json:"pending_tasks"`
	Status     string `json:"status"`
}

// CurrentPeriods returns current period identifiers for all levels
func CurrentPeriods() (daily, weekly, monthly, yearly string) {
	now := time.Now()
	year, month, day := now.Date()
	_, week := now.ISOWeek()

	_ = day
	daily = now.Format("2006-01-02")
	weekly = fmt.Sprintf("%d-W%02d", year, week)
	monthly = fmt.Sprintf("%d-%02d", year, month)
	yearly = fmt.Sprintf("%d", year)
	return
}

// ============================================================================
// Blog title generation
// ============================================================================

func goalTitle(level, period string) string {
	return fmt.Sprintf("目标_%s_%s", level, period)
}

func tagForLevel(level string) string {
	return "目标_" + level
}

// ============================================================================
// CRUD Operations
// ============================================================================

// GetGoal retrieves a goal, auto-creating if it doesn't exist
func GetGoal(account, level, period string) (*Goal, error) {
	title := goalTitle(level, period)
	b := blog.GetBlogWithAccount(account, title)

	if b == nil {
		return newGoal(level, period), nil
	}

	var goal Goal
	if err := json.Unmarshal([]byte(b.Content), &goal); err != nil {
		return nil, fmt.Errorf("failed to parse goal: %w", err)
	}

	// Ensure defaults
	if goal.Tasks == nil {
		goal.Tasks = []Task{}
	}
	if goal.Status == "" {
		goal.Status = "active"
	}
	goal.recalcProgress()
	return &goal, nil
}

// SaveGoal persists a goal to the blog system
func SaveGoal(account string, goal *Goal) error {
	goal.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if goal.CreatedAt == "" {
		goal.CreatedAt = goal.UpdatedAt
	}
	if goal.Status == "" {
		goal.Status = "active"
	}
	goal.recalcProgress()

	title := goalTitle(goal.Level, goal.Period)
	content, err := json.MarshalIndent(goal, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal goal: %w", err)
	}

	udb := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		AuthType: module.EAuthType_private,
		Tags:     tagForLevel(goal.Level),
		Encrypt:  0,
		Account:  account,
	}

	existing := blog.GetBlogWithAccount(account, title)
	if existing != nil {
		if ret := blog.ModifyBlogWithAccount(account, udb); ret != 0 {
			return fmt.Errorf("failed to update goal")
		}
	} else {
		if ret := blog.AddBlogWithAccount(account, udb); ret != 0 {
			return fmt.Errorf("failed to create goal")
		}
	}
	return nil
}

// AddTask adds a task to a goal
func AddTask(account, level, period string, task Task) error {
	goal, err := GetGoal(account, level, period)
	if err != nil {
		return err
	}

	if task.CreatedAt == "" {
		task.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	task.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if task.Status == "" {
		task.Status = "pending"
	}
	if task.Priority == "" {
		task.Priority = "medium"
	}
	if task.ID == "" {
		task.ID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	goal.Tasks = append(goal.Tasks, task)
	return SaveGoal(account, goal)
}

// UpdateTask updates a task within a goal
func UpdateTask(account, level, period, taskID string, updated Task) error {
	goal, err := GetGoal(account, level, period)
	if err != nil {
		return err
	}

	for i, t := range goal.Tasks {
		if t.ID == taskID {
			updated.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
			if updated.ID == "" {
				updated.ID = taskID
			}
			goal.Tasks[i] = updated
			return SaveGoal(account, goal)
		}
	}
	return fmt.Errorf("task %s not found", taskID)
}

// DeleteTask removes a task from a goal
func DeleteTask(account, level, period, taskID string) error {
	goal, err := GetGoal(account, level, period)
	if err != nil {
		return err
	}

	for i, t := range goal.Tasks {
		if t.ID == taskID {
			goal.Tasks = append(goal.Tasks[:i], goal.Tasks[i+1:]...)
			return SaveGoal(account, goal)
		}
	}
	return fmt.Errorf("task %s not found", taskID)
}

// GetCurrentGoals returns active goals for all current periods
func GetCurrentGoals(account string) (map[string]*GoalSummary, error) {
	daily, weekly, monthly, yearly := CurrentPeriods()
	summaries := make(map[string]*GoalSummary)

	levels := []struct{ level, period string }{
		{LevelDaily, daily},
		{LevelWeekly, weekly},
		{LevelMonthly, monthly},
		{LevelYearly, yearly},
	}

	for _, l := range levels {
		goal, err := GetGoal(account, l.level, l.period)
		if err != nil {
			continue
		}
		summaries[l.level] = goal.Summary()
	}
	return summaries, nil
}

// ListGoalsByLevel lists all goals for a given level within a year range
func ListGoalsByLevel(account, level string, year int) ([]*GoalSummary, error) {
	prefix := goalTitle(level, fmt.Sprintf("%d", year))
	var summaries []*GoalSummary

	for _, b := range blog.GetBlogsWithAccount(account) {
		if !strings.Contains(b.Title, "目标_"+level) {
			continue
		}
		var goal Goal
		if err := json.Unmarshal([]byte(b.Content), &goal); err != nil {
			continue
		}
		_ = prefix
		summaries = append(summaries, goal.Summary())
	}
	return summaries, nil
}

// ============================================================================
// Helper methods
// ============================================================================

func (g *Goal) recalcProgress() {
	if len(g.Tasks) == 0 {
		g.Progress = 0
		return
	}
	done := 0
	for _, t := range g.Tasks {
		if t.Status == "completed" {
			done++
		}
	}
	g.Progress = done * 100 / len(g.Tasks)
}

// Summary returns a lightweight summary of the goal
func (g *Goal) Summary() *GoalSummary {
	done := 0
	pending := 0
	for _, t := range g.Tasks {
		if t.Status == "completed" {
			done++
		} else if t.Status != "cancelled" {
			pending++
		}
	}
	return &GoalSummary{
		Level:        g.Level,
		Period:       g.Period,
		Overview:     g.Overview,
		Progress:     g.Progress,
		TotalTasks:   len(g.Tasks),
		DoneTasks:    done,
		PendingTasks: pending,
		Status:       g.Status,
	}
}

func newGoal(level, period string) *Goal {
	return &Goal{
		ID:        strconv.FormatInt(time.Now().UnixNano(), 10),
		Level:     level,
		Period:    period,
		Overview:  "",
		Tasks:     []Task{},
		Progress:  0,
		Status:    "active",
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
}

// Info prints module info
func Info() {
	log.InfoF(log.ModuleYearPlan, "info goal v1.0")
}

// InitGoalModule initializes the goal module
func InitGoalModule() error {
	log.Debug(log.ModuleYearPlan, "Initializing goal module")
	return nil
}
