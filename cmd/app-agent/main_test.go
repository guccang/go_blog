package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHTTPHandlerRegistersButlerRoutes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ButlerAffinityFile = filepath.Join(t.TempDir(), "butler-affinity.json")
	bridge := NewBridge(cfg)
	handler := &Handler{
		cfg:      cfg,
		bridge:   bridge,
		auth:     newAuthManager(cfg),
		affinity: NewButlerAffinityStore(cfg.ButlerAffinityFile),
	}
	router := newHTTPHandler(handler, bridge)

	tests := []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{
			name:   "tool",
			method: http.MethodPost,
			target: "/api/app/butler/tool",
			body:   []byte(`{}`),
		},
		{
			name:   "feedback",
			method: http.MethodPost,
			target: "/api/app/butler/feedback",
			body:   []byte(`{}`),
		},
		{
			name:   "affinity",
			method: http.MethodGet,
			target: "/api/app/butler/affinity",
		},
		{
			name:   "tool alias",
			method: http.MethodPost,
			target: "/butler/tool",
			body:   []byte(`{}`),
		},
		{
			name:   "feedback alias",
			method: http.MethodPost,
			target: "/butler/feedback",
			body:   []byte(`{}`),
		},
		{
			name:   "affinity alias",
			method: http.MethodGet,
			target: "/butler/affinity",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.target, bytes.NewReader(test.body))
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("%s route is not registered", test.target)
			}
		})
	}
}
