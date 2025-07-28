package mcp

import (
	"auth"
	"config"
	"encoding/json"
	"fmt"
	log "mylog"
	"net/http"
	"view"
)

type MCPResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func checkLogin(r *http.Request) int {
	session, err := r.Cookie("session")
	if err != nil {
		log.ErrorF("not find cookie session err=%s", err.Error())
		return 1
	}

	log.DebugF("checkLogin session=%s", session.Value)
	if auth.CheckLoginSession(session.Value) != 0 {
		return 1
	}
	return 0
}

func HandleMCPPage(w http.ResponseWriter, r *http.Request) {
	log.Debug("HandleMCPPage called")

	if checkLogin(r) != 0 {
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}

	// Get all MCP configurations
	configs := GetAllConfigs()

	// Get available tools for display
	availableTools := GetAvailableToolsImproved()

	// Prepare template data
	data := struct {
		Title          string
		Configs        []MCPConfig
		AvailableTools []MCPTool
		CurrentTime    string
		LLMConfigured  bool
	}{
		Title:          "MCP Assistant - LLM with Tool Calling",
		Configs:        configs,
		AvailableTools: availableTools,
		CurrentTime:    getCurrentTimeString(),
		LLMConfigured:  config.GetConfig("deepseek_api_key") != "",
	}

	view.PageMCP(w, data)
}

func HandleMCPAPI(w http.ResponseWriter, r *http.Request) {
	log.Debug("HandleMCPAPI called")

	if checkLogin(r) != 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		handleMCPGet(w, r)
	case "POST":
		handleMCPPost(w, r)
	case "PUT":
		handleMCPPut(w, r)
	case "DELETE":
		handleMCPDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleMCPGet(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")

	switch action {
	case "list":
		configs := GetAllConfigs()
		response := MCPResponse{
			Success: true,
			Message: "MCP configurations retrieved successfully",
			Data:    configs,
		}
		json.NewEncoder(w).Encode(response)

	case "get":
		name := r.URL.Query().Get("name")
		if name == "" {
			response := MCPResponse{
				Success: false,
				Message: "Name parameter is required",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		config, found := GetConfig(name)
		if !found {
			response := MCPResponse{
				Success: false,
				Message: fmt.Sprintf("MCP config '%s' not found", name),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		response := MCPResponse{
			Success: true,
			Message: "MCP configuration retrieved successfully",
			Data:    config,
		}
		json.NewEncoder(w).Encode(response)

	default:
		response := MCPResponse{
			Success: false,
			Message: "Invalid action",
		}
		json.NewEncoder(w).Encode(response)
	}
}

func handleMCPPost(w http.ResponseWriter, r *http.Request) {
	var config MCPConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		response := MCPResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid JSON: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if err := ValidateConfig(config); err != nil {
		response := MCPResponse{
			Success: false,
			Message: fmt.Sprintf("Validation error: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if err := AddConfig(config); err != nil {
		response := MCPResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to add config: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := MCPResponse{
		Success: true,
		Message: "MCP configuration added successfully",
		Data:    config,
	}
	json.NewEncoder(w).Encode(response)
}

func handleMCPPut(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	action := r.URL.Query().Get("action")

	if action == "toggle" {
		if name == "" {
			response := MCPResponse{
				Success: false,
				Message: "Name parameter is required",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if err := ToggleConfig(name); err != nil {
			response := MCPResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to toggle config: %v", err),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		config, _ := GetConfig(name)
		response := MCPResponse{
			Success: true,
			Message: "MCP configuration toggled successfully",
			Data:    config,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if name == "" {
		response := MCPResponse{
			Success: false,
			Message: "Name parameter is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	var config MCPConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		response := MCPResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid JSON: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if err := ValidateConfig(config); err != nil {
		response := MCPResponse{
			Success: false,
			Message: fmt.Sprintf("Validation error: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if err := UpdateConfig(name, config); err != nil {
		response := MCPResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update config: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := MCPResponse{
		Success: true,
		Message: "MCP configuration updated successfully",
		Data:    config,
	}
	json.NewEncoder(w).Encode(response)
}

func handleMCPDelete(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		response := MCPResponse{
			Success: false,
			Message: "Name parameter is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if err := DeleteConfig(name); err != nil {
		response := MCPResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete config: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := MCPResponse{
		Success: true,
		Message: "MCP configuration deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

func getCurrentTimeString() string {
	return fmt.Sprintf("%d", GetEnabledConfigsCount())
}

// GetAvailableToolsHandler handles requests for available tools
func GetAvailableToolsHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("GetAvailableToolsHandler called")

	if checkLogin(r) != 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	tools := GetAvailableToolsImproved()
	response := MCPResponse{
		Success: true,
		Message: "Available tools retrieved successfully",
		Data:    tools,
	}
	json.NewEncoder(w).Encode(response)
}

func GetEnabledConfigsCount() int {
	count := 0
	for _, config := range GetAllConfigs() {
		if config.Enabled {
			count++
		}
	}
	return count
}
