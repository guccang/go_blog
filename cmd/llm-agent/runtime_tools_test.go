package main

import (
	"context"
	"testing"
)

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

func TestBuildRootToolRuntimeViewFiltersCodingAndDeployTasks(t *testing.T) {
	bridge := &Bridge{
		cfg: &Config{MaxMatchedSkills: 2},
		skillMgr: &SkillManager{
			skills: []SkillEntry{
				{Name: "coding", Tools: []string{"AcpStartSession"}, Keywords: []string{"编码", "实现"}},
				{Name: "deploy", Tools: []string{"DeployAdhoc"}, Keywords: []string{"部署"}},
				{Name: "debug", Tools: []string{"Bash"}, Keywords: []string{"排查"}},
			},
		},
	}

	view := bridge.buildRootToolRuntimeView(&TaskContext{}, "编码，使用go语言实现web计算器，监听8888端口，然后部署到ssh-prod", []LLMTool{
		testTool("Bash"),
		testTool("AcpStartSession"),
		testTool("DeployAdhoc"),
	})

	var (
		hasBash   bool
		hasACP    bool
		hasDeploy bool
	)
	for _, tool := range view.VisibleTools {
		switch tool.Function.Name {
		case "Bash":
			hasBash = true
		case "AcpStartSession":
			hasACP = true
		case "DeployAdhoc":
			hasDeploy = true
		}
	}

	if hasBash {
		t.Fatalf("coding+deploy root task should not expose Bash")
	}
	if !hasACP || !hasDeploy {
		t.Fatalf("expected coding and deploy tools to remain visible, got %#v", view.VisibleTools)
	}
}

func TestBashToolRejectsSourceWriteCommands(t *testing.T) {
	manager := &BashToolManager{}
	_, err := manager.Exec(context.Background(), "echo 'package main' > main.go", "")
	if err == nil {
		t.Fatalf("expected source write command to be rejected")
	}
	if got := err.Error(); got == "" || !containsAny(got, "不允许直接创建或编辑源码文件", "编码请改用") {
		t.Fatalf("unexpected error: %v", err)
	}
}
