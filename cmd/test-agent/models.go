package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"uap"
)

const (
	EntryTypeNotify     = "notify"
	EntryTypeTaskAssign = "task_assign"
	EntryTypeToolCall   = "tool_call"
)

const (
	RunStatusPending   = "pending"
	RunStatusRunning   = "running"
	RunStatusPassed    = "passed"
	RunStatusFailed    = "failed"
	RunStatusTimeout   = "timeout"
	RunStatusError     = "error"
	RunStatusSkipped   = "skipped"
	StepStatusPending  = "pending"
	StepStatusRunning  = "running"
	StepStatusPassed   = "passed"
	StepStatusFailed   = "failed"
	StepStatusSkipped  = "skipped"
	defaultTraceStatus = "not_found"
)

const (
	CollectionTypeManual      = "manual"
	CollectionTypeStatic      = "static"
	CollectionTypeDynamicPlan = "dynamic_plan"
	CollectionTypeDynamic     = "dynamic"
)

// TestSuite 测试套件定义。
type TestSuite struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Collection  string         `json:"collection,omitempty"`
	Source      string         `json:"source,omitempty"`
	GeneratedBy string         `json:"generated_by,omitempty"`
	Scenarios   []TestScenario `json:"scenarios"`
}

// TestScenario 单个测试场景。
type TestScenario struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	Category       string         `json:"category,omitempty"`
	Priority       string         `json:"priority,omitempty"`
	DependencyMode string         `json:"dependency_mode,omitempty"`
	CollectionType string         `json:"collection_type,omitempty"`
	Source         string         `json:"source,omitempty"`
	GeneratedBy    string         `json:"generated_by,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Entry          ScenarioEntry  `json:"entry"`
	Assertions     TestAssertions `json:"assertions,omitempty"`
	Meta           map[string]any `json:"meta,omitempty"`
}

// ScenarioEntry 入口消息定义。
type ScenarioEntry struct {
	Type    string           `json:"type"`
	ToAgent string           `json:"to_agent"`
	Notify  *NotifyEntry     `json:"notify,omitempty"`
	Task    *TaskAssignEntry `json:"task,omitempty"`
	Tool    *ToolCallEntry   `json:"tool,omitempty"`
}

type NotifyEntry struct {
	Channel     string         `json:"channel"`
	To          string         `json:"to"`
	Content     string         `json:"content"`
	MessageType string         `json:"message_type,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

type TaskAssignEntry struct {
	TaskID  string          `json:"task_id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type ToolCallEntry struct {
	ToolName          string          `json:"tool_name"`
	Arguments         json.RawMessage `json:"arguments,omitempty"`
	AuthenticatedUser string          `json:"authenticated_user,omitempty"`
}

// TestAssertions 场景断言。
type TestAssertions struct {
	TimeoutSec             int      `json:"timeout_sec,omitempty"`
	ExpectMessageType      string   `json:"expect_message_type,omitempty"`
	ExpectTaskStatus       string   `json:"expect_task_status,omitempty"`
	ExpectToolSuccess      *bool    `json:"expect_tool_success,omitempty"`
	ResultContains         []string `json:"result_contains,omitempty"`
	ErrorContains          []string `json:"error_contains,omitempty"`
	RequireAgents          []string `json:"require_agents,omitempty"`
	RequireMsgTypes        []string `json:"require_msg_types,omitempty"`
	RequireSummaryContains []string `json:"require_summary_contains,omitempty"`
	ExpectedPath           []string `json:"expected_path,omitempty"`
	MinTraceEvents         int      `json:"min_trace_events,omitempty"`
	FailureExpected        bool     `json:"failure_expected,omitempty"`
}

// GatewayAgentSnapshot 对应 gateway agents API。
type GatewayAgentSnapshot struct {
	AgentID      string         `json:"agent_id"`
	AgentType    string         `json:"agent_type"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	HostPlatform string         `json:"host_platform,omitempty"`
	HostIP       string         `json:"host_ip,omitempty"`
	Workspace    string         `json:"workspace,omitempty"`
	Tools        []string       `json:"tools,omitempty"`
	Capacity     int            `json:"capacity,omitempty"`
	LastHB       string         `json:"last_hb,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
}

// GatewayHealthSnapshot 对应 gateway health API。
type GatewayHealthSnapshot struct {
	Status string `json:"status"`
	Agents int    `json:"agents"`
}

// GatewayEvent 对应 gateway trace 事件。
type GatewayEvent struct {
	Seq            uint64 `json:"seq"`
	Ts             int64  `json:"ts"`
	Kind           string `json:"kind"`
	TraceID        string `json:"trace_id"`
	MsgID          string `json:"msg_id,omitempty"`
	From           string `json:"from,omitempty"`
	FromName       string `json:"from_name,omitempty"`
	To             string `json:"to,omitempty"`
	ToName         string `json:"to_name,omitempty"`
	MsgType        string `json:"msg_type,omitempty"`
	PayloadSummary string `json:"summary,omitempty"`
	DurationMs     int64  `json:"duration_ms,omitempty"`
	Error          string `json:"error,omitempty"`
}

// ExecutionTraceSnapshot 归一化后的执行链路快照。
type ExecutionTraceSnapshot struct {
	TraceID      string         `json:"trace_id"`
	Status       string         `json:"status"`
	DurationMs   int64          `json:"duration_ms"`
	Events       []GatewayEvent `json:"events"`
	Agents       []string       `json:"agents,omitempty"`
	MessageTypes []string       `json:"message_types,omitempty"`
	Summaries    []string       `json:"summaries,omitempty"`
	Path         []string       `json:"path,omitempty"`
}

// LLMTraceSnapshot 收集到的 llm-agent trace。
type LLMTraceSnapshot struct {
	FilePath  string         `json:"file_path"`
	RootID    string         `json:"root_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	Query     string         `json:"query,omitempty"`
	Trace     map[string]any `json:"trace"`
}

// ObservedMessage 当前测试 run 实际收到的消息。
type ObservedMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	From    string          `json:"from"`
	To      string          `json:"to"`
	Payload json.RawMessage `json:"payload"`
	Ts      int64           `json:"ts"`
}

// AssertionOutcome 单个断言结果。
type AssertionOutcome struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Detail  string `json:"detail,omitempty"`
}

// ScoreBreakdown 评分拆分。
type ScoreBreakdown struct {
	CompletionScore  int `json:"completion_score"`
	RoutingScore     int `json:"routing_score"`
	ToolUsageScore   int `json:"tool_usage_score"`
	RecoveryScore    int `json:"recovery_score"`
	FinalAnswerScore int `json:"final_answer_score"`
	Total            int `json:"total"`
}

// TestRunResult 运行结果。
type TestRunResult struct {
	Success          bool               `json:"success"`
	Status           string             `json:"status"`
	FinalMessageType string             `json:"final_message_type,omitempty"`
	FinalStatus      string             `json:"final_status,omitempty"`
	FinalResult      string             `json:"final_result,omitempty"`
	FinalError       string             `json:"final_error,omitempty"`
	Outcomes         []AssertionOutcome `json:"outcomes,omitempty"`
	Scores           ScoreBreakdown     `json:"scores"`
}

// TestStepResult 单个执行阶段。
type TestStepResult struct {
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Detail     string         `json:"detail,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

// TestRun 单场景完整执行记录。
type TestRun struct {
	RunID            string                  `json:"run_id"`
	EvaluationID     string                  `json:"evaluation_id,omitempty"`
	SuiteID          string                  `json:"suite_id,omitempty"`
	ScenarioID       string                  `json:"scenario_id"`
	Title            string                  `json:"title"`
	Description      string                  `json:"description,omitempty"`
	Category         string                  `json:"category,omitempty"`
	Priority         string                  `json:"priority,omitempty"`
	DependencyMode   string                  `json:"dependency_mode,omitempty"`
	CollectionType   string                  `json:"collection_type,omitempty"`
	Status           string                  `json:"status"`
	EntryType        string                  `json:"entry_type"`
	TargetAgent      string                  `json:"target_agent"`
	TraceID          string                  `json:"trace_id"`
	TaskID           string                  `json:"task_id,omitempty"`
	StartedAt        time.Time               `json:"started_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
	FinishedAt       *time.Time              `json:"finished_at,omitempty"`
	Health           *GatewayHealthSnapshot  `json:"health,omitempty"`
	OnlineAgents     []GatewayAgentSnapshot  `json:"online_agents,omitempty"`
	Steps            []TestStepResult        `json:"steps,omitempty"`
	ObservedMessages []ObservedMessage       `json:"observed_messages,omitempty"`
	Trace            *ExecutionTraceSnapshot `json:"trace,omitempty"`
	LLMTrace         *LLMTraceSnapshot       `json:"llm_trace,omitempty"`
	Result           TestRunResult           `json:"result"`
}

// SystemEvaluationReport 套件级聚合结果。
type SystemEvaluationReport struct {
	RunID             string            `json:"run_id"`
	EvaluationID      string            `json:"evaluation_id,omitempty"`
	SuiteID           string            `json:"suite_id,omitempty"`
	Title             string            `json:"title"`
	CollectionType    string            `json:"collection_type,omitempty"`
	Status            string            `json:"status"`
	StartedAt         time.Time         `json:"started_at"`
	FinishedAt        time.Time         `json:"finished_at"`
	TotalScenarios    int               `json:"total_scenarios"`
	ExecutedScenarios int               `json:"executed_scenarios,omitempty"`
	SkippedScenarios  int               `json:"skipped_scenarios,omitempty"`
	PassedScenarios   int               `json:"passed_scenarios"`
	FailedScenarios   int               `json:"failed_scenarios"`
	AverageScore      int               `json:"average_score"`
	SourceFiles       []string          `json:"source_files,omitempty"`
	Skipped           []SkippedScenario `json:"skipped,omitempty"`
	DimensionScores   []DimensionScore  `json:"dimension_scores,omitempty"`
	Runs              []*TestRun        `json:"runs"`
}

// ScenarioPlanItem 描述单个场景是否会被执行。
type ScenarioPlanItem struct {
	SuiteID        string   `json:"suite_id,omitempty"`
	ScenarioID     string   `json:"scenario_id"`
	Title          string   `json:"title"`
	CollectionType string   `json:"collection_type"`
	Source         string   `json:"source,omitempty"`
	EntryType      string   `json:"entry_type"`
	TargetAgent    string   `json:"target_agent"`
	RequiredAgents []string `json:"required_agents,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Priority       string   `json:"priority,omitempty"`
	Eligible       bool     `json:"eligible"`
	SkipReason     string   `json:"skip_reason,omitempty"`
}

// CollectionExecutionPlan 表示一组静态或动态测试计划。
type CollectionExecutionPlan struct {
	ID             string             `json:"id"`
	Title          string             `json:"title"`
	CollectionType string             `json:"collection_type"`
	Source         string             `json:"source,omitempty"`
	GeneratedBy    string             `json:"generated_by,omitempty"`
	ScenarioCount  int                `json:"scenario_count"`
	Items          []ScenarioPlanItem `json:"items,omitempty"`
}

// EvaluationPlan 描述一次完整评估的执行蓝图。
type EvaluationPlan struct {
	RunID               string                    `json:"run_id"`
	Title               string                    `json:"title"`
	StartedAt           time.Time                 `json:"started_at"`
	Health              *GatewayHealthSnapshot    `json:"health,omitempty"`
	OnlineAgents        []GatewayAgentSnapshot    `json:"online_agents,omitempty"`
	StaticCollections   []CollectionExecutionPlan `json:"static_collections,omitempty"`
	DynamicCollection   *CollectionExecutionPlan  `json:"dynamic_collection,omitempty"`
	DynamicPlannerRunID string                    `json:"dynamic_planner_run_id,omitempty"`
	Notes               []string                  `json:"notes,omitempty"`
}

// SkippedScenario 记录被跳过的场景与原因。
type SkippedScenario struct {
	SuiteID        string   `json:"suite_id,omitempty"`
	ScenarioID     string   `json:"scenario_id"`
	Title          string   `json:"title"`
	CollectionType string   `json:"collection_type"`
	TargetAgent    string   `json:"target_agent,omitempty"`
	RequiredAgents []string `json:"required_agents,omitempty"`
	Reason         string   `json:"reason"`
}

// DimensionScore 多维度评分聚合。
type DimensionScore struct {
	Name         string `json:"name"`
	AverageScore int    `json:"average_score"`
	PassedRuns   int    `json:"passed_runs"`
	TotalRuns    int    `json:"total_runs"`
}

// AgentEvaluation 单个 agent 的参与度与质量。
type AgentEvaluation struct {
	AgentID           string `json:"agent_id"`
	AgentType         string `json:"agent_type,omitempty"`
	Online            bool   `json:"online"`
	TargetedRuns      int    `json:"targeted_runs"`
	ObservedRuns      int    `json:"observed_runs"`
	PassedRuns        int    `json:"passed_runs"`
	FailedRuns        int    `json:"failed_runs"`
	AverageScore      int    `json:"average_score"`
	LastObservedTrace string `json:"last_observed_trace,omitempty"`
}

// FinalEvaluationReport 完整评估输出。
type FinalEvaluationReport struct {
	RunID            string                  `json:"run_id"`
	Title            string                  `json:"title"`
	Status           string                  `json:"status"`
	StartedAt        time.Time               `json:"started_at"`
	FinishedAt       time.Time               `json:"finished_at"`
	Health           *GatewayHealthSnapshot  `json:"health,omitempty"`
	OnlineAgents     []GatewayAgentSnapshot  `json:"online_agents,omitempty"`
	ExecutionPlan    *EvaluationPlan         `json:"execution_plan,omitempty"`
	StaticReport     *SystemEvaluationReport `json:"static_report,omitempty"`
	DynamicReport    *SystemEvaluationReport `json:"dynamic_report,omitempty"`
	DimensionScores  []DimensionScore        `json:"dimension_scores,omitempty"`
	AgentEvaluations []AgentEvaluation       `json:"agent_evaluations,omitempty"`
	OverallScore     int                     `json:"overall_score"`
	Findings         []string                `json:"findings,omitempty"`
}

type scenarioRuntime struct {
	run            *TestRun
	scenario       *TestScenario
	expectedDirect string
}

func newTestRun(suiteID string, scenario *TestScenario, traceID, taskID string) *TestRun {
	now := time.Now()
	return &TestRun{
		RunID:          fmt.Sprintf("%s-%s", scenario.ID, traceID),
		SuiteID:        strings.TrimSpace(suiteID),
		ScenarioID:     strings.TrimSpace(scenario.ID),
		Title:          strings.TrimSpace(scenario.Title),
		Description:    strings.TrimSpace(scenario.Description),
		Category:       strings.TrimSpace(scenario.Category),
		Priority:       strings.TrimSpace(scenario.Priority),
		DependencyMode: strings.TrimSpace(scenario.DependencyMode),
		CollectionType: strings.TrimSpace(scenario.CollectionType),
		Status:         RunStatusRunning,
		EntryType:      strings.TrimSpace(scenario.Entry.Type),
		TargetAgent:    strings.TrimSpace(scenario.Entry.ToAgent),
		TraceID:        strings.TrimSpace(traceID),
		TaskID:         strings.TrimSpace(taskID),
		StartedAt:      now,
		UpdatedAt:      now,
		Result: TestRunResult{
			Status: RunStatusRunning,
		},
	}
}

func (r *TestRun) beginStep(name, detail string) int {
	step := TestStepResult{
		Name:      name,
		Status:    StepStatusRunning,
		Detail:    strings.TrimSpace(detail),
		StartedAt: time.Now(),
	}
	r.Steps = append(r.Steps, step)
	r.UpdatedAt = time.Now()
	return len(r.Steps) - 1
}

func (r *TestRun) finishStep(index int, status, detail string, data map[string]any) {
	if index < 0 || index >= len(r.Steps) {
		return
	}
	now := time.Now()
	r.Steps[index].Status = strings.TrimSpace(status)
	if strings.TrimSpace(detail) != "" {
		r.Steps[index].Detail = strings.TrimSpace(detail)
	}
	r.Steps[index].FinishedAt = &now
	if len(data) > 0 {
		r.Steps[index].Data = data
	}
	r.UpdatedAt = now
}

func (r *TestRun) updateStep(index int, detail string, data map[string]any) {
	if index < 0 || index >= len(r.Steps) {
		return
	}
	now := time.Now()
	if strings.TrimSpace(detail) != "" {
		r.Steps[index].Detail = strings.TrimSpace(detail)
	}
	if data != nil {
		r.Steps[index].Data = data
	}
	r.UpdatedAt = now
}

func (r *TestRun) appendObservedMessage(msg *uap.Message) {
	if msg == nil {
		return
	}
	r.ObservedMessages = append(r.ObservedMessages, ObservedMessage{
		Type:    msg.Type,
		ID:      msg.ID,
		From:    msg.From,
		To:      msg.To,
		Payload: msg.Payload,
		Ts:      msg.Ts,
	})
	r.UpdatedAt = time.Now()
}

func (r *TestRun) finish(status string) {
	now := time.Now()
	r.Status = status
	r.Result.Status = status
	r.FinishedAt = &now
	r.UpdatedAt = now
}

func (s *TestScenario) timeoutOrDefault(fallback int) time.Duration {
	timeout := s.Assertions.TimeoutSec
	if timeout <= 0 {
		timeout = fallback
	}
	if timeout <= 0 {
		timeout = 20
	}
	return time.Duration(timeout) * time.Second
}

func (s *TestScenario) resolvedTaskID() string {
	if s.Entry.Task == nil {
		return ""
	}
	return strings.TrimSpace(s.Entry.Task.TaskID)
}

func (s *TestScenario) directMessageExpectation() string {
	if strings.TrimSpace(s.Assertions.ExpectMessageType) != "" {
		return strings.TrimSpace(s.Assertions.ExpectMessageType)
	}
	switch strings.TrimSpace(s.Entry.Type) {
	case EntryTypeToolCall:
		return uap.MsgToolResult
	case EntryTypeTaskAssign:
		return uap.MsgTaskComplete
	default:
		return ""
	}
}

func scenarioHintText(s *TestScenario) string {
	if s == nil {
		return ""
	}
	switch strings.TrimSpace(s.Entry.Type) {
	case EntryTypeNotify:
		if s.Entry.Notify != nil {
			return strings.TrimSpace(s.Entry.Notify.Content)
		}
	case EntryTypeToolCall:
		if s.Entry.Tool != nil {
			return strings.TrimSpace(s.Entry.Tool.ToolName)
		}
	case EntryTypeTaskAssign:
		if s.Entry.Task != nil {
			return strings.TrimSpace(string(s.Entry.Task.Payload))
		}
	}
	return ""
}

func (s *TestScenario) requiredAgents() []string {
	if s == nil {
		return nil
	}
	seen := make(map[string]bool)
	var agents []string
	appendIfMissing := func(agentID string) {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" || seen[agentID] {
			return
		}
		seen[agentID] = true
		agents = append(agents, agentID)
	}
	appendIfMissing(s.Entry.ToAgent)
	for _, agentID := range s.Assertions.RequireAgents {
		appendIfMissing(agentID)
	}
	return agents
}
