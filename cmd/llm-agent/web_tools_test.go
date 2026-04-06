package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestInjectVirtualToolsAddsWebTools(t *testing.T) {
	bridge := &Bridge{
		cfg: &Config{
			Providers: map[string]ProviderConfig{},
		},
	}
	tools := bridge.injectVirtualTools(nil, false)

	foundSearch := false
	foundFetch := false
	for _, tool := range tools {
		switch tool.Function.Name {
		case "WebSearch":
			foundSearch = true
		case "WebFetch":
			foundFetch = true
		}
	}
	if !foundSearch || !foundFetch {
		t.Fatalf("expected WebSearch and WebFetch in virtual tools")
	}
}

func TestBuiltinWebFetchRejectsInvalidURL(t *testing.T) {
	args, _ := json.Marshal(map[string]any{
		"url": "ftp://example.com/file.txt",
	})
	result, err := builtinWebFetch(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !strings.Contains(result.Result, "无效的 URL") {
		t.Fatalf("expected invalid URL result, got %#v", result)
	}
}

func TestWebHTMLToTextStripsMarkup(t *testing.T) {
	got := webHTMLToText(`<html><body><h1>标题</h1><p>Hello <b>world</b></p><script>alert(1)</script></body></html>`)
	if strings.Contains(got, "alert(1)") {
		t.Fatalf("expected scripts stripped, got %q", got)
	}
	if !strings.Contains(got, "标题") || !strings.Contains(got, "Hello world") {
		t.Fatalf("expected text content preserved, got %q", got)
	}
}
