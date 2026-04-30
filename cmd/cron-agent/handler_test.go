package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"uap"
)

func TestBuildToolDefsExposeAccountAndTaskName(t *testing.T) {
	tools := buildToolDefs()

	listSchema := mustDecodeToolParameters(t, tools, "cronListTasks")
	if _, ok := listSchema["account"]; !ok {
		t.Fatalf("expected cronListTasks to expose account parameter")
	}

	pendingSchema := mustDecodeToolParameters(t, tools, "cronListPending")
	if _, ok := pendingSchema["account"]; !ok {
		t.Fatalf("expected cronListPending to expose account parameter")
	}

	deleteSchema := mustDecodeToolParameters(t, tools, "cronDeleteTask")
	if _, ok := deleteSchema["task_name"]; !ok {
		t.Fatalf("expected cronDeleteTask to expose task_name parameter")
	}
}

func TestToolListTasksScopesByAuthenticatedUser(t *testing.T) {
	conn := newTestConnection(t)
	defer conn.engine.Stop()

	mustAddTask(t, conn, &CronTask{ID: "task-ztt", Name: "ztt-task", TaskType: "cron_query", Schedule: "@every 1h", Account: "ztt", CreatedBy: "ztt", Query: "hello"})
	mustAddTask(t, conn, &CronTask{ID: "task-alice", Name: "alice-task", TaskType: "cron_query", Schedule: "@every 1h", Account: "alice", CreatedBy: "alice", Query: "world"})

	result, ok := conn.toolListTasks("ztt", map[string]interface{}{"account": "ztt"})
	if !ok {
		t.Fatalf("expected list tasks to succeed")
	}

	var payload struct {
		Tasks  []CronTask `json:"tasks"`
		Total  int        `json:"total"`
		Owner  string     `json:"owner"`
		Scoped bool       `json:"scoped"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal list result: %v", err)
	}

	if payload.Total != 1 || len(payload.Tasks) != 1 {
		t.Fatalf("expected 1 scoped task, got total=%d len=%d", payload.Total, len(payload.Tasks))
	}
	if payload.Tasks[0].ID != "task-ztt" {
		t.Fatalf("expected ztt task, got %+v", payload.Tasks[0])
	}
	if payload.Owner != "ztt" || !payload.Scoped {
		t.Fatalf("expected scoped owner ztt, got owner=%q scoped=%v", payload.Owner, payload.Scoped)
	}
}

func TestToolDeleteTaskAllowsDeletingByTaskName(t *testing.T) {
	conn := newTestConnection(t)
	defer conn.engine.Stop()

	mustAddTask(t, conn, &CronTask{ID: "task-ztt", Name: "nightly-report", TaskType: "cron_query", Schedule: "@every 1h", Account: "ztt", CreatedBy: "ztt", Query: "hello"})

	result, ok := conn.toolDeleteTask("ztt", map[string]interface{}{"task_name": "nightly-report"})
	if !ok {
		t.Fatalf("expected delete by task_name to succeed, got %s", result)
	}

	if _, exists := conn.engine.GetTask("task-ztt"); exists {
		t.Fatalf("expected task to be deleted")
	}
}

func TestToolDeleteTaskRejectsAmbiguousTaskName(t *testing.T) {
	conn := newTestConnection(t)
	defer conn.engine.Stop()

	mustAddTask(t, conn, &CronTask{ID: "task-1", Name: "nightly-report", TaskType: "cron_query", Schedule: "@every 1h", Account: "ztt", CreatedBy: "ztt", Query: "hello"})
	mustAddTask(t, conn, &CronTask{ID: "task-2", Name: "nightly-report", TaskType: "cron_query", Schedule: "@every 2h", Account: "ztt", CreatedBy: "ztt", Query: "world"})

	result, ok := conn.toolDeleteTask("ztt", map[string]interface{}{"task_name": "nightly-report"})
	if ok {
		t.Fatalf("expected delete by ambiguous task_name to fail")
	}
	if !strings.Contains(result, "请改用 task_id 删除") {
		t.Fatalf("expected ambiguous delete hint, got %q", result)
	}
}

func newTestConnection(t *testing.T) *Connection {
	t.Helper()

	cfg := DefaultConfig()
	cfg.TaskFile = filepath.Join(t.TempDir(), "cron-tasks.json")

	conn := NewConnection(cfg, "cron-agent-test")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	return conn
}

func mustAddTask(t *testing.T, conn *Connection, task *CronTask) {
	t.Helper()
	if err := conn.engine.AddTask(task); err != nil {
		t.Fatalf("add task: %v", err)
	}
}

func mustDecodeToolParameters(t *testing.T, tools []uap.ToolDef, toolName string) map[string]interface{} {
	t.Helper()

	for _, tool := range tools {
		if tool.Name != toolName {
			continue
		}
		var schema struct {
			Properties map[string]interface{} `json:"properties"`
		}
		if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
			t.Fatalf("decode schema for %s: %v", toolName, err)
		}
		return schema.Properties
	}
	t.Fatalf("tool not found: %s", toolName)
	return nil
}
