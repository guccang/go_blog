package http

import (
	"blog"
	"encoding/json"
	"module"
	h "net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	control "service"
)

const largeBlogChunkSize = 128 * 1024

// HandleBlogContentChunk returns a bounded range of a large blog so the page
// does not need to embed and render a multi-megabyte document at once.
func HandleBlogContentChunk(w h.ResponseWriter, r *h.Request) {
	if r.Method != h.MethodGet {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	title := r.URL.Query().Get("blogname")
	if title == "" {
		h.Error(w, "blogname is required", h.StatusBadRequest)
		return
	}
	account := r.URL.Query().Get("account")
	requestAccount := getAccountFromRequest(r)
	if account == "" {
		account = requestAccount
	}
	if account == "" {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	blogObj := control.GetBlog(account, title)
	if blogObj == nil {
		h.Error(w, "blog not found", h.StatusNotFound)
		return
	}
	if requestAccount != account && (blogObj.AuthType&module.EAuthType_public) == 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	if r.URL.Query().Get("offset") == "" || r.URL.Query().Get("offset") == "0" {
		emitUsageHook(r, account, blog.HookFeatureUsed, "large_blog_reader", "blog", title, title, "", nil, map[string]any{"status": "started"})
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 || offset > len(blogObj.Content) {
		h.Error(w, "invalid offset", h.StatusBadRequest)
		return
	}
	end := offset + largeBlogChunkSize
	if end > len(blogObj.Content) {
		end = len(blogObj.Content)
	}
	if end < len(blogObj.Content) {
		if lineEnd := strings.LastIndex(blogObj.Content[offset:end], "\n"); lineEnd > 0 {
			end = offset + lineEnd + 1
		}
		for end > offset && !utf8.RuneStart(blogObj.Content[end]) {
			end--
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"content":     blogObj.Content[offset:end],
		"next_offset": end,
		"has_more":    end < len(blogObj.Content),
		"total_bytes": len(blogObj.Content),
	})
}
