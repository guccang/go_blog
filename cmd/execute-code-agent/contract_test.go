package main

import "testing"

func TestNewConnectionRegistersExecuteCodeTools(t *testing.T) {
	cfg := DefaultConfig()
	conn := NewConnection(cfg, "execute-code-test", "3.11.0")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	if conn.AgentType != "execute_code" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if len(conn.Client.Tools) == 0 {
		t.Fatalf("expected execute-code tools to be registered")
	}
	if conn.Client.Tools[0].Name != "ExecuteCode" {
		t.Fatalf("expected first tool ExecuteCode, got %s", conn.Client.Tools[0].Name)
	}
}
