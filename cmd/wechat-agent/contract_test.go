package main

import (
	"strings"
	"testing"
)

func TestNewBridgeRegistersWechatTools(t *testing.T) {
	cfg := DefaultConfig()
	bridge := NewBridge(cfg)
	if bridge == nil {
		t.Fatalf("expected bridge to be created")
	}
	if len(bridge.client.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(bridge.client.Tools))
	}
	if bridge.client.Tools[0].Name != "wechat.SendMessage" {
		t.Fatalf("unexpected first tool: %s", bridge.client.Tools[0].Name)
	}
	if bridge.client.Tools[1].Name != "wechat.SendMarkdown" {
		t.Fatalf("unexpected second tool: %s", bridge.client.Tools[1].Name)
	}
	if got := bridge.client.Meta["http_port"]; got != cfg.HTTPPort {
		t.Fatalf("expected http_port=%d, got %#v", cfg.HTTPPort, got)
	}
}

func TestGetHelpTextMentionsCommonCommands(t *testing.T) {
	help := getHelpText()
	if !strings.Contains(help, "/help") {
		t.Fatalf("expected help to mention /help")
	}
	if !strings.Contains(help, "/status") {
		t.Fatalf("expected help to mention /status")
	}
}
