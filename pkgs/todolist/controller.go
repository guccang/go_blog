package todolist

import (
    "net/http"
    "encoding/json"
    "time"
    "auth"
)

// Controller handles HTTP requests for todo list operations
type Controller struct {
    manager *TodoManager
}

// NewController creates a new todo list controller
func NewController(manager *TodoManager) *Controller {
    return &Controller{
        manager: manager,
    }
}

// getsession extracts session value from request cookie
func getsession(r *http.Request) string {
    session, err := r.Cookie("session")
    if err != nil {
        return ""
    }
    return session.Value
}

// getAccountFromRequest extracts account by resolving the session cookie
func getAccountFromRequest(r *http.Request) string {
    s := getsession(r)
    if s == "" {
        return ""
    }
    return auth.GetAccountBySession(s)
}

// GetManager returns the todo manager instance
func (c *Controller) GetManager() *TodoManager {
    return c.manager
}

// HandleGetTodos handles GET request to retrieve todos
func (c *Controller) HandleGetTodos(w http.ResponseWriter, r *http.Request) {
    // Set headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    account := getAccountFromRequest(r)
    date := r.URL.Query().Get("date")
    if date == "" {
        date = time.Now().Format("2006-01-02")
    }

    todoList, err := c.manager.GetTodosByDate(account, date)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to get todos: " + err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(todoList)
}

// HandleAddTodo handles POST request to add a new todo
func (c *Controller) HandleAddTodo(w http.ResponseWriter, r *http.Request) {
    // Set headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    // Check content type
    contentType := r.Header.Get("Content-Type")
    if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
        w.WriteHeader(http.StatusUnsupportedMediaType)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Content-Type must be application/json",
        })
        return
    }
    
    var request struct {
        Content string `json:"content"`
        Date    string `json:"date"`
        Hours   int    `json:"hours"`
        Minutes int    `json:"minutes"`
    }

    // Parse request body
    decoder := json.NewDecoder(r.Body)
    if err := decoder.Decode(&request); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid JSON: " + err.Error(),
        })
        return
    }
    defer r.Body.Close()

    if request.Content == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Content is required",
        })
        return
    }
    
    account := getAccountFromRequest(r)
    // Use provided date or default to today
    date := request.Date
    if date == "" {
        date = time.Now().Format("2006-01-02")
    }

    todo, err := c.manager.AddTodo(account, date, request.Content, request.Hours, request.Minutes)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to add todo: " + err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(todo)
}

// HandleDeleteTodo handles DELETE request to remove a todo
func (c *Controller) HandleDeleteTodo(w http.ResponseWriter, r *http.Request) {
    // Set headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    id := r.URL.Query().Get("id")
    if id == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "ID is required",
        })
        return
    }
    
    account := getAccountFromRequest(r)
    date := r.URL.Query().Get("date")
    if date == "" {
        date = time.Now().Format("2006-01-02")
    }

    if err := c.manager.DeleteTodo(account, date, id); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to delete todo: " + err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
        "message": "Todo deleted successfully",
    })
}

// HandleToggleTodo handles PUT request to toggle todo completion
func (c *Controller) HandleToggleTodo(w http.ResponseWriter, r *http.Request) {
    // Set headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    id := r.URL.Query().Get("id")
    if id == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "ID is required",
        })
        return
    }
    
    account := getAccountFromRequest(r)
    date := r.URL.Query().Get("date")
    if date == "" {
        date = time.Now().Format("2006-01-02")
    }

    if err := c.manager.ToggleTodo(account, date, id); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to toggle todo: " + err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
        "message": "Todo status toggled successfully",
    })
}

// HandleUpdateTodoTime handles PUT request to update todo time
func (c *Controller) HandleUpdateTodoTime(w http.ResponseWriter, r *http.Request) {
    // Set headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    // Check content type
    contentType := r.Header.Get("Content-Type")
    if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
        w.WriteHeader(http.StatusUnsupportedMediaType)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Content-Type must be application/json",
        })
        return
    }
    
    id := r.URL.Query().Get("id")
    if id == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "ID is required",
        })
        return
    }
    
    account := getAccountFromRequest(r)
    date := r.URL.Query().Get("date")
    if date == "" {
        date = time.Now().Format("2006-01-02")
    }
    
    var request struct {
        Hours   int `json:"hours"`
        Minutes int `json:"minutes"`
    }
    
    // Parse request body
    decoder := json.NewDecoder(r.Body)
    if err := decoder.Decode(&request); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid JSON: " + err.Error(),
        })
        return
    }
    defer r.Body.Close()
    
    // Validate time values
    if request.Hours < 0 || request.Hours > 24 {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Hours must be between 0 and 24",
        })
        return
    }
    
    if request.Minutes < 0 || request.Minutes > 59 {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Minutes must be between 0 and 59",
        })
        return
    }
    
    if err := c.manager.UpdateTodoTime(account, date, id, request.Hours, request.Minutes); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to update todo time: " + err.Error(),
        })
        return
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
        "message": "Todo time updated successfully",
    })
}

// HandleGetHistoricalTodos handles GET request to retrieve historical todos
func (c *Controller) HandleGetHistoricalTodos(w http.ResponseWriter, r *http.Request) {
    // Set headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    account := getAccountFromRequest(r)
    startDate := r.URL.Query().Get("start_date")
    endDate := r.URL.Query().Get("end_date")

    if startDate == "" || endDate == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Start date and end date are required",
        })
        return
    }

    historicalTodos, err := c.manager.GetHistoricalTodos(account, startDate, endDate)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to get historical todos: " + err.Error(),
        })
        return
    }
    
    // Convert the map of TodoLists to a map of []TodoItem to maintain API compatibility
    result := make(map[string][]TodoItem)
    for date, todoList := range historicalTodos {
        result[date] = todoList.Items
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}

// TodoOrder represents the order of todos for a specific date
type TodoOrder struct {
    Date string   `json:"date"`
    Order []string `json:"order"` // Array of todo IDs in the desired order
}

// HandleUpdateTodoOrder handles PUT request to update todo order
func (c *Controller) HandleUpdateTodoOrder(w http.ResponseWriter, r *http.Request) {
    // Set headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    // Check content type
    contentType := r.Header.Get("Content-Type")
    if contentType != "application/json" && contentType != "application/json; charset=UTF-8" {
        w.WriteHeader(http.StatusUnsupportedMediaType)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Content-Type must be application/json",
        })
        return
    }
    
    var request TodoOrder
    
    // Parse request body
    decoder := json.NewDecoder(r.Body)
    if err := decoder.Decode(&request); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid JSON: " + err.Error(),
        })
        return
    }
    defer r.Body.Close()
    
    if request.Date == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Date is required",
        })
        return
    }
    
    if len(request.Order) == 0 {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Order array cannot be empty",
        })
        return
    }
    
    account := getAccountFromRequest(r)
    if err := c.manager.UpdateTodoOrder(account, request.Date, request.Order); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Failed to update todo order: " + err.Error(),
        })
        return
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
        "message": "Todo order updated successfully",
    })
} 