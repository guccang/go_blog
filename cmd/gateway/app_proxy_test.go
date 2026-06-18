package main

import (
	"net/http"
	"testing"
)

func TestGatewayRoutesAppAgentPathsBeforeBlogFallback(t *testing.T) {
	cfg := DefaultConfig()
	mux := http.NewServeMux()
	registerAppAgentProxyRoutes(mux, cfg)
	mux.Handle("/", NewProxy(cfg.GoBackendURL))

	tests := []struct {
		path        string
		wantPattern string
	}{
		{path: "/api/app/butler/tool", wantPattern: "/api/app/"},
		{path: "/api/app/butler/feedback", wantPattern: "/api/app/"},
		{path: "/api/app/butler/affinity", wantPattern: "/api/app/"},
		{path: "/butler/tool", wantPattern: "/butler/"},
		{path: "/butler/feedback", wantPattern: "/butler/"},
		{path: "/butler/affinity", wantPattern: "/butler/"},
		{path: "/ws/app", wantPattern: "/ws/app"},
		{path: "/api/blogs", wantPattern: "/"},
	}

	for _, tt := range tests {
		req, err := http.NewRequest(http.MethodGet, tt.path, nil)
		if err != nil {
			t.Fatalf("new request %s: %v", tt.path, err)
		}
		_, pattern := mux.Handler(req)
		if pattern != tt.wantPattern {
			t.Fatalf("%s pattern = %q, want %q", tt.path, pattern, tt.wantPattern)
		}
	}
}
