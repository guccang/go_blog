package http

import (
	"encoding/json"
	"fmt"
	"mcp"
	h "net/http"
	"time"
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

	// 生成请求 ID 用于 delegation token 上下文
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	// 检查是否有 delegation token
	delegationTokenHeader := r.Header.Get("X-Delegation-Token")
	if delegationTokenHeader != "" {
		// 解析并验证 delegation token
		token, err := mcp.ParseDelegationTokenFromHeader(delegationTokenHeader)
		if err != nil {
			response := map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Invalid delegation token: %v", err),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		// 设置到上下文
		mcp.SetDelegationToken(requestID, token)
		defer mcp.ClearDelegationToken(requestID)
	} else {
		// 没有 delegation token，检查 session cookie
		if checkLogin(r) != 0 {
			h.Error(w, "Unauthorized", h.StatusUnauthorized)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// 获取可用工具列表
		tools := mcp.GetAvailableTools()
		response := map[string]interface{}{
			"success": true,
			"message": "MCP tools retrieved successfully",
			"data":    tools,
		}
		json.NewEncoder(w).Encode(response)

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

		result := mcp.CallToolForAPI(toolCall, requestID)
		json.NewEncoder(w).Encode(result)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}
