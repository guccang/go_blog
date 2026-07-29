package http

import (
	"blog"
	"encoding/json"
	log "mylog"
	h "net/http"
	"net/url"
	"persistence"
	"strconv"
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
	limit := 5
	offset := 0
	all := r.URL.Query().Get("all") == "1"
	if all {
		limit = 0
	} else if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}
	if rawOffset := r.URL.Query().Get("offset"); rawOffset != "" {
		if parsed, parseErr := strconv.Atoi(rawOffset); parseErr == nil && parsed >= 0 {
			offset = parsed
		}
	}
	searchLimit := limit
	if limit > 0 {
		searchLimit++ // Fetch one extra row to decide whether to show "view all".
	}
	account := getAccountFromRequest(r)
	var results []persistence.BlogSearchResult
	var err error
	if all {
		results, err = blog.SearchFTSWithAccount(account, query, 0)
	} else {
		results, err = blog.SearchFTSPageWithAccount(account, query, searchLimit, offset)
	}
	if err != nil {
		log.ErrorF(log.ModuleBlog, "FTS search failed: %v", err)
		h.Error(w, "search failed", h.StatusInternalServerError)
		return
	}
	hasMore := limit > 0 && len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	emitUsageHook(r, account, blog.HookFeatureUsed, "fts_search", "search_query", "", "", query, map[string]any{"all": all, "offset": offset}, map[string]any{"result_count": len(results), "has_more": hasMore})
	items := make([]map[string]string, 0, len(results))
	for _, result := range results {
		items = append(items, map[string]string{
			"title":   result.Title,
			"snippet": result.Snippet,
			"url":     "/get?blogname=" + url.QueryEscape(result.Title),
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	nextOffset := offset + len(items)
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "has_more": hasMore, "next_offset": nextOffset})
}
