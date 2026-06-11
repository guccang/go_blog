package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// butlerAllowedTools 管家面板允许经代理调用的 blog-agent 工具（记忆 + 目标）。
var butlerAllowedTools = map[string]struct{}{
	"RawMemoryRead":       {},
	"RawMemoryWrite":      {},
	"RawMemoryAppend":     {},
	"RawMemoryJournal":    {},
	"RawMemorySearch":     {},
	"RawMemoryRewriteSection": {},
	"RawMemoryListFiles":  {},
	"RawGetCurrentGoals":  {},
	"RawGetDailyGoal":     {},
	"RawGetWeeklyGoal":    {},
	"RawGetMonthlyGoal":   {},
	"RawGetYearlyGoal":    {},
	"RawListGoalsByLevel": {},
	"RawSaveGoal":         {},
	"RawAddGoalTask":      {},
	"RawUpdateGoalTask":   {},
	"RawDeleteGoalTask":   {},
}

type butlerToolRequest struct {
	UserID    string         `json:"user_id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// HandleButlerTool 代理 Flutter 管家面板的记忆/目标工具调用到 blog-agent。
// POST /api/app/butler/tool {user_id, name, arguments}
func (h *Handler) HandleButlerTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req butlerToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" || !h.validateAppSession(r, req.UserID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}
	toolName := strings.TrimSpace(req.Name)
	if _, ok := butlerAllowedTools[toolName]; !ok {
		writeButlerError(w, http.StatusBadRequest, fmt.Sprintf("tool not allowed: %s", toolName))
		return
	}

	arguments := req.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	arguments["account"] = req.UserID

	result, status, err := h.callBlogAgentTool(req.UserID, toolName, arguments)
	if err != nil {
		writeButlerError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(result)
}

func (h *Handler) callBlogAgentTool(userID, toolName string, arguments map[string]any) ([]byte, int, error) {
	base := strings.TrimRight(strings.TrimSpace(h.cfg.BlogAgentBaseURL), "/")
	if base == "" {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("blog-agent base url is not configured")
	}
	payload, err := json.Marshal(map[string]any{
		"name":      toolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, base+"/api/mcp/tools", bytes.NewReader(payload))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token := h.bridge.GetDelegationToken(userID); token != "" {
		httpReq.Header.Set("X-Delegation-Token", token)
	}
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("blog-agent unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, http.StatusBadGateway,
			fmt.Errorf("blog-agent returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, http.StatusOK, nil
}

func writeButlerError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": message})
}
