package http

import (
	"blog"
	"encoding/json"
	log "mylog"
	h "net/http"
	"net/url"
	"strings"
)

// HandleBlogFTSSearch returns account-scoped, local SQLite FTS results.
// It is intentionally retrieval-only: no external AI call and no mutation occur here.
func HandleBlogFTSSearch(w h.ResponseWriter, r *h.Request) {
	if r.Method != h.MethodGet {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		h.Error(w, "query is required", h.StatusBadRequest)
		return
	}
	account := getAccountFromRequest(r)
	results, err := blog.SearchFTSWithAccount(account, query, 5)
	if err != nil {
		log.ErrorF(log.ModuleBlog, "FTS search failed: %v", err)
		h.Error(w, "search failed", h.StatusInternalServerError)
		return
	}
	items := make([]map[string]string, 0, len(results))
	for _, result := range results {
		items = append(items, map[string]string{
			"title":   result.Title,
			"snippet": result.Snippet,
			"url":     "/get?blogname=" + url.QueryEscape(result.Title),
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}
