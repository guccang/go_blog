package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionStoreSaveRequestTrace(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	session := NewRootSession("task_trace_case", "deploy blog-agent", "alice")
	session.Description = "帮我部署 blog-agent 到 ssh-prod"
	trace := NewRequestTrace("task_trace_case", "web", "root_query", session.Description, session)
	trace.SetDescription(session.Description)
	trace.SetToolView(&ToolRuntimeView{
		Policy:        "root_skill_match",
		MatchedSkills: []string{"deploy"},
		AllTools: []LLMTool{
			{Function: LLMFunction{Name: "DeployProject"}},
			{Function: LLMFunction{Name: "execute_skill"}},
		},
		VisibleTools: []LLMTool{
			{Function: LLMFunction{Name: "DeployProject"}},
			{Function: LLMFunction{Name: "execute_skill"}},
		},
		SourceReasons: map[string]string{
			"DeployProject": "skill",
			"execute_skill": "runtime",
		},
	})
	trace.RecordPath("task_start", "进入根任务 QueryLoop", nil)
	trace.RecordPath("round_1_llm", "LLM返回 text_len=32 tool_calls=1", map[string]string{"tools": "execute_skill"})
	trace.RecordEvent("tool_call", "execute_skill", "skill=deploy child_session=skill_abcd1234", 1, nil)
	trace.RecordRoundLLM(1, 320*time.Millisecond, "我将调用 deploy skill 来处理部署任务。", []ToolCall{
		{ID: "call_1", Function: FunctionCall{Name: "execute_skill", Arguments: `{"skill_name":"deploy"}`}},
	}, []LLMTool{
		{Function: LLMFunction{Name: "DeployProject"}},
		{Function: LLMFunction{Name: "execute_skill"}},
	})
	round := trace.EnsureRound(1)
	round.ToolCalls = append(round.ToolCalls, TraceToolCall{
		ToolName:      "execute_skill",
		ToolCallID:    "call_1",
		Success:       true,
		DurationMs:    1800,
		ResultLen:     96,
		ResultPreview: "技能 deploy 子任务已结束，status=done，session_id=skill_abcd1234。",
	})
	trace.Finish("done", "部署完成，session_id=skill_abcd1234", nil)

	if err := store.SaveRequestTrace(trace); err != nil {
		t.Fatalf("save request trace: %v", err)
	}

	jsonPath := filepath.Join(store.baseDir, session.RootID, "trace_"+session.ID+".json")
	mdPath := filepath.Join(store.baseDir, session.RootID, "trace_"+session.ID+".md")

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read trace json: %v", err)
	}
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read trace markdown: %v", err)
	}

	jsonText := string(jsonData)
	mdText := string(mdData)
	if !strings.Contains(jsonText, `"scope": "root_query"`) {
		t.Fatalf("expected scope in trace json, got: %s", jsonText)
	}
	if !strings.Contains(jsonText, `"tool_name": "execute_skill"`) {
		t.Fatalf("expected tool call in trace json, got: %s", jsonText)
	}
	if !strings.Contains(mdText, "```mermaid") {
		t.Fatalf("expected mermaid chart in trace markdown, got: %s", mdText)
	}
	if !strings.Contains(mdText, "execute_skill") {
		t.Fatalf("expected tool call in trace markdown, got: %s", mdText)
	}
	if !strings.Contains(mdText, "帮我部署 blog-agent 到 ssh-prod") {
		t.Fatalf("expected task description in trace markdown, got: %s", mdText)
	}
}
