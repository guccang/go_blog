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

// HandlePIAsk answers a question with local FTS retrieval followed by one configured provider call.
func HandlePIAsk(w h.ResponseWriter, r *h.Request) {
	if r.Method != h.MethodPost {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	var request struct {
		Question string `json:"question"`
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(h.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
		h.Error(w, "invalid JSON", h.StatusBadRequest)
		return
	}
	request.Question = strings.TrimSpace(request.Question)
	if request.Question == "" {
		h.Error(w, "question is required", h.StatusBadRequest)
		return
	}
	account := getAccountFromRequest(r)
	migratePIProvidersToJSON(account)
	config.ReloadConfigFromSQLite(account)
	emitUsageHook(r, account, blog.HookAIAsked, "pi_agent", "question", "", "", request.Question, map[string]any{"provider": request.Provider}, nil)
	answer, err := piagent.Ask(account, request.Question, request.Provider)
	if err != nil {
		log.ErrorF(log.ModuleBlog, "PI Agent request failed account=%s: %v", account, err)
		emitUsageHook(r, account, blog.HookAIAnswered, "pi_agent", "question", "", "", request.Question, nil, map[string]any{"status": "error"})
		h.Error(w, err.Error(), h.StatusBadGateway)
		return
	}
	if err := persistence.RecordPIUsage(account, answer.Provider, answer.Model, answer.Usage.PromptTokens, answer.Usage.CompletionTokens, answer.Usage.TotalTokens, answer.DurationMs, "success"); err != nil {
		log.ErrorF(log.ModuleBlog, "record PI usage failed account=%s: %v", account, err)
	}
	emitUsageHook(r, account, blog.HookAIAnswered, "pi_agent", "question", "", "", request.Question, map[string]any{"provider": answer.Provider, "model": answer.Model}, map[string]any{"status": "success", "source_count": len(answer.Sources)})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(answer)
}

func HandlePIUsage(w h.ResponseWriter, r *h.Request) {
	if r.Method != h.MethodGet {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	stats, records, err := persistence.GetPIUsage(getAccountFromRequest(r), 30)
	if err != nil {
		h.Error(w, "load PI usage failed", h.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"stats": stats, "records": records})
}
