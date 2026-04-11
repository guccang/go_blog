package main

import "encoding/json"

const (
	RunnerEventRunUpdated        = "run_updated"
	RunnerEventSuiteReport       = "suite_report"
	RunnerEventCollectionReport  = "collection_report"
	RunnerEventEvaluationPlan    = "evaluation_plan"
	RunnerEventEvaluationStarted = "evaluation_started"
	RunnerEventEvaluationDone    = "evaluation_done"
	RunnerEventRunnerError       = "runner_error"
)

// RunnerEvent 实时推送给 Web 控制台的运行期事件。
type RunnerEvent struct {
	Type             string                  `json:"type"`
	Mode             string                  `json:"mode,omitempty"`
	Message          string                  `json:"message,omitempty"`
	EvaluationID     string                  `json:"evaluation_id,omitempty"`
	Run              *TestRun                `json:"run,omitempty"`
	SuiteReport      *SystemEvaluationReport `json:"suite_report,omitempty"`
	CollectionReport *SystemEvaluationReport `json:"collection_report,omitempty"`
	Plan             *EvaluationPlan         `json:"plan,omitempty"`
	FinalReport      *FinalEvaluationReport  `json:"final_report,omitempty"`
	Health           *GatewayHealthSnapshot  `json:"health,omitempty"`
	OnlineAgents     []GatewayAgentSnapshot  `json:"online_agents,omitempty"`
	AvailableSuites  []DashboardSuiteOption  `json:"available_suites,omitempty"`
}

// RunnerEventSink 接收 test-agent 的运行期事件。
type RunnerEventSink interface {
	HandleRunnerEvent(event RunnerEvent)
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var cloned T
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil
	}
	return &cloned
}

func cloneSlice[T any](items []T) []T {
	if len(items) == 0 {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	var cloned []T
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil
	}
	return cloned
}
