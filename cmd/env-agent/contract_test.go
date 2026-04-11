package main

import "testing"

func TestNewConnectionRegistersEnvTools(t *testing.T) {
	cfg := DefaultConfig()
	conn := NewConnection(cfg, "env-agent-test")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	if conn.AgentType != "env_agent" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if len(conn.Client.Tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(conn.Client.Tools))
	}
	if conn.Client.Tools[0].Name != "EnvCheck" {
		t.Fatalf("expected first tool EnvCheck, got %s", conn.Client.Tools[0].Name)
	}
	if conn.Client.Tools[3].Name != "EnvSetup" {
		t.Fatalf("expected last tool EnvSetup, got %s", conn.Client.Tools[3].Name)
	}
}
