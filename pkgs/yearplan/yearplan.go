package yearplan

import (
	"fmt"
	"time"
	"encoding/json"
	"blog"
	"module"
	log "mylog"
	"strconv"
)

// Info provides version info about the yearplan module
func Info() {
	fmt.Println("info yearplan v1.0")
}

// RegisterHandlers sets up the HTTP handlers for yearplan routes
func RegisterHandlers() {
	log.Debug("Registering yearplan handlers")
}

// MonthGoal represents a monthly work goal
type MonthGoal struct {
	Year     int                    `json:"year"`
	Month    int                    `json:"month"`
	Overview string                 `json:"overview"`
	Weeks    map[int]*WeekGoal      `json:"weeks"` // week number -> week goal
	Tasks    []Task                 `json:"tasks"`
}

// WeekGoal represents a weekly work goal
type WeekGoal struct {
	Year     int    `json:"year"`
	Month    int    `json:"month"`
	Week     int    `json:"week"`
	Overview string `json:"overview"`
	Tasks    []Task `json:"tasks"`
}

// Task represents a work task
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // pending, in_progress, completed, cancelled
	Priority    string `json:"priority"` // low, medium, high, urgent
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	DueDate     string `json:"due_date,omitempty"`
}

// GetCurrentMonth returns current year and month
func GetCurrentMonth() (int, int) {
	now := time.Now()
	return now.Year(), int(now.Month())
}

// GetWeekNumber returns the week number of the month for a given date
func GetWeekNumber(date time.Time) int {
	// Calculate week number within the month
	firstDay := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	weekStart := firstDay.AddDate(0, 0, -int(firstDay.Weekday()))
	
	daysDiff := int(date.Sub(weekStart).Hours() / 24)
	return (daysDiff / 7) + 1
}

// GetWeeksInMonth returns the number of weeks in a month
func GetWeeksInMonth(year, month int) int {
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	lastDay := firstDay.AddDate(0, 1, -1)
	
	firstWeek := GetWeekNumber(firstDay)
	lastWeek := GetWeekNumber(lastDay)
	
	return lastWeek - firstWeek + 1
}

// GetMonthGoal retrieves or creates a month goal
func GetMonthGoal(year, month int) (*MonthGoal, error) {
	title := fmt.Sprintf("月度目标_%d-%02d", year, month)
	
	// Try to get existing blog
	b := blog.GetBlog(title)
	if b == nil {
		// Create new month goal
		goal := &MonthGoal{
			Year:     year,
			Month:    month,
			Overview: "",
			Weeks:    make(map[int]*WeekGoal),
			Tasks:    []Task{},
		}
		
		// Initialize weeks
		weeksCount := GetWeeksInMonth(year, month)
		for week := 1; week <= weeksCount; week++ {
			goal.Weeks[week] = &WeekGoal{
				Year:     year,
				Month:    month,
				Week:     week,
				Overview: "",
				Tasks:    []Task{},
			}
		}
		
		return goal, nil
	}
	
	// Parse existing blog content
	var goal MonthGoal
	err := json.Unmarshal([]byte(b.Content), &goal)
	if err != nil {
		return nil, fmt.Errorf("failed to parse month goal: %w", err)
	}
	
	// Ensure weeks are initialized
	if goal.Weeks == nil {
		goal.Weeks = make(map[int]*WeekGoal)
	}
	
	// Initialize missing weeks
	weeksCount := GetWeeksInMonth(year, month)
	for week := 1; week <= weeksCount; week++ {
		if goal.Weeks[week] == nil {
			goal.Weeks[week] = &WeekGoal{
				Year:     year,
				Month:    month,
				Week:     week,
				Overview: "",
				Tasks:    []Task{},
			}
		}
	}
	
	return &goal, nil
}

// SaveMonthGoal saves a month goal to the blog system
func SaveMonthGoal(goal *MonthGoal) error {
	title := fmt.Sprintf("月度目标_%d-%02d", goal.Year, goal.Month)
	
	// Convert to JSON
	content, err := json.MarshalIndent(goal, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal month goal: %w", err)
	}
	
	// Create blog data
	udb := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		AuthType: module.EAuthType_private,
		Tags:     "月度目标",
		Encrypt:  0,
	}
	
	// Check if blog exists
	existingBlog := blog.GetBlog(title)
	if existingBlog != nil {
		// Update existing blog
		ret := blog.ModifyBlog(udb)
		if ret != 0 {
			return fmt.Errorf("failed to update month goal")
		}
	} else {
		// Create new blog
		ret := blog.AddBlog(udb)
		if ret != 0 {
			return fmt.Errorf("failed to create month goal")
		}
	}
	
	return nil
}

// GetWeekGoal retrieves a specific week goal
func GetWeekGoal(year, month, week int) (*WeekGoal, error) {
	monthGoal, err := GetMonthGoal(year, month)
	if err != nil {
		return nil, err
	}
	
	weekGoal, exists := monthGoal.Weeks[week]
	if !exists {
		return nil, fmt.Errorf("week %d not found in month %d-%02d", week, year, month)
	}
	
	return weekGoal, nil
}

// SaveWeekGoal saves a week goal
func SaveWeekGoal(weekGoal *WeekGoal) error {
	monthGoal, err := GetMonthGoal(weekGoal.Year, weekGoal.Month)
	if err != nil {
		return err
	}
	
	monthGoal.Weeks[weekGoal.Week] = weekGoal
	return SaveMonthGoal(monthGoal)
}

// AddTask adds a task to a month goal
func AddTask(year, month int, task Task) error {
	monthGoal, err := GetMonthGoal(year, month)
	if err != nil {
		return err
	}
	
	// Set timestamps if not provided
	if task.CreatedAt == "" {
		task.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	task.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	
	// Set default status if not provided
	if task.Status == "" {
		task.Status = "pending"
	}
	
	// Set default priority if not provided
	if task.Priority == "" {
		task.Priority = "medium"
	}
	
	// Generate ID if not provided
	if task.ID == "" {
		task.ID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	
	monthGoal.Tasks = append(monthGoal.Tasks, task)
	return SaveMonthGoal(monthGoal)
}

// UpdateTask updates a task in a month goal
func UpdateTask(year, month int, taskID string, updatedTask Task) error {
	monthGoal, err := GetMonthGoal(year, month)
	if err != nil {
		return err
	}
	
	// Find and update task
	for i, task := range monthGoal.Tasks {
		if task.ID == taskID {
			updatedTask.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
			monthGoal.Tasks[i] = updatedTask
			return SaveMonthGoal(monthGoal)
		}
	}
	
	return fmt.Errorf("task with ID %s not found", taskID)
}

// DeleteTask deletes a task from a month goal
func DeleteTask(year, month int, taskID string) error {
	monthGoal, err := GetMonthGoal(year, month)
	if err != nil {
		return err
	}
	
	// Find and remove task
	for i, task := range monthGoal.Tasks {
		if task.ID == taskID {
			monthGoal.Tasks = append(monthGoal.Tasks[:i], monthGoal.Tasks[i+1:]...)
			return SaveMonthGoal(monthGoal)
		}
	}
	
	return fmt.Errorf("task with ID %s not found", taskID)
}

// GetMonthGoals retrieves all month goals for a year
func GetMonthGoals(year int) (map[int]*MonthGoal, error) {
	goals := make(map[int]*MonthGoal)
	
	// Get all blogs with month goal pattern
	for _, b := range blog.Blogs {
		var goalYear, goalMonth int
		_, err := fmt.Sscanf(b.Title, "月度目标_%d-%02d", &goalYear, &goalMonth)
		if err == nil && goalYear == year {
			var goal MonthGoal
			err := json.Unmarshal([]byte(b.Content), &goal)
			if err == nil {
				goals[goalMonth] = &goal
			}
		}
	}
	
	return goals, nil
}

// InitYearPlanModule initializes the year plan module
func InitYearPlanModule() error {
	log.Debug("Initializing year plan module")
	return nil
} 