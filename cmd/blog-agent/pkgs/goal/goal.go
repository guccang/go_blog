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
	Level     string `json:"level"`
	Period    string `json:"period"`
	ParentID  string `json:"parent_id,omitempty"` // OKR 对齐
	Overview  string `json:"overview"`
	Judge     string `json:"judge,omitempty"`
	Tasks     []Task `json:"tasks"`
	Progress  int    `json:"progress"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Task represents a sub-task within a goal
type Task struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Status        string     `json:"status"`   // pending, in_progress, completed, cancelled
	Priority      string     `json:"priority"` // low, medium, high
	Deadline      string     `json:"deadline,omitempty"`       // YYYY-MM-DD
	EstimateHours float64    `json:"estimate_hours,omitempty"`
	Subtasks      []Subtask  `json:"subtasks,omitempty"`
	Notes         []TaskNote `json:"notes,omitempty"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
}

// Subtask represents a checkable sub-item within a Task
type Subtask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // pending, completed
}

// TaskNote represents a note attached to a Task
type TaskNote struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// GoalSummary provides a lightweight summary for cortana-friendly access
type GoalSummary struct {
	Level      string `json:"level"`
	Period     string `json:"period"`
	Overview   string `json:"overview"`
	Judge      string `json:"judge,omitempty"`
	Progress   int    `json:"progress"`
	TotalTasks int    `json:"total_tasks"`
	DoneTasks  int    `json:"done_tasks"`
	PendingTasks int  `json:"pending_tasks"`
	Status     string `json:"status"`
}

// Review represents a periodic review of goals at a given level
type Review struct {
	ID        string `json:"id"`
	Level     string `json:"level"`     // weekly, monthly
	Period    string `json:"period"`
	Content   string `json:"content"`   // markdown
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
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

func parentGoalTitle(level, period string) string {
	return fmt.Sprintf("目标_%s_%s", level, period)
}

func reviewTitle(level, period string) string {
	return fmt.Sprintf("回顾_%s_%s", level, period)
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

// DeleteGoal removes an entire goal (blog entry) for the given level and period
func DeleteGoal(account, level, period string) error {
	title := goalTitle(level, period)
	ret := blog.DeleteBlogWithAccount(account, title)
	if ret != 0 {
		return fmt.Errorf("failed to delete goal: not found or system file")
	}
	return nil
}

// PrevPeriod computes the previous period for navigation
func PrevPeriod(level, period string) (string, error) {
	switch level {
	case LevelDaily:
		t, err := time.Parse("2006-01-02", period)
		if err != nil {
			return "", fmt.Errorf("invalid daily period: %s", period)
		}
		return t.AddDate(0, 0, -1).Format("2006-01-02"), nil
	case LevelWeekly:
		year, week, err := parseISOWeek(period)
		if err != nil {
			return "", err
		}
		// Get the Monday of the given ISO week, then subtract 7 days
		t := isoWeekStart(year, week).AddDate(0, 0, -7)
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w), nil
	case LevelMonthly:
		t, err := time.Parse("2006-01", period)
		if err != nil {
			return "", fmt.Errorf("invalid monthly period: %s", period)
		}
		return t.AddDate(0, -1, 0).Format("2006-01"), nil
	case LevelYearly:
		year, err := strconv.Atoi(period)
		if err != nil {
			return "", fmt.Errorf("invalid yearly period: %s", period)
		}
		return fmt.Sprintf("%d", year-1), nil
	default:
		return "", fmt.Errorf("unknown level: %s", level)
	}
}

// NextPeriod computes the next period for navigation
func NextPeriod(level, period string) (string, error) {
	switch level {
	case LevelDaily:
		t, err := time.Parse("2006-01-02", period)
		if err != nil {
			return "", fmt.Errorf("invalid daily period: %s", period)
		}
		return t.AddDate(0, 0, 1).Format("2006-01-02"), nil
	case LevelWeekly:
		year, week, err := parseISOWeek(period)
		if err != nil {
			return "", err
		}
		t := isoWeekStart(year, week).AddDate(0, 0, 7)
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w), nil
	case LevelMonthly:
		t, err := time.Parse("2006-01", period)
		if err != nil {
			return "", fmt.Errorf("invalid monthly period: %s", period)
		}
		return t.AddDate(0, 1, 0).Format("2006-01"), nil
	case LevelYearly:
		year, err := strconv.Atoi(period)
		if err != nil {
			return "", fmt.Errorf("invalid yearly period: %s", period)
		}
		return fmt.Sprintf("%d", year+1), nil
	default:
		return "", fmt.Errorf("unknown level: %s", level)
	}
}

// parseISOWeek parses "2006-W05" format, returning year and week number
func parseISOWeek(s string) (int, int, error) {
	var year, week int
	if _, err := fmt.Sscanf(s, "%d-W%d", &year, &week); err != nil {
		return 0, 0, fmt.Errorf("invalid ISO week format: %s", s)
	}
	return year, week, nil
}

// isoWeekStart returns the Monday of the given ISO year and week
func isoWeekStart(year, week int) time.Time {
	// January 4th is always in ISO week 1
	t := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	// Back up to Monday
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, -1)
	}
	// Add (week-1) weeks
	return t.AddDate(0, 0, 7*(week-1))
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

// ListGoalsByLevel lists all goals for a given level, optionally filtered by year (0 = no filter)
func ListGoalsByLevel(account, level string, year int) ([]*GoalSummary, error) {
	yearStr := fmt.Sprintf("%d", year)
	var summaries []*GoalSummary

	for _, b := range blog.GetBlogsWithAccount(account) {
		if !strings.Contains(b.Title, "目标_"+level) {
			continue
		}
		var goal Goal
		if err := json.Unmarshal([]byte(b.Content), &goal); err != nil {
			continue
		}
		if year > 0 && !strings.HasPrefix(goal.Period, yearStr) {
			continue
		}
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
		Judge:        g.Judge,
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

// GetParentGoals returns goals from the parent level for OKR alignment
func GetParentGoals(account, level, period string) ([]*GoalSummary, error) {
	if level == LevelYearly {
		return nil, nil // 年目标没有父级
	}

	parentLevels := map[string]string{
		LevelDaily:   LevelWeekly,
		LevelWeekly:  LevelMonthly,
		LevelMonthly: LevelYearly,
	}

	parentLevel := parentLevels[level]
	var candidates []*GoalSummary

	for _, b := range blog.GetBlogsWithAccount(account) {
		if !strings.Contains(b.Title, "目标_"+parentLevel) {
			continue
		}
		var goal Goal
		if err := json.Unmarshal([]byte(b.Content), &goal); err != nil {
			continue
		}
		if goal.Status == "archived" {
			continue
		}
		candidates = append(candidates, goal.Summary())
	}
	return candidates, nil
}

// AddTaskNote adds a note to a specific task within a goal
func AddTaskNote(account, level, period, taskID, content string) error {
	goal, err := GetGoal(account, level, period)
	if err != nil {
		return err
	}

	for i, t := range goal.Tasks {
		if t.ID == taskID {
			note := TaskNote{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				Content:   content,
				CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
			}
			goal.Tasks[i].Notes = append(goal.Tasks[i].Notes, note)
			return SaveGoal(account, goal)
		}
	}
	return fmt.Errorf("task %s not found", taskID)
}

// GetReview retrieves a review blog entry
func GetReview(account, level, period string) (*Review, error) {
	title := reviewTitle(level, period)
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		return nil, nil
	}
	var review Review
	if err := json.Unmarshal([]byte(b.Content), &review); err != nil {
		return nil, fmt.Errorf("failed to parse review: %w", err)
	}
	return &review, nil
}

// SaveReview persists a review to the blog system
func SaveReview(account string, review *Review) error {
	review.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if review.CreatedAt == "" {
		review.CreatedAt = review.UpdatedAt
	}

	title := reviewTitle(review.Level, review.Period)
	content, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal review: %w", err)
	}

	udb := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		AuthType: module.EAuthType_private,
		Tags:     "回顾_" + review.Level,
		Encrypt:  0,
		Account:  account,
	}

	existing := blog.GetBlogWithAccount(account, title)
	if existing != nil {
		blog.ModifyBlogWithAccount(account, udb)
	} else {
		blog.AddBlogWithAccount(account, udb)
	}
	return nil
}

// GenerateReview creates a review draft from a goal's task status
func GenerateReview(account, level, period string) (*Review, error) {
	goal, err := GetGoal(account, level, period)
	if err != nil {
		return nil, err
	}

	completed := 0
	for _, t := range goal.Tasks {
		if t.Status == "completed" {
			completed++
		}
	}

	total := len(goal.Tasks)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s 回顾\n\n", periodLabel(level, period)))
	sb.WriteString(fmt.Sprintf("## 完成情况\n\n"))
	sb.WriteString(fmt.Sprintf("- 总任务: %d\n", total))
	sb.WriteString(fmt.Sprintf("- 已完成: %d\n", completed))
	sb.WriteString(fmt.Sprintf("- 完成率: %d%%\n\n", goal.Progress))
	sb.WriteString("## 任务详情\n\n")
	for _, t := range goal.Tasks {
		icon := "[ ]"
		if t.Status == "completed" {
			icon = "[x]"
		}
		sb.WriteString(fmt.Sprintf("- %s %s\n", icon, t.Title))
	}
	sb.WriteString("\n## 总结反思\n\n")

	review := &Review{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Level:     level,
		Period:    period,
		Content:   sb.String(),
		Completed: completed,
		Total:     total,
	}
	return review, nil
}

func periodLabel(level, period string) string {
	switch level {
	case LevelWeekly:
		year, week, _ := parseISOWeek(period)
		return fmt.Sprintf("%d年第%d周", year, week)
	case LevelMonthly:
		return period + "月"
	}
	return period
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
