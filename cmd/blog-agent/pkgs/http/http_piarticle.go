package http

import (
	"blog"
	"config"
	"encoding/json"
	log "mylog"
	h "net/http"
	"persistence"
	"piagent"
	"strings"
)

// HandlePIArticle handles explicit reading-assistant actions for the current article.
func HandlePIArticle(w h.ResponseWriter, r *h.Request) {
	if r.Method != h.MethodPost {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	var request struct {
		Title    string `json:"title"`
		Action   string `json:"action"`
		Question string `json:"question"`
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(h.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
		h.Error(w, "invalid JSON", h.StatusBadRequest)
		return
	}
	request.Title = strings.TrimSpace(request.Title)
	request.Action = strings.TrimSpace(request.Action)
	request.Question = strings.TrimSpace(request.Question)
	if request.Title == "" || request.Action == "" {
		h.Error(w, "title and action are required", h.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	migratePIProvidersToJSON(account)
	config.ReloadConfigFromSQLite(account)
	emitUsageHook(r, account, blog.HookAIAsked, "article_assistant", "blog", request.Title, request.Title, request.Question, map[string]any{"action": request.Action, "provider": request.Provider}, nil)
	answer, err := piagent.AskArticle(account, request.Title, request.Action, request.Question, request.Provider)
	if err != nil {
		log.ErrorF(log.ModuleBlog, "PI article request failed account=%s title=%s: %v", account, request.Title, err)
		emitUsageHook(r, account, blog.HookAIAnswered, "article_assistant", "blog", request.Title, request.Title, request.Question, map[string]any{"action": request.Action}, map[string]any{"status": "error"})
		h.Error(w, err.Error(), h.StatusBadGateway)
		return
	}
	if err := persistence.RecordPIUsage(account, answer.Provider, answer.Model, answer.Usage.PromptTokens, answer.Usage.CompletionTokens, answer.Usage.TotalTokens, answer.DurationMs, "article_assistant"); err != nil {
		log.ErrorF(log.ModuleBlog, "record PI article usage failed account=%s: %v", account, err)
	}
	emitUsageHook(r, account, blog.HookAIAnswered, "article_assistant", "blog", request.Title, request.Title, request.Question, map[string]any{"action": request.Action, "provider": answer.Provider, "model": answer.Model}, map[string]any{"status": "success", "source_count": len(answer.Sources)})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(answer)
}
