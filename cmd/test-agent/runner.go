package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"uap"
)

type gatewayTraceResponse struct {
	Success    bool           `json:"success"`
	TraceID    string         `json:"trace_id"`
	Events     []GatewayEvent `json:"events"`
	DurationMs int64          `json:"duration_ms"`
	Status     string         `json:"status"`
}

type gatewayAgentsResponse struct {
	Success bool                   `json:"success"`
	Agents  []GatewayAgentSnapshot `json:"agents"`
}

type gatewayHealthResponse struct {
	Status string `json:"status"`
	Agents int    `json:"agents"`
}

type directObservation struct {
	msg          *uap.Message
	taskComplete *uap.TaskCompletePayload
	toolResult   *uap.ToolResultPayload
	errorMsg     *uap.ErrorPayload
	notify       *uap.NotifyPayload
}

type runScenarioOptions struct {
	EvaluationID   string
	CollectionType string
}

type Runner struct {
	cfg          *Config
	store        *RunStore
	httpClient   *http.Client
	client       *uap.Client
	registeredCh chan bool
	eventSink    RunnerEventSink

	mu      sync.Mutex
	current *scenarioRuntime
	direct  *directObservation
}

func NewRunner(cfg *Config) *Runner {
	r := &Runner{
		cfg:        cfg,
		store:      NewRunStore(cfg.OutputDir),
		httpClient: &http.Client{Timeout: 5 * time.Second},
		client:     uap.NewClient(cfg.GatewayURL, cfg.AgentID, "test_agent", cfg.AgentName),
	}
	r.client.AuthToken = cfg.AuthToken
	r.client.Description = "用户模拟测试 agent，负责回放测试场景并落盘执行细节"
	r.client.Capacity = 1
	r.client.Meta = map[string]any{
		"purpose": "system-evaluation",
	}
	r.client.OnMessage = r.handleMessage
	r.client.OnRegistered = func(success bool) {
		if r.registeredCh != nil {
			select {
			case r.registeredCh <- success:
			default:
			}
		}
	}
	return r
}

func (r *Runner) SetEventSink(sink RunnerEventSink) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventSink = sink
}

func (r *Runner) Close() {
	if r.client != nil {
		r.client.Stop()
	}
}

func (r *Runner) Connect(ctx context.Context) error {
	if r.client.IsConnected() {
		return nil
	}
	r.registeredCh = make(chan bool, 1)
	go r.client.Run()
	select {
	case ok := <-r.registeredCh:
		if !ok {
			return fmt.Errorf("gateway registration rejected")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return fmt.Errorf("wait register timeout")
	}
}

func (r *Runner) RunSuite(ctx context.Context, suite *TestSuite, scenarioID string) (*SystemEvaluationReport, error) {
	if suite == nil {
		return nil, fmt.Errorf("suite is nil")
	}
	if err := r.Connect(ctx); err != nil {
		return nil, err
	}
	report := &SystemEvaluationReport{
		RunID:          fmt.Sprintf("%s-%s", strings.TrimSpace(suite.ID), uap.NewMsgID()),
		SuiteID:        strings.TrimSpace(suite.ID),
		Title:          firstNonEmpty(strings.TrimSpace(suite.Title), strings.TrimSpace(suite.ID), "test-suite"),
		CollectionType: firstNonEmpty(strings.TrimSpace(suite.Collection), CollectionTypeManual),
		Status:         RunStatusRunning,
		StartedAt:      time.Now(),
	}
	for i := range suite.Scenarios {
		scenario := &suite.Scenarios[i]
		if strings.TrimSpace(scenarioID) != "" && scenario.ID != scenarioID {
			continue
		}
		run, err := r.runScenario(ctx, suite.ID, scenario, runScenarioOptions{
			CollectionType: report.CollectionType,
		})
		if err != nil {
			return nil, err
		}
		report.Runs = append(report.Runs, run)
		report.ExecutedScenarios++
		if run.Status == RunStatusPassed {
			report.PassedScenarios++
		} else {
			report.FailedScenarios++
		}
		report.TotalScenarios++
		r.saveSuiteReport(report)
	}
	report.FinishedAt = time.Now()
	report.Status = RunStatusPassed
	if report.FailedScenarios > 0 {
		report.Status = RunStatusFailed
	}
	totalScore := 0
	for _, run := range report.Runs {
		totalScore += run.Result.Scores.Total
	}
	if report.TotalScenarios > 0 {
		report.AverageScore = totalScore / report.TotalScenarios
	}
	report.DimensionScores = aggregateDimensionScores(report.Runs)
	r.saveSuiteReport(report)
	return report, nil
}

func (r *Runner) RunScenario(ctx context.Context, suiteID string, scenario *TestScenario) (*TestRun, error) {
	return r.runScenario(ctx, suiteID, scenario, runScenarioOptions{})
}

func (r *Runner) runScenario(ctx context.Context, suiteID string, scenario *TestScenario, opts runScenarioOptions) (*TestRun, error) {
	if scenario == nil {
		return nil, fmt.Errorf("scenario is nil")
	}
	traceID := uap.NewMsgID()
	taskID := scenario.resolvedTaskID()
	if taskID == "" && strings.TrimSpace(scenario.Entry.Type) == EntryTypeTaskAssign {
		taskID = "task-" + traceID
	}
	run := newTestRun(suiteID, scenario, traceID, taskID)
	run.EvaluationID = strings.TrimSpace(opts.EvaluationID)
	run.CollectionType = firstNonEmpty(strings.TrimSpace(opts.CollectionType), run.CollectionType)
	r.mu.Lock()
	r.current = &scenarioRuntime{
		run:            run,
		scenario:       scenario,
		expectedDirect: scenario.directMessageExpectation(),
	}
	r.direct = nil
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.current = nil
		r.direct = nil
		r.mu.Unlock()
	}()

	_ = r.store.SaveScenario(run, scenario)
	r.saveRun(run)

	availStep := run.beginStep("capture_availability", "抓取 Gateway health 和 agents 快照")
	health, agents, err := r.captureAvailability()
	if err != nil {
		run.finishStep(availStep, StepStatusFailed, err.Error(), nil)
		run.Result.FinalError = err.Error()
		run.finish(RunStatusError)
		r.saveRun(run)
		return run, nil
	}
	run.Health = health
	run.OnlineAgents = agents
	run.finishStep(availStep, StepStatusPassed, "Gateway 可访问", map[string]any{
		"agents": len(agents),
	})
	r.saveRun(run)

	dispatchStep := run.beginStep("dispatch_entry", "发送入口消息")
	if err := r.dispatchScenario(run, scenario); err != nil {
		run.finishStep(dispatchStep, StepStatusFailed, err.Error(), nil)
		run.Result.FinalError = err.Error()
		run.finish(RunStatusError)
		r.saveRun(run)
		return run, nil
	}
	run.finishStep(dispatchStep, StepStatusPassed, "入口消息发送成功", map[string]any{
		"trace_id": traceID,
		"task_id":  taskID,
	})
	r.saveRun(run)

	waitStep := run.beginStep("await_execution", "轮询等待链路完成")
	waitErr := r.awaitScenario(ctx, run, scenario)
	if waitErr != nil {
		run.finishStep(waitStep, StepStatusFailed, waitErr.Error(), nil)
	} else {
		run.finishStep(waitStep, StepStatusPassed, "链路执行结束", nil)
	}

	traceStep := run.beginStep("collect_trace", "抓取 Gateway trace")
	trace, err := r.fetchTrace(run.TraceID)
	if err != nil {
		run.finishStep(traceStep, StepStatusFailed, err.Error(), nil)
	} else {
		run.Trace = trace
		run.finishStep(traceStep, StepStatusPassed, firstNonEmpty(trace.Status, defaultTraceStatus), map[string]any{
			"events": len(trace.Events),
		})
	}
	r.saveRun(run)

	llmStep := run.beginStep("collect_llm_trace", "尝试匹配 llm-agent trace 文件")
	llmTrace, err := loadRecentLLMTrace(r.cfg.LLMTraceDir, scenario, run.StartedAt, run.TaskID)
	if err != nil {
		run.finishStep(llmStep, StepStatusFailed, err.Error(), nil)
	} else if llmTrace == nil {
		run.finishStep(llmStep, StepStatusSkipped, "未匹配到 llm-agent trace", nil)
	} else {
		run.LLMTrace = llmTrace
		run.finishStep(llmStep, StepStatusPassed, filepath.Base(llmTrace.FilePath), nil)
	}
	r.saveRun(run)

	evalStep := run.beginStep("evaluate_assertions", "评估断言与评分")
	r.evaluateRun(run, scenario, waitErr)
	status := RunStatusPassed
	if !run.Result.Success {
		status = RunStatusFailed
		if waitErr != nil {
			status = RunStatusTimeout
		}
	}
	run.finish(status)
	run.finishStep(evalStep, StepStatusPassed, run.Result.Status, map[string]any{
		"score": run.Result.Scores.Total,
	})
	r.saveRun(run)
	return run, nil
}

func (r *Runner) captureAvailability() (*GatewayHealthSnapshot, []GatewayAgentSnapshot, error) {
	var health *GatewayHealthSnapshot
	var agents []GatewayAgentSnapshot
	if r.cfg.CaptureHealth {
		var payload gatewayHealthResponse
		if err := r.getJSON("/api/gateway/health", &payload); err != nil {
			return nil, nil, err
		}
		health = &GatewayHealthSnapshot{Status: payload.Status, Agents: payload.Agents}
	}
	if r.cfg.CaptureAgents {
		var payload gatewayAgentsResponse
		if err := r.getJSON("/api/gateway/agents", &payload); err != nil {
			return health, nil, err
		}
		agents = payload.Agents
	}
	return health, agents, nil
}

func (r *Runner) dispatchScenario(run *TestRun, scenario *TestScenario) error {
	switch strings.TrimSpace(scenario.Entry.Type) {
	case EntryTypeNotify:
		if scenario.Entry.Notify == nil {
			return fmt.Errorf("notify entry is required")
		}
		payload := uap.NotifyPayload{
			Channel:     scenario.Entry.Notify.Channel,
			To:          scenario.Entry.Notify.To,
			Content:     scenario.Entry.Notify.Content,
			MessageType: scenario.Entry.Notify.MessageType,
			Meta:        cloneMap(scenario.Entry.Notify.Meta),
		}
		return r.client.Send(&uap.Message{
			Type:    uap.MsgNotify,
			ID:      run.TraceID,
			From:    r.cfg.AgentID,
			To:      scenario.Entry.ToAgent,
			Payload: mustMarshal(payload),
			Ts:      time.Now().UnixMilli(),
		})
	case EntryTypeTaskAssign:
		if scenario.Entry.Task == nil {
			return fmt.Errorf("task entry is required")
		}
		payload := uap.TaskAssignPayload{
			TaskID:  run.TaskID,
			Payload: scenario.Entry.Task.Payload,
		}
		return r.client.Send(&uap.Message{
			Type:    uap.MsgTaskAssign,
			ID:      run.TraceID,
			From:    r.cfg.AgentID,
			To:      scenario.Entry.ToAgent,
			Payload: mustMarshal(payload),
			Ts:      time.Now().UnixMilli(),
		})
	case EntryTypeToolCall:
		if scenario.Entry.Tool == nil {
			return fmt.Errorf("tool entry is required")
		}
		payload := uap.ToolCallPayload{
			ToolName:          scenario.Entry.Tool.ToolName,
			Arguments:         scenario.Entry.Tool.Arguments,
			AuthenticatedUser: scenario.Entry.Tool.AuthenticatedUser,
		}
		return r.client.Send(&uap.Message{
			Type:    uap.MsgToolCall,
			ID:      run.TraceID,
			From:    r.cfg.AgentID,
			To:      scenario.Entry.ToAgent,
			Payload: mustMarshal(payload),
			Ts:      time.Now().UnixMilli(),
		})
	default:
		return fmt.Errorf("unsupported entry type: %s", scenario.Entry.Type)
	}
}

func (r *Runner) awaitScenario(ctx context.Context, run *TestRun, scenario *TestScenario) error {
	timeout := scenario.timeoutOrDefault(r.cfg.DefaultTimeoutSec)
	deadline := time.Now().Add(timeout)
	settle := time.Duration(r.cfg.SettleAfterMs) * time.Millisecond
	poll := time.Duration(r.cfg.PollIntervalMs) * time.Millisecond

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("scenario timeout after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		trace, _ := r.fetchTrace(run.TraceID)
		if trace != nil {
			run.Trace = trace
		}

		terminal := r.currentDirectObservation()
		if terminal != nil && r.current != nil && r.current.expectedDirect != "" {
			if settle > 0 {
				time.Sleep(settle)
				if trace, _ := r.fetchTrace(run.TraceID); trace != nil {
					run.Trace = trace
				}
			}
			return nil
		}
		if r.current != nil && r.current.expectedDirect == "" && traceAssertionsSatisfied(scenario, run.Trace) {
			return nil
		}
		time.Sleep(poll)
	}
}

func (r *Runner) handleMessage(msg *uap.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil || r.current.run == nil {
		return
	}
	r.current.run.appendObservedMessage(msg)
	switch msg.Type {
	case uap.MsgToolResult:
		var payload uap.ToolResultPayload
		if json.Unmarshal(msg.Payload, &payload) == nil && payload.RequestID == r.current.run.TraceID {
			r.direct = &directObservation{msg: msg, toolResult: &payload}
		}
	case uap.MsgTaskAccepted, uap.MsgTaskRejected, uap.MsgTaskEvent, uap.MsgTaskComplete:
		var payload struct {
			TaskID string `json:"task_id"`
			Status string `json:"status,omitempty"`
			Error  string `json:"error,omitempty"`
			Result string `json:"result,omitempty"`
		}
		if json.Unmarshal(msg.Payload, &payload) == nil && payload.TaskID == r.current.run.TaskID {
			if msg.Type == uap.MsgTaskComplete {
				r.direct = &directObservation{
					msg: msg,
					taskComplete: &uap.TaskCompletePayload{
						TaskID: payload.TaskID,
						Status: payload.Status,
						Error:  payload.Error,
						Result: payload.Result,
					},
				}
			}
			if msg.Type == uap.MsgTaskRejected {
				r.direct = &directObservation{
					msg: msg,
					errorMsg: &uap.ErrorPayload{
						Code:    "task_rejected",
						Message: payload.Error,
					},
				}
			}
		}
	case uap.MsgError:
		var payload uap.ErrorPayload
		if json.Unmarshal(msg.Payload, &payload) == nil && msg.ID == r.current.run.TraceID {
			r.direct = &directObservation{msg: msg, errorMsg: &payload}
		}
	case uap.MsgNotify:
		var payload uap.NotifyPayload
		if json.Unmarshal(msg.Payload, &payload) == nil {
			r.direct = &directObservation{msg: msg, notify: &payload}
		}
	}
	r.saveRun(r.current.run)
}

func (r *Runner) currentDirectObservation() *directObservation {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.direct == nil {
		return nil
	}
	copyValue := *r.direct
	return &copyValue
}

func (r *Runner) fetchTrace(traceID string) (*ExecutionTraceSnapshot, error) {
	var payload gatewayTraceResponse
	if err := r.getJSON("/api/gateway/events/trace/"+traceID, &payload); err != nil {
		return nil, err
	}
	trace := &ExecutionTraceSnapshot{
		TraceID:    payload.TraceID,
		Status:     firstNonEmpty(strings.TrimSpace(payload.Status), defaultTraceStatus),
		DurationMs: payload.DurationMs,
		Events:     payload.Events,
	}
	trace.Agents = collectTraceAgents(payload.Events)
	trace.MessageTypes = collectTraceMsgTypes(payload.Events)
	trace.Summaries = collectTraceSummaries(payload.Events)
	trace.Path = collectTracePath(payload.Events)
	return trace, nil
}

func (r *Runner) publishEvent(event RunnerEvent) {
	r.mu.Lock()
	sink := r.eventSink
	r.mu.Unlock()
	if sink == nil {
		return
	}
	sink.HandleRunnerEvent(event)
}

func (r *Runner) saveRun(run *TestRun) {
	_ = r.store.SaveRun(run)
	r.publishEvent(RunnerEvent{
		Type:         RunnerEventRunUpdated,
		Mode:         firstNonEmpty(strings.TrimSpace(run.CollectionType), CollectionTypeManual),
		EvaluationID: strings.TrimSpace(run.EvaluationID),
		Run:          clonePointer(run),
	})
}

func (r *Runner) saveSuiteReport(report *SystemEvaluationReport) {
	_ = r.store.SaveSuiteReport(report)
	r.publishEvent(RunnerEvent{
		Type:         RunnerEventSuiteReport,
		Mode:         CollectionTypeManual,
		EvaluationID: strings.TrimSpace(report.EvaluationID),
		SuiteReport:  clonePointer(report),
	})
}

func (r *Runner) saveCollectionReport(report *SystemEvaluationReport) {
	_ = r.store.SaveCollectionReport(report)
	r.publishEvent(RunnerEvent{
		Type:             RunnerEventCollectionReport,
		Mode:             firstNonEmpty(strings.TrimSpace(report.CollectionType), CollectionTypeManual),
		EvaluationID:     strings.TrimSpace(report.EvaluationID),
		CollectionReport: clonePointer(report),
	})
}

func (r *Runner) saveEvaluationPlan(evaluationID string, plan *EvaluationPlan) {
	_ = r.store.SaveEvaluationPlan(evaluationID, plan)
	r.publishEvent(RunnerEvent{
		Type:         RunnerEventEvaluationPlan,
		Mode:         "evaluation",
		EvaluationID: strings.TrimSpace(evaluationID),
		Plan:         clonePointer(plan),
	})
}

func (r *Runner) saveFinalReport(report *FinalEvaluationReport) {
	_ = r.store.SaveFinalReport(report)
	r.publishEvent(RunnerEvent{
		Type:         RunnerEventEvaluationDone,
		Mode:         "evaluation",
		EvaluationID: strings.TrimSpace(report.RunID),
		FinalReport:  clonePointer(report),
	})
}

func (r *Runner) getJSON(path string, target any) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(r.cfg.GatewayHTTP, "/")+path, nil)
	if err != nil {
		return err
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (r *Runner) evaluateRun(run *TestRun, scenario *TestScenario, waitErr error) {
	direct := r.currentDirectObservation()
	if direct != nil {
		run.Result.FinalMessageType = direct.msg.Type
		switch {
		case direct.toolResult != nil:
			if direct.toolResult.Success {
				run.Result.FinalStatus = "success"
				run.Result.FinalResult = direct.toolResult.Result
			} else {
				run.Result.FinalStatus = "failed"
				run.Result.FinalError = direct.toolResult.Error
			}
		case direct.taskComplete != nil:
			run.Result.FinalStatus = strings.TrimSpace(direct.taskComplete.Status)
			run.Result.FinalResult = strings.TrimSpace(direct.taskComplete.Result)
			run.Result.FinalError = strings.TrimSpace(direct.taskComplete.Error)
		case direct.errorMsg != nil:
			run.Result.FinalStatus = "error"
			run.Result.FinalError = firstNonEmpty(direct.errorMsg.Message, run.Result.FinalError)
		case direct.notify != nil:
			run.Result.FinalStatus = "notified"
			run.Result.FinalResult = strings.TrimSpace(direct.notify.Content)
		}
	}
	if waitErr != nil && run.Result.FinalError == "" {
		run.Result.FinalError = waitErr.Error()
	}

	var outcomes []AssertionOutcome
	assertions := scenario.Assertions
	if expected := scenario.directMessageExpectation(); expected != "" {
		success := run.Result.FinalMessageType == expected
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "direct_message_type",
			Success: success,
			Detail:  fmt.Sprintf("got=%s want=%s", run.Result.FinalMessageType, expected),
		})
	}
	if assertions.ExpectTaskStatus != "" {
		success := run.Result.FinalStatus == assertions.ExpectTaskStatus
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "task_status",
			Success: success,
			Detail:  fmt.Sprintf("got=%s want=%s", run.Result.FinalStatus, assertions.ExpectTaskStatus),
		})
	}
	if assertions.ExpectToolSuccess != nil {
		got := run.Result.FinalStatus == "success"
		success := got == *assertions.ExpectToolSuccess
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "tool_success",
			Success: success,
			Detail:  fmt.Sprintf("got=%t want=%t", got, *assertions.ExpectToolSuccess),
		})
	}
	for _, needle := range assertions.ResultContains {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		success := strings.Contains(run.Result.FinalResult, needle)
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "result_contains",
			Success: success,
			Detail:  fmt.Sprintf("needle=%q", needle),
		})
	}
	for _, needle := range assertions.ErrorContains {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		success := strings.Contains(run.Result.FinalError, needle)
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "error_contains",
			Success: success,
			Detail:  fmt.Sprintf("needle=%q", needle),
		})
	}
	if assertions.MinTraceEvents > 0 {
		got := 0
		if run.Trace != nil {
			got = len(run.Trace.Events)
		}
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "trace_events",
			Success: got >= assertions.MinTraceEvents,
			Detail:  fmt.Sprintf("got=%d want>=%d", got, assertions.MinTraceEvents),
		})
	}
	for _, agentID := range assertions.RequireAgents {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "require_agent",
			Success: traceContains(run.Trace, func(e GatewayEvent) bool { return e.From == agentID || e.To == agentID }),
			Detail:  agentID,
		})
	}
	for _, msgType := range assertions.RequireMsgTypes {
		msgType = strings.TrimSpace(msgType)
		if msgType == "" {
			continue
		}
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "require_msg_type",
			Success: traceContains(run.Trace, func(e GatewayEvent) bool { return e.MsgType == msgType }),
			Detail:  msgType,
		})
	}
	for _, text := range assertions.RequireSummaryContains {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "require_summary_contains",
			Success: traceContains(run.Trace, func(e GatewayEvent) bool { return strings.Contains(e.PayloadSummary, text) }),
			Detail:  text,
		})
	}
	if len(assertions.ExpectedPath) > 0 {
		actualPath := []string(nil)
		if run.Trace != nil {
			actualPath = run.Trace.Path
		}
		outcomes = append(outcomes, AssertionOutcome{
			Name:    "expected_path",
			Success: isSubsequence(assertions.ExpectedPath, actualPath),
			Detail:  fmt.Sprintf("actual=%s", strings.Join(actualPath, " -> ")),
		})
	}

	success := true
	for _, outcome := range outcomes {
		if !outcome.Success {
			success = false
			break
		}
	}
	if len(outcomes) == 0 {
		success = waitErr == nil
	}
	run.Result.Success = success
	run.Result.Outcomes = outcomes
	run.Result.Scores = buildScores(run, scenario, outcomes)
	if !success && run.Result.Status == RunStatusRunning {
		run.Result.Status = RunStatusFailed
	}
	if success {
		run.Result.Status = RunStatusPassed
	}
}

func buildScores(run *TestRun, scenario *TestScenario, outcomes []AssertionOutcome) ScoreBreakdown {
	score := ScoreBreakdown{}
	score.CompletionScore = 100
	if run.Result.FinalError != "" || run.Result.Status == RunStatusTimeout {
		score.CompletionScore = 0
	}

	routingChecks := 0
	routingPassed := 0
	for _, outcome := range outcomes {
		if outcome.Name == "require_agent" || outcome.Name == "require_msg_type" || outcome.Name == "expected_path" {
			routingChecks++
			if outcome.Success {
				routingPassed++
			}
		}
	}
	score.RoutingScore = percentageOrFull(routingPassed, routingChecks)

	toolChecks := 0
	toolPassed := 0
	for _, outcome := range outcomes {
		if outcome.Name == "require_summary_contains" || outcome.Name == "tool_success" {
			toolChecks++
			if outcome.Success {
				toolPassed++
			}
		}
	}
	score.ToolUsageScore = percentageOrFull(toolPassed, toolChecks)

	if scenario.Assertions.FailureExpected {
		if run.Result.FinalError != "" || run.Result.FinalStatus == "failed" || run.Result.FinalStatus == "error" {
			score.RecoveryScore = 100
		} else {
			score.RecoveryScore = 0
		}
	} else if run.Result.FinalError == "" {
		score.RecoveryScore = 100
	}

	answerChecks := 0
	answerPassed := 0
	for _, outcome := range outcomes {
		if outcome.Name == "result_contains" || outcome.Name == "error_contains" || outcome.Name == "task_status" || outcome.Name == "direct_message_type" {
			answerChecks++
			if outcome.Success {
				answerPassed++
			}
		}
	}
	score.FinalAnswerScore = percentageOrFull(answerPassed, answerChecks)
	score.Total = (score.CompletionScore + score.RoutingScore + score.ToolUsageScore + score.RecoveryScore + score.FinalAnswerScore) / 5
	return score
}

func traceAssertionsSatisfied(scenario *TestScenario, trace *ExecutionTraceSnapshot) bool {
	if scenario == nil || trace == nil {
		return false
	}
	assertions := scenario.Assertions
	if assertions.MinTraceEvents > 0 && len(trace.Events) < assertions.MinTraceEvents {
		return false
	}
	for _, agentID := range assertions.RequireAgents {
		if !traceContains(trace, func(e GatewayEvent) bool { return e.From == agentID || e.To == agentID }) {
			return false
		}
	}
	for _, msgType := range assertions.RequireMsgTypes {
		if !traceContains(trace, func(e GatewayEvent) bool { return e.MsgType == msgType }) {
			return false
		}
	}
	for _, text := range assertions.RequireSummaryContains {
		if !traceContains(trace, func(e GatewayEvent) bool { return strings.Contains(e.PayloadSummary, text) }) {
			return false
		}
	}
	if len(assertions.ExpectedPath) > 0 && !isSubsequence(assertions.ExpectedPath, trace.Path) {
		return false
	}
	if assertions.MinTraceEvents == 0 && len(assertions.RequireAgents) == 0 && len(assertions.RequireMsgTypes) == 0 &&
		len(assertions.RequireSummaryContains) == 0 && len(assertions.ExpectedPath) == 0 {
		return false
	}
	return true
}

func traceContains(trace *ExecutionTraceSnapshot, pred func(e GatewayEvent) bool) bool {
	if trace == nil {
		return false
	}
	for _, event := range trace.Events {
		if pred(event) {
			return true
		}
	}
	return false
}

func collectTraceAgents(events []GatewayEvent) []string {
	seen := make(map[string]bool)
	var agents []string
	for _, event := range events {
		if event.From != "" && !seen[event.From] {
			seen[event.From] = true
			agents = append(agents, event.From)
		}
		if event.To != "" && !seen[event.To] {
			seen[event.To] = true
			agents = append(agents, event.To)
		}
	}
	sort.Strings(agents)
	return agents
}

func collectTraceMsgTypes(events []GatewayEvent) []string {
	seen := make(map[string]bool)
	var msgTypes []string
	for _, event := range events {
		if event.MsgType == "" || seen[event.MsgType] {
			continue
		}
		seen[event.MsgType] = true
		msgTypes = append(msgTypes, event.MsgType)
	}
	sort.Strings(msgTypes)
	return msgTypes
}

func collectTraceSummaries(events []GatewayEvent) []string {
	var items []string
	for _, event := range events {
		if strings.TrimSpace(event.PayloadSummary) != "" {
			items = append(items, strings.TrimSpace(event.PayloadSummary))
		}
	}
	return items
}

func collectTracePath(events []GatewayEvent) []string {
	var path []string
	appendIfNeeded := func(agentID string) {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			return
		}
		if len(path) == 0 || path[len(path)-1] != agentID {
			path = append(path, agentID)
		}
	}
	for _, event := range events {
		if event.Kind != "msg_out" && event.Kind != "msg_in" {
			continue
		}
		appendIfNeeded(event.From)
		appendIfNeeded(event.To)
	}
	return path
}

func isSubsequence(expected, actual []string) bool {
	if len(expected) == 0 {
		return true
	}
	idx := 0
	for _, item := range actual {
		if strings.TrimSpace(item) == strings.TrimSpace(expected[idx]) {
			idx++
			if idx == len(expected) {
				return true
			}
		}
	}
	return false
}

func percentageOrFull(passed, total int) int {
	if total <= 0 {
		return 100
	}
	return passed * 100 / total
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func LoadSuite(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var suite TestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return nil, err
	}
	if strings.TrimSpace(suite.ID) == "" {
		suite.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if strings.TrimSpace(suite.Title) == "" {
		suite.Title = suite.ID
	}
	return &suite, nil
}
