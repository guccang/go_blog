package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"uap"
)

type testGateway struct {
	server     *uap.Server
	httpServer *httptest.Server

	mu         sync.Mutex
	events     map[string][]GatewayEvent
	taskTraces map[string]string
}

func newTestGateway(t *testing.T) *testGateway {
	t.Helper()
	gw := &testGateway{
		server:     uap.NewServer(),
		events:     make(map[string][]GatewayEvent),
		taskTraces: make(map[string]string),
	}
	gw.server.OnMessageReceived = func(from *uap.AgentConn, msg *uap.Message) {
		gw.record("msg_in", from, nil, msg)
	}
	gw.server.OnMessageForwarded = func(from *uap.AgentConn, to *uap.AgentConn, msg *uap.Message) {
		gw.record("msg_out", from, to, msg)
	}
	gw.server.OnRouteError = func(from *uap.AgentConn, msg *uap.Message) {
		gw.record("route_error", from, nil, msg)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/uap", gw.server.HandleWebSocket)
	mux.HandleFunc("/api/gateway/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"agents": len(gw.server.GetAllAgents()),
		})
	})
	mux.HandleFunc("/api/gateway/agents", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"agents":  gw.server.GetAllAgents(),
		})
	})
	mux.HandleFunc("/api/gateway/events/trace/", func(w http.ResponseWriter, r *http.Request) {
		traceID := strings.TrimPrefix(r.URL.Path, "/api/gateway/events/trace/")
		gw.mu.Lock()
		events := append([]GatewayEvent(nil), gw.events[traceID]...)
		gw.mu.Unlock()
		status := "not_found"
		for _, event := range events {
			if event.MsgType == uap.MsgToolResult || event.MsgType == uap.MsgTaskComplete {
				status = "completed"
			}
		}
		if len(events) > 0 && status == "not_found" {
			status = "in_progress"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     true,
			"trace_id":    traceID,
			"events":      events,
			"duration_ms": 1,
			"status":      status,
		})
	})
	gw.httpServer = httptest.NewServer(mux)
	return gw
}

func (g *testGateway) close() {
	g.httpServer.Close()
}

func (g *testGateway) wsURL() string {
	return "ws" + strings.TrimPrefix(g.httpServer.URL, "http") + "/ws/uap"
}

func (g *testGateway) httpURL() string {
	return g.httpServer.URL
}

func (g *testGateway) record(kind string, from *uap.AgentConn, to *uap.AgentConn, msg *uap.Message) {
	traceID := msg.ID
	switch msg.Type {
	case uap.MsgToolResult:
		var payload uap.ToolResultPayload
		if json.Unmarshal(msg.Payload, &payload) == nil && payload.RequestID != "" {
			traceID = payload.RequestID
		}
	case uap.MsgTaskAssign:
		var payload uap.TaskAssignPayload
		if json.Unmarshal(msg.Payload, &payload) == nil && payload.TaskID != "" {
			g.taskTraces[payload.TaskID] = msg.ID
		}
	case uap.MsgTaskComplete, uap.MsgTaskAccepted, uap.MsgTaskRejected:
		var payload struct {
			TaskID string `json:"task_id"`
		}
		if json.Unmarshal(msg.Payload, &payload) == nil && payload.TaskID != "" {
			traceID = g.taskTraces[payload.TaskID]
		}
	}
	event := GatewayEvent{
		Seq:            uint64(len(g.events[traceID]) + 1),
		Kind:           kind,
		TraceID:        traceID,
		MsgID:          msg.ID,
		From:           msg.From,
		To:             msg.To,
		MsgType:        msg.Type,
		PayloadSummary: string(msg.Payload),
	}
	if from != nil {
		event.From = from.ID
	}
	if to != nil {
		event.To = to.ID
	}
	g.mu.Lock()
	g.events[traceID] = append(g.events[traceID], event)
	g.mu.Unlock()
}

func TestRunStoreWritesArtifacts(t *testing.T) {
	store := NewRunStore(t.TempDir())
	scenario := &TestScenario{ID: "s1", Title: "demo", Entry: ScenarioEntry{Type: EntryTypeToolCall, ToAgent: "demo-agent"}}
	run := newTestRun("suite", scenario, "trace123", "")
	run.Trace = &ExecutionTraceSnapshot{TraceID: "trace123", Status: "completed"}
	run.Result = TestRunResult{Success: true, Status: RunStatusPassed}
	run.finish(RunStatusPassed)

	if err := store.SaveScenario(run, scenario); err != nil {
		t.Fatalf("SaveScenario error: %v", err)
	}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	report := &SystemEvaluationReport{
		RunID:           "suite-run",
		SuiteID:         "suite",
		Title:           "suite",
		Status:          RunStatusPassed,
		StartedAt:       time.Now(),
		FinishedAt:      time.Now(),
		TotalScenarios:  1,
		PassedScenarios: 1,
		Runs:            []*TestRun{run},
	}
	if err := store.SaveSuiteReport(report); err != nil {
		t.Fatalf("SaveSuiteReport error: %v", err)
	}
}

func TestRunnerRunToolCallScenario(t *testing.T) {
	gw := newTestGateway(t)
	defer gw.close()

	agent := uap.NewClient(gw.wsURL(), "echo-agent", "echo", "echo-agent")
	agent.Tools = []uap.ToolDef{{Name: "Echo"}}
	agent.OnMessage = func(msg *uap.Message) {
		if msg.Type != uap.MsgToolCall {
			return
		}
		var payload uap.ToolCallPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Errorf("unmarshal tool call: %v", err)
			return
		}
		var args map[string]any
		_ = json.Unmarshal(payload.Arguments, &args)
		text, _ := args["text"].(string)
		_ = agent.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   true,
			Result:    `{"echo":"` + text + `"}`,
		})
	}
	go agent.Run()
	defer agent.Stop()

	time.Sleep(300 * time.Millisecond)

	cfg := DefaultConfig()
	cfg.GatewayURL = gw.wsURL()
	cfg.GatewayHTTP = gw.httpURL()
	cfg.OutputDir = t.TempDir()
	cfg.DefaultTimeoutSec = 5
	runner := NewRunner(cfg)
	defer runner.Close()

	suite := &TestSuite{
		ID:    "tool-suite",
		Title: "tool-suite",
		Scenarios: []TestScenario{
			{
				ID:    "echo-tool",
				Title: "echo-tool",
				Entry: ScenarioEntry{
					Type:    EntryTypeToolCall,
					ToAgent: "echo-agent",
					Tool: &ToolCallEntry{
						ToolName:  "Echo",
						Arguments: mustMarshal(map[string]any{"text": "hello"}),
					},
				},
				Assertions: TestAssertions{
					ExpectMessageType: "tool_result",
					ResultContains:    []string{"hello"},
					RequireAgents:     []string{"echo-agent"},
					RequireMsgTypes:   []string{"tool_call", "tool_result"},
					MinTraceEvents:    2,
				},
			},
		},
	}

	report, err := runner.RunSuite(context.Background(), suite, "")
	if err != nil {
		t.Fatalf("RunSuite error: %v", err)
	}
	if report.TotalScenarios != 1 {
		t.Fatalf("expected 1 scenario, got %d", report.TotalScenarios)
	}
	if report.PassedScenarios != 1 {
		t.Fatalf("expected 1 passed scenario, got %d", report.PassedScenarios)
	}
	if len(report.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(report.Runs))
	}
	run := report.Runs[0]
	if !run.Result.Success {
		t.Fatalf("expected success, got %+v", run.Result)
	}
	if !strings.Contains(run.Result.FinalResult, "hello") {
		t.Fatalf("expected final result to contain hello, got %s", run.Result.FinalResult)
	}
}
