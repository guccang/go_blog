package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"uap"
)

func TestHandleGatewayLifecycleNotifyRefreshesCatalogImmediately(t *testing.T) {
	toolPayload := map[string]any{
		"success": true,
		"tools": []map[string]any{
			{
				"agent_id":    "mcp_bridge_1",
				"name":        "mcp.maps_weather",
				"description": "天气查询",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	agentPayload := map[string]any{
		"success": true,
		"agents": []map[string]any{
			{
				"agent_id":    "mcp_bridge_1",
				"agent_type":  "mcp_bridge",
				"name":        "mcp-agent",
				"description": "高德地图服务：天气查询 (1 工具)",
				"tools":       []string{"mcp.maps_weather"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/gateway/tools":
			_ = json.NewEncoder(w).Encode(toolPayload)
		case "/api/gateway/agents":
			_ = json.NewEncoder(w).Encode(agentPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.GatewayHTTP = server.URL
	bridge := NewBridge(cfg)

	ok := bridge.handleGatewayLifecycleNotify(&uap.Message{
		Type:    uap.MsgNotify,
		From:    "gateway",
		Payload: json.RawMessage(`{"event":"agent_online","agent_id":"mcp_bridge_1","agent_type":"mcp_bridge","agent_name":"mcp-agent"}`),
	})
	if !ok {
		t.Fatalf("expected lifecycle notify to be handled")
	}

	bridge.catalogMu.RLock()
	defer bridge.catalogMu.RUnlock()

	if got := bridge.toolCatalog["mcp.maps_weather"]; got != "mcp_bridge_1" {
		t.Fatalf("unexpected tool catalog mapping: %q", got)
	}
	if len(bridge.agentTools["mcp_bridge_1"]) != 1 {
		t.Fatalf("expected 1 agent tool, got %d", len(bridge.agentTools["mcp_bridge_1"]))
	}
	if got := bridge.agentInfo["mcp_bridge_1"].Description; got != "高德地图服务：天气查询 (1 工具)" {
		t.Fatalf("unexpected agent description: %q", got)
	}
}
