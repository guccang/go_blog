package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewProxy 创建 HTTP 反向代理到 go_blog 后端
func NewProxy(backendURL string) http.Handler {
	target, err := url.Parse(backendURL)
	if err != nil {
		log.Fatalf("[Proxy] invalid backend URL %s: %v", backendURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// 自定义 Director：保留原始 Host
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy] backend error: %v (url=%s)", err, r.URL.Path)
		http.Error(w, "Backend unavailable", http.StatusBadGateway)
	}

	log.Printf("[Proxy] reverse proxy to %s", backendURL)
	return proxy
}
