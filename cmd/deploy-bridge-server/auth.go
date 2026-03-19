package main

import (
	"net/http"
	"strings"
)

// authMiddleware Token 认证中间件
// 支持两种方式：Authorization: Bearer <token> 或 ?token=xxx
func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从 Authorization header 提取
		if auth := r.Header.Get("Authorization"); auth != "" {
			if strings.HasPrefix(auth, "Bearer ") {
				if strings.TrimPrefix(auth, "Bearer ") == token {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// 从 query param 提取
		if r.URL.Query().Get("token") == token {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})
}
