package todolist

import (
	"blog"
	"encoding/json"
	"fmt"
	"module"
	"strings"
	"time"
)

// TodoItem represents a single todo item
type TodoItem struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Completed  bool      `json:"completed"`
	CreatedAt  time.Time `json:"created_at"`
	Hours      int       `json:"hours,omitempty"`
	Minutes    int       `json:"minutes,omitempty"`
	Urgency    int       `json:"urgency,omitempty"`     // 紧急程度: 1-3 (1最紧急)
	Importance int       `json:"importance,omitempty"`  // 重要程度: 1-3 (1最重要)
	Score      int       `json:"score,omitempty"`       // 积分 (计算字段)
}

// TodoList represents a collection of todo items for a specific date
type TodoList struct {
	Date  string     `json:"date"`
	Items []TodoItem `json:"items"`
	Order []string   `json:"order,omitempty"` // Array of todo IDs in the desired order
}

// TodoManager handles todo list operations using the blog system
type TodoManager struct {
	// No need for redisClient anymore
}

// NewTodoManager creates a new TodoManager instance
func NewTodoManager() *TodoManager {
	return &TodoManager{}
}

// generateBlogTitle generates a blog title for a specific date's todo list
func generateBlogTitle(date string) string {
	return fmt.Sprintf("todolist-%s", date)
}

// getDateFromTitle extracts the date from a todo list blog title
func getDateFromTitle(title string) string {
	if strings.HasPrefix(title, "todolist-") {
		return strings.TrimPrefix(title, "todolist-")
	}
	return ""
}

// AddTodo adds a new todo item to a specific date's list
func (tm *TodoManager) AddTodo(account, date, content string, hours, minutes, urgency, importance int) (*TodoItem, error) {
	// Get or create todo list for the date
	todoList, err := tm.GetTodosByDate(account, date)
	if err != nil {
		todoList = TodoList{
			Date:  date,
			Items: []TodoItem{},
		}
	}

	// Create new todo item
	item := TodoItem{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		Content:    content,
		Completed:  false,
		CreatedAt:  time.Now(),
		Hours:      hours,
		Minutes:    minutes,
		Urgency:    urgency,
		Importance: importance,
		Score:      calculateScore(urgency, importance, hours, minutes),
	}

	// Add item to list
	todoList.Items = append(todoList.Items, item)

	// Save to blog
	if err := tm.saveTodosToBlog(account, todoList); err != nil {
		return nil, err
	}

	return &item, nil
}

// DeleteTodo removes a todo item by ID
func (tm *TodoManager) DeleteTodo(account, date, id string) error {
	// Get todo list for the date
	todoList, err := tm.GetTodosByDate(account, date)
	if err != nil {
		return err
	}

	// Find and remove the item
	found := false
	updatedItems := make([]TodoItem, 0, len(todoList.Items))
	for _, item := range todoList.Items {
		if item.ID != id {
			updatedItems = append(updatedItems, item)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("todo item not found")
	}

	// Update list
	todoList.Items = updatedItems

	// Save to blog
	return tm.saveTodosToBlog(account, todoList)
}

// ToggleTodo toggles the completion status of a todo item
func (tm *TodoManager) ToggleTodo(account, date, id string) error {
	// Get todo list for the date
	todoList, err := tm.GetTodosByDate(account, date)
	if err != nil {
		return err
	}

	// Find and toggle the item
	found := false
	for i := range todoList.Items {
		if todoList.Items[i].ID == id {
			todoList.Items[i].Completed = !todoList.Items[i].Completed
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("todo item not found")
	}

	// Save to blog
	return tm.saveTodosToBlog(account, todoList)
}

// UpdateTodoTime updates the time spent on a todo item
func (tm *TodoManager) UpdateTodoTime(account, date, id string, hours, minutes int) error {
	// Get todo list for the date
	todoList, err := tm.GetTodosByDate(account, date)
	if err != nil {
		return err
	}

	// Find and update the item
	found := false
	for i := range todoList.Items {
		if todoList.Items[i].ID == id {
			todoList.Items[i].Hours = hours
			todoList.Items[i].Minutes = minutes
			// Recalculate score when time changes
			todoList.Items[i].Score = calculateScore(
				todoList.Items[i].Urgency,
				todoList.Items[i].Importance,
				hours,
				minutes,
			)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("todo item not found")
	}

	// Save to blog
	return tm.saveTodosToBlog(account, todoList)
}

// GetTodosByDate retrieves the todo list for a specific date
func (tm *TodoManager) GetTodosByDate(account, date string) (TodoList, error) {
	title := generateBlogTitle(date)

	// Find blog by title using account-based interface
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		return TodoList{Date: date, Items: []TodoItem{}}, nil
	}

	// Parse content as JSON
	var todoList TodoList
	if err := json.Unmarshal([]byte(b.Content), &todoList); err != nil {
		return TodoList{Date: date, Items: []TodoItem{}}, fmt.Errorf("failed to parse todo list: %w", err)
	}

	return todoList, nil
}

// GetAllTodos retrieves all todo lists from the blog system
func (tm *TodoManager) GetAllTodos(account string) (map[string]TodoList, error) {
	result := make(map[string]TodoList)

	// Iterate through all blogs for the account
	for _, b := range blog.GetBlogsWithAccount(account) {
		date := getDateFromTitle(b.Title)
		if date != "" {
			var todoList TodoList
			if err := json.Unmarshal([]byte(b.Content), &todoList); err == nil {
				result[date] = todoList
			}
		}
	}

	return result, nil
}

// GetHistoricalTodos retrieves todos for a date range
func (tm *TodoManager) GetHistoricalTodos(account, startDate, endDate string) (map[string]TodoList, error) {
	allTodos, err := tm.GetAllTodos(account)
	if err != nil {
		return nil, err
	}

	result := make(map[string]TodoList)
	for date, todoList := range allTodos {
		if date >= startDate && date <= endDate {
			result[date] = todoList
		}
	}

	return result, nil
}

// ParseTodoListFromBlog parses a blog content string into a TodoList
func ParseTodoListFromBlog(content string) TodoList {
	var todoList TodoList
	if err := json.Unmarshal([]byte(content), &todoList); err != nil {
		// Return empty TodoList if parsing fails
		return TodoList{Items: []TodoItem{}}
	}
	return todoList
}

// saveTodosToBlog saves a TodoList as a blog post
func (tm *TodoManager) saveTodosToBlog(account string, todoList TodoList) error {
	title := generateBlogTitle(todoList.Date)

	// Convert to JSON
	content, err := json.MarshalIndent(todoList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert todo list to JSON: %w", err)
	}

	// Find existing blog or create new one using account-based interface
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		// Create new blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "todolist",
			AuthType: module.EAuthType_private,
			Account:  account,
		}
		blog.AddBlogWithAccount(account, ubd)
	} else {
		// Update existing blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "todolist",
			AuthType: module.EAuthType_private,
			Account:  account,
		}
		blog.ModifyBlogWithAccount(account, ubd)
	}

	return nil
}

// UpdateTodoOrder updates the order of todo items for a specific date
func (tm *TodoManager) UpdateTodoOrder(account, date string, order []string) error {
	// Get todo list for the date
	todoList, err := tm.GetTodosByDate(account, date)
	if err != nil {
		return err
	}

	// Create a map of existing todo IDs for validation
	todoMap := make(map[string]bool)
	for _, item := range todoList.Items {
		todoMap[item.ID] = true
	}

	// Validate order - ensure all IDs in order exist in the todo list
	for _, id := range order {
		if !todoMap[id] {
			return fmt.Errorf("todo ID %s not found in the list", id)
		}
	}

	// Update order
	todoList.Order = order

	// Save to blog
	return tm.saveTodosToBlog(account, todoList)
}

// calculateScore calculates the priority score for a todo item
// urgency: 1-3 (1 most urgent), importance: 1-3 (1 most important)
// returns total score based on urgency, importance, and time
func calculateScore(urgency, importance, hours, minutes int) int {
	// Map urgency/importance levels to scores: 1->5, 2->3, 3->1
	urgencyScore := map[int]int{1: 5, 2: 3, 3: 1}
	importanceScore := map[int]int{1: 5, 2: 3, 3: 1}

	var uScore, iScore int
	if urgency >= 1 && urgency <= 3 {
		uScore = urgencyScore[urgency]
	}
	if importance >= 1 && importance <= 3 {
		iScore = importanceScore[importance]
	}

	// Calculate time score
	totalMinutes := hours*60 + minutes
	var timeScore int
	switch {
	case totalMinutes <= 30:
		timeScore = 5
	case totalMinutes <= 120:
		timeScore = 3
	default:
		timeScore = 1
	}

	return uScore + iScore + timeScore
}
