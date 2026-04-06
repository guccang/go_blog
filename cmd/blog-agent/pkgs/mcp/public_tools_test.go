package mcp

import "testing"

func TestGetInnerMCPToolsProcessedOnlyReturnsPublicTools(t *testing.T) {
	tools := GetInnerMCPToolsProcessed()
	if len(tools) == 0 {
		t.Fatal("expected public tools")
	}
	for _, tool := range tools {
		if !isPublicToolName(tool.Function.Name) {
			t.Fatalf("unexpected non-public tool exposed: %s", tool.Function.Name)
		}
	}
}

func TestCallToolForAPIRejectsHiddenTools(t *testing.T) {
	result := CallToolForAPI(MCPToolCall{
		Name: "WebSearch",
		Arguments: map[string]interface{}{
			"query": "golang",
		},
	}, "test-request")
	if result.Success {
		t.Fatalf("expected hidden tool to be rejected")
	}
	if result.Error == "" {
		t.Fatalf("expected error for hidden tool")
	}
}
