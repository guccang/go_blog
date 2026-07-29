package http

import (
	"blog"
	"encoding/json"
	t "html/template"
	h "net/http"
	"persistence"
	"strconv"
	"strings"
)

func HandleHookInsights(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleHookInsights", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	tmpl, err := t.ParseFiles(GetTemplatePath("hook_insights.template"))
	if err != nil {
		h.Error(w, "Failed to parse hook insights template", h.StatusInternalServerError)
		return
	}
	emitUsageHook(r, getAccountFromRequest(r), blog.HookPageOpened, "hook_insights", "page", "insights", "使用洞察", "", nil, map[string]any{"status": "success"})
	if err := tmpl.Execute(w, nil); err != nil {
		h.Error(w, "Failed to render hook insights template", h.StatusInternalServerError)
	}
}

func HandleHookInsightsAPI(w h.ResponseWriter, r *h.Request) {
	if r.Method != h.MethodGet {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	filter := persistence.HookInsightFilter{
		Feature:   strings.TrimSpace(r.URL.Query().Get("feature")),
		EventType: strings.TrimSpace(r.URL.Query().Get("event")),
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
	}
	if filter.Status != "" && filter.Status != "success" && filter.Status != "error" && filter.Status != "unknown" {
		h.Error(w, "invalid status", h.StatusBadRequest)
		return
	}
	insights, err := persistence.GetHookInsights(getAccountFromRequest(r), days, filter)
	if err != nil {
		h.Error(w, "load hook insights failed", h.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(insights)
}
