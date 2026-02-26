package http

import (
	"encoding/json"
	"fmt"
	"mcp"
	h "net/http"
	"view"
)

// HandleAssistant renders the assistant page
func HandleAssistant(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistant", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageAssistant(w)
}

// HandleMCPToolsAPI handles MCP tools API requests
// MCP工具API处理函数
func HandleMCPToolsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMCPToolsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// 获取可用工具列表和服务器状态
		action := r.URL.Query().Get("action")

		switch action {
		case "status":
			// 获取服务器状态
			status := mcp.GetServerStatus()
			response := map[string]interface{}{
				"success": true,
				"status":  status,
			}
			json.NewEncoder(w).Encode(response)
		default:
			// 获取工具列表
			tools := mcp.GetAvailableToolsImproved()
			response := map[string]interface{}{
				"success": true,
				"message": "MCP tools retrieved successfully",
				"data":    tools,
			}
			json.NewEncoder(w).Encode(response)
		}

	case "POST":
		// 测试工具调用
		var toolCall mcp.MCPToolCall
		if err := json.NewDecoder(r.Body).Decode(&toolCall); err != nil {
			response := map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Invalid JSON: %v", err),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		result := mcp.CallToolImproved(toolCall)
		json.NewEncoder(w).Encode(result)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}
