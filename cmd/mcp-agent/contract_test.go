package main

import "testing"

func TestDiffServersDetectsAddedRemovedChanged(t *testing.T) {
	oldServers := map[string]MCPServerConfig{
		"a": {Transport: "stdio", Command: "tool-a"},
		"b": {Transport: "http", URL: "http://old"},
	}
	newServers := map[string]MCPServerConfig{
		"b": {Transport: "http", URL: "http://new"},
		"c": {Transport: "stdio", Command: "tool-c"},
	}

	added, removed, changed := DiffServers(oldServers, newServers)
	if len(added) != 1 || added[0] != "c" {
		t.Fatalf("unexpected added servers: %v", added)
	}
	if len(removed) != 1 || removed[0] != "a" {
		t.Fatalf("unexpected removed servers: %v", removed)
	}
	if len(changed) != 1 || changed[0] != "b" {
		t.Fatalf("unexpected changed servers: %v", changed)
	}
}

func TestNewConnectionUsesMCPBridgeAgentType(t *testing.T) {
	cfg := DefaultConfig()
	manager := NewMCPManager(cfg.ToolPrefix)
	conn := NewConnection(cfg, "mcp-agent-test", manager, "mcp-agent.json")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	if conn.AgentType != "mcp_bridge" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if conn.Client.Tools != nil {
		t.Fatalf("expected tools to be loaded after manager discovery, got %d", len(conn.Client.Tools))
	}
}
