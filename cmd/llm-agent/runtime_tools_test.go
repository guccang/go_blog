package main

import "testing"

func testTool(name string) LLMTool {
	return LLMTool{
		Type: "function",
		Function: LLMFunction{
			Name:        name,
			Description: name,
		},
	}
}

func TestBuildSubTaskToolRuntimeViewKeepsExecuteCode(t *testing.T) {
	bridge := &Bridge{}
	tools := []LLMTool{
		testTool("plan_and_execute"),
		testTool("ExecuteCode"),
		testTool("RawGetTodosByDate"),
	}

	view := bridge.buildSubTaskToolRuntimeView(tools, nil)
	if len(view.VisibleTools) != 2 {
		t.Fatalf("expected 2 visible tools, got %d", len(view.VisibleTools))
	}
	foundExecuteCode := false
	for _, tool := range view.VisibleTools {
		if tool.Function.Name == "ExecuteCode" {
			foundExecuteCode = true
		}
		if tool.Function.Name == "plan_and_execute" {
			t.Fatalf("plan_and_execute should be hidden in subtask view")
		}
	}
	if !foundExecuteCode {
		t.Fatalf("expected ExecuteCode to remain visible")
	}
}

func TestExpandSiblingToolsInViewAddsDiscoveredTools(t *testing.T) {
	bridge := &Bridge{
		toolCatalog: map[string]string{
			"ToolA": "agent-a",
		},
		agentTools: map[string][]LLMTool{
			"agent-a": {testTool("ToolA"), testTool("ToolB")},
		},
	}

	view := newToolRuntimeView([]LLMTool{testTool("ToolA")}, []LLMTool{testTool("ToolA")})
	added := bridge.expandSiblingToolsInView(view, []string{"ToolA"})
	if len(added) != 1 || added[0] != "ToolB" {
		t.Fatalf("expected discovered sibling ToolB, got %#v", added)
	}
	if _, ok := view.DiscoveredTools["ToolB"]; !ok {
		t.Fatalf("expected ToolB to be recorded as discovered")
	}
}
