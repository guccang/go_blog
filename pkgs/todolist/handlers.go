package todolist

import (
    "net/http"
    "encoding/json"
    "html/template"
	log "mylog"
)

var controller *Controller

// SetController sets the controller instance for the handlers
func SetController(c *Controller) {
    controller = c
}

// HandleTodoList handles the todolist page request
func HandleTodoList(w http.ResponseWriter, r *http.Request) {
	log.DebugF("HandleTodoList %s", r.Method)
    
    // 检查用户是否已登录
    session, err := r.Cookie("session")
    if err != nil || session.Value == "" {
        // 未登录，重定向到登录页面
        http.Redirect(w, r, "/index", http.StatusFound)
        return
    }
    
    if r.Method != http.MethodGet {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
        return
    }

    // Parse template
    tmpl, err := template.ParseFiles("templates/todolist.template")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Execute template
    if err := tmpl.Execute(w, nil); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

// HandleTodos handles GET, POST, and DELETE requests for todos
func HandleTodos(w http.ResponseWriter, r *http.Request) {
    // Set CORS headers for all responses
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
    
    // Handle CORS preflight request
    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusOK)
        return
    }

    log.DebugF("HandleTodos %s", r.Method)

    // 检查用户是否已登录
    session, err := r.Cookie("session")
    if err != nil || session.Value == "" {
        // 未登录，返回错误
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
        return
    }

    // Check if controller is initialized
    if controller == nil {
        log.ErrorF("HandleTodos: controller is nil")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
        return
    }

    switch r.Method {
    case http.MethodGet:
        controller.HandleGetTodos(w, r)
    case http.MethodPost:
        controller.HandleAddTodo(w, r)
    case http.MethodDelete:
        controller.HandleDeleteTodo(w, r)
    default:
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
    }
}

// HandleToggleTodo handles PUT request to toggle todo completion
func HandleToggleTodo(w http.ResponseWriter, r *http.Request) {
    // Set CORS headers for all responses
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "PUT, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
    
    // Handle CORS preflight request
    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusOK)
        return
    }

    if r.Method != http.MethodPut {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
        return
    }

    // Check if controller is initialized
    if controller == nil {
        log.ErrorF("HandleToggleTodo: controller is nil")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
        return
    }

    controller.HandleToggleTodo(w, r)
}

// HandleUpdateTodoTime handles PUT request to update todo time
func HandleUpdateTodoTime(w http.ResponseWriter, r *http.Request) {
    // Set CORS headers for all responses
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "PUT, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
    
    // Handle CORS preflight request
    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusOK)
        return
    }

    if r.Method != http.MethodPut {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
        return
    }

    // Check if controller is initialized
    if controller == nil {
        log.ErrorF("HandleUpdateTodoTime: controller is nil")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
        return
    }

    controller.HandleUpdateTodoTime(w, r)
}

// HandleHistoricalTodos handles GET request to retrieve historical todos
func HandleHistoricalTodos(w http.ResponseWriter, r *http.Request) {
    // Set CORS headers for all responses
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
    
    // Handle CORS preflight request
    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusOK)
        return
    }

    if r.Method != http.MethodGet {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
        return
    }

    // Check if controller is initialized
    if controller == nil {
        log.ErrorF("HandleHistoricalTodos: controller is nil")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
        return
    }

    controller.HandleGetHistoricalTodos(w, r)
}

// HandleUpdateTodoOrder handles PUT request to update todo order
func HandleUpdateTodoOrder(w http.ResponseWriter, r *http.Request) {
    // Set CORS headers for all responses
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "PUT, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
    
    // Handle CORS preflight request
    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusOK)
        return
    }

    if r.Method != http.MethodPut {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
        return
    }

    // Check if controller is initialized
    if controller == nil {
        log.ErrorF("HandleUpdateTodoOrder: controller is nil")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": "Service not initialized"})
        return
    }

    controller.HandleUpdateTodoOrder(w, r)
} 