package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"uap"
)

type loadedSuite struct {
	path  string
	suite *TestSuite
}

func (r *Runner) RunEvaluation(ctx context.Context) (*FinalEvaluationReport, error) {
	if err := r.Connect(ctx); err != nil {
		return nil, err
	}

	evaluationID := "evaluation-" + uap.NewMsgID()
	startedAt := time.Now()

	health, agents, err := r.captureAvailability()
	if err != nil {
		return nil, err
	}

	plan := &EvaluationPlan{
		RunID:        evaluationID,
		Title:        "test-agent-system-evaluation",
		StartedAt:    startedAt,
		Health:       health,
		OnlineAgents: agents,
	}
	_ = r.store.SaveAvailability(evaluationID, health, agents)
	r.publishEvent(RunnerEvent{
		Type:         RunnerEventEvaluationStarted,
		Mode:         "evaluation",
		EvaluationID: evaluationID,
		Health:       clonePointer(health),
		OnlineAgents: cloneSlice(agents),
		Message:      "evaluation started",
	})

	staticSuites, err := LoadSuitesFromDir(r.cfg.StaticSuiteDir)
	if err != nil {
		return nil, err
	}
	plan.StaticCollections = buildCollectionPlans(CollectionTypeStatic, staticSuites, indexAgentsByID(agents))
	r.saveEvaluationPlan(evaluationID, plan)

	staticReport, err := r.runCollection(ctx, evaluationID, CollectionTypeStatic, "Static Evaluation Collection", staticSuites, nil, agents)
	if err != nil {
		return nil, err
	}
	r.saveCollectionReport(staticReport)

	dynamicReport := newCollectionReport(evaluationID, CollectionTypeDynamic, "Dynamic Evaluation Collection")
	dynamicSuite, plannerRun, plannerSkipped, err := r.buildDynamicSuite(ctx, evaluationID, agents, staticReport)
	if plannerRun != nil {
		plan.DynamicPlannerRunID = plannerRun.RunID
	}
	plannerFailed := plannerRun != nil && !plannerRun.Result.Success
	if plannerFailed {
		dynamicReport.Runs = append(dynamicReport.Runs, plannerRun)
		dynamicReport.ExecutedScenarios++
		dynamicReport.FailedScenarios++
		dynamicReport.TotalScenarios++
		plannerSkipped = nil
	}
	if len(plannerSkipped) > 0 {
		dynamicReport.Skipped = append(dynamicReport.Skipped, plannerSkipped...)
		dynamicReport.SkippedScenarios += len(plannerSkipped)
		dynamicReport.TotalScenarios += len(plannerSkipped)
	}
	if err != nil {
		plan.Notes = append(plan.Notes, "dynamic plan generation failed: "+err.Error())
	} else if dynamicSuite != nil {
		dynamicPlan := buildCollectionPlan(CollectionTypeDynamic, *dynamicSuite, indexAgentsByID(agents))
		plan.DynamicCollection = &dynamicPlan
		dynamicReport, err = r.runCollection(ctx, evaluationID, CollectionTypeDynamic, "Dynamic Evaluation Collection", []loadedSuite{*dynamicSuite}, plannerSkipped, agents)
		if err != nil {
			return nil, err
		}
		r.saveCollectionReport(dynamicReport)
	} else {
		if plannerFailed {
			plan.Notes = append(plan.Notes, "dynamic plan execution failed")
		} else if len(plan.Notes) == 0 {
			plan.Notes = append(plan.Notes, "dynamic collection skipped")
		}
		finalizeCollectionReport(dynamicReport)
		r.saveCollectionReport(dynamicReport)
	}
	r.saveEvaluationPlan(evaluationID, plan)

	final := buildFinalEvaluationReport(r.cfg, startedAt, health, agents, plan, staticReport, dynamicReport)
	r.saveFinalReport(final)
	return final, nil
}

func (r *Runner) runCollection(ctx context.Context, evaluationID, collectionType, title string, suites []loadedSuite, preSkipped []SkippedScenario, onlineAgents []GatewayAgentSnapshot) (*SystemEvaluationReport, error) {
	report := newCollectionReport(evaluationID, collectionType, title)
	report.Skipped = append(report.Skipped, preSkipped...)
	report.SkippedScenarios += len(preSkipped)
	report.TotalScenarios += len(preSkipped)

	onlineIndex := indexAgentsByID(onlineAgents)
	var denyKeywords []string
	if collectionType == CollectionTypeDynamic {
		denyKeywords = r.cfg.Dynamic.DenyToolKeywords
	}
	for _, item := range suites {
		if item.suite == nil {
			continue
		}
		report.SourceFiles = appendIfMissingString(report.SourceFiles, item.path)
		for i := range item.suite.Scenarios {
			scenario := item.suite.Scenarios[i]
			normalizeScenarioAgentRefs(&scenario, onlineIndex)
			scenario.CollectionType = collectionType
			scenario.Source = firstNonEmpty(strings.TrimSpace(scenario.Source), item.path)
			report.TotalScenarios++

			if skipReason := scenarioSkipReason(&scenario, onlineIndex, denyKeywords); skipReason != "" {
				report.Skipped = append(report.Skipped, SkippedScenario{
					SuiteID:        item.suite.ID,
					ScenarioID:     scenario.ID,
					Title:          scenario.Title,
					CollectionType: collectionType,
					TargetAgent:    scenario.Entry.ToAgent,
					RequiredAgents: scenario.requiredAgents(),
					Reason:         skipReason,
				})
				report.SkippedScenarios++
				r.saveCollectionReport(report)
				continue
			}

			run, err := r.runScenario(ctx, item.suite.ID, &scenario, runScenarioOptions{
				EvaluationID:   evaluationID,
				CollectionType: collectionType,
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
			r.saveCollectionReport(report)
		}
	}

	finalizeCollectionReport(report)
	return report, nil
}

func (r *Runner) buildDynamicSuite(ctx context.Context, evaluationID string, agents []GatewayAgentSnapshot, staticReport *SystemEvaluationReport) (*loadedSuite, *TestRun, []SkippedScenario, error) {
	if !r.cfg.Dynamic.Enabled {
		return nil, nil, []SkippedScenario{{
			ScenarioID:     "dynamic-plan-generator",
			Title:          "dynamic plan generator",
			CollectionType: CollectionTypeDynamic,
			Reason:         "dynamic evaluation disabled by config",
		}}, nil
	}

	onlineIndex := indexAgentsByID(agents)
	generatorAgent := resolveOnlineAgentRef(r.cfg.Dynamic.GeneratorAgent, onlineIndex)
	if _, ok := onlineIndex[generatorAgent]; !ok {
		return nil, nil, []SkippedScenario{{
			ScenarioID:     "dynamic-plan-generator",
			Title:          "dynamic plan generator",
			CollectionType: CollectionTypeDynamic,
			TargetAgent:    r.cfg.Dynamic.GeneratorAgent,
			Reason:         "dynamic generator agent is offline",
		}}, nil
	}

	plannerScenario := r.buildDynamicPlannerScenario(agents, staticReport)
	plannerScenario.Entry.ToAgent = generatorAgent
	plannerRun, err := r.runScenario(ctx, "__dynamic_plan__", plannerScenario, runScenarioOptions{
		EvaluationID:   evaluationID,
		CollectionType: CollectionTypeDynamicPlan,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if !plannerRun.Result.Success {
		return nil, plannerRun, []SkippedScenario{{
			ScenarioID:     plannerScenario.ID,
			Title:          plannerScenario.Title,
			CollectionType: CollectionTypeDynamic,
			TargetAgent:    plannerScenario.Entry.ToAgent,
			Reason:         firstNonEmpty(plannerRun.Result.FinalError, plannerRun.Result.FinalStatus, "dynamic plan task failed"),
		}}, nil
	}

	suite, err := decodeGeneratedSuite(plannerRun.Result.FinalResult)
	if err != nil {
		return nil, plannerRun, []SkippedScenario{{
			ScenarioID:     plannerScenario.ID,
			Title:          plannerScenario.Title,
			CollectionType: CollectionTypeDynamic,
			TargetAgent:    plannerScenario.Entry.ToAgent,
			Reason:         "dynamic plan parse failed: " + err.Error(),
		}}, nil
	}

	validSuite, skipped := validateGeneratedSuite(r.cfg, suite, plannerRun.RunID, onlineIndex)
	if validSuite == nil || len(validSuite.Scenarios) == 0 {
		return nil, plannerRun, skipped, nil
	}
	return &loadedSuite{
		path:  "generated:" + plannerRun.RunID,
		suite: validSuite,
	}, plannerRun, skipped, nil
}

func (r *Runner) buildDynamicPlannerScenario(agents []GatewayAgentSnapshot, staticReport *SystemEvaluationReport) *TestScenario {
	return &TestScenario{
		ID:             "dynamic-plan-generator",
		Title:          "dynamic-plan-generator",
		Description:    "请求 llm-agent 基于在线 agents 生成动态评估集",
		Category:       "planner",
		Priority:       "P1",
		CollectionType: CollectionTypeDynamicPlan,
		Entry: ScenarioEntry{
			Type:    EntryTypeTaskAssign,
			ToAgent: r.cfg.Dynamic.GeneratorAgent,
			Task: &TaskAssignEntry{
				Payload: mustMarshal(map[string]any{
					"task_type": "llm_request",
					"account":   r.cfg.Dynamic.Account,
					"no_tools":  true,
					"messages": []map[string]string{
						{
							"role":    "system",
							"content": dynamicPlannerSystemPrompt(r.cfg.Dynamic.MaxScenarios),
						},
						{
							"role":    "user",
							"content": dynamicPlannerUserPrompt(agents, staticReport),
						},
					},
				}),
			},
		},
		Assertions: TestAssertions{
			TimeoutSec:        r.cfg.Dynamic.TimeoutSec,
			ExpectMessageType: uap.MsgTaskComplete,
			ExpectTaskStatus:  "success",
			RequireAgents:     []string{r.cfg.Dynamic.GeneratorAgent},
			RequireMsgTypes:   []string{uap.MsgTaskAssign, uap.MsgTaskComplete},
			MinTraceEvents:    2,
		},
	}
}

func LoadSuitesFromDir(dir string) ([]loadedSuite, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read suite dir: %w", err)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var suites []loadedSuite
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		suite, err := LoadSuite(path)
		if err != nil {
			return nil, fmt.Errorf("load suite %s: %w", path, err)
		}
		suite.Collection = firstNonEmpty(strings.TrimSpace(suite.Collection), CollectionTypeStatic)
		suite.Source = firstNonEmpty(strings.TrimSpace(suite.Source), path)
		for i := range suite.Scenarios {
			suite.Scenarios[i].CollectionType = firstNonEmpty(strings.TrimSpace(suite.Scenarios[i].CollectionType), suite.Collection)
			suite.Scenarios[i].Source = firstNonEmpty(strings.TrimSpace(suite.Scenarios[i].Source), path)
		}
		suites = append(suites, loadedSuite{path: path, suite: suite})
	}
	return suites, nil
}

func buildCollectionPlans(collectionType string, suites []loadedSuite, online map[string]GatewayAgentSnapshot) []CollectionExecutionPlan {
	plans := make([]CollectionExecutionPlan, 0, len(suites))
	for _, item := range suites {
		plans = append(plans, buildCollectionPlan(collectionType, item, online))
	}
	return plans
}

func buildCollectionPlan(collectionType string, item loadedSuite, online map[string]GatewayAgentSnapshot) CollectionExecutionPlan {
	plan := CollectionExecutionPlan{
		ID:             firstNonEmpty(item.suite.ID, filepath.Base(item.path)),
		Title:          firstNonEmpty(item.suite.Title, item.suite.ID),
		CollectionType: collectionType,
		Source:         item.path,
		GeneratedBy:    item.suite.GeneratedBy,
	}
	for _, scenario := range item.suite.Scenarios {
		normalizeScenarioAgentRefs(&scenario, online)
		skipReason := scenarioSkipReason(&scenario, online, nil)
		plan.Items = append(plan.Items, ScenarioPlanItem{
			SuiteID:        item.suite.ID,
			ScenarioID:     scenario.ID,
			Title:          scenario.Title,
			CollectionType: collectionType,
			Source:         item.path,
			EntryType:      scenario.Entry.Type,
			TargetAgent:    scenario.Entry.ToAgent,
			RequiredAgents: scenario.requiredAgents(),
			Tags:           append([]string(nil), scenario.Tags...),
			Priority:       scenario.Priority,
			Eligible:       skipReason == "",
			SkipReason:     skipReason,
		})
	}
	plan.ScenarioCount = len(plan.Items)
	return plan
}

func newCollectionReport(evaluationID, collectionType, title string) *SystemEvaluationReport {
	return &SystemEvaluationReport{
		RunID:          firstNonEmpty(evaluationID, collectionType+"-"+uap.NewMsgID()),
		EvaluationID:   evaluationID,
		Title:          title,
		CollectionType: collectionType,
		Status:         RunStatusRunning,
		StartedAt:      time.Now(),
	}
}

func finalizeCollectionReport(report *SystemEvaluationReport) {
	if report == nil {
		return
	}
	report.FinishedAt = time.Now()
	totalScore := 0
	for _, run := range report.Runs {
		totalScore += run.Result.Scores.Total
	}
	if report.ExecutedScenarios > 0 {
		report.AverageScore = totalScore / report.ExecutedScenarios
	}
	report.DimensionScores = aggregateDimensionScores(report.Runs)
	switch {
	case report.FailedScenarios > 0:
		report.Status = RunStatusFailed
	case report.ExecutedScenarios > 0:
		report.Status = RunStatusPassed
	default:
		report.Status = RunStatusSkipped
	}
}

func aggregateDimensionScores(runs []*TestRun) []DimensionScore {
	type aggregate struct {
		total  int
		count  int
		passed int
	}
	names := []struct {
		key   string
		value func(run *TestRun) int
	}{
		{key: "completion", value: func(run *TestRun) int { return run.Result.Scores.CompletionScore }},
		{key: "routing", value: func(run *TestRun) int { return run.Result.Scores.RoutingScore }},
		{key: "tool_usage", value: func(run *TestRun) int { return run.Result.Scores.ToolUsageScore }},
		{key: "recovery", value: func(run *TestRun) int { return run.Result.Scores.RecoveryScore }},
		{key: "final_answer", value: func(run *TestRun) int { return run.Result.Scores.FinalAnswerScore }},
	}
	agg := make(map[string]*aggregate, len(names))
	for _, item := range names {
		agg[item.key] = &aggregate{}
	}
	for _, run := range runs {
		for _, item := range names {
			value := item.value(run)
			agg[item.key].total += value
			agg[item.key].count++
			if value >= 60 {
				agg[item.key].passed++
			}
		}
	}
	scores := make([]DimensionScore, 0, len(names))
	for _, item := range names {
		entry := agg[item.key]
		score := DimensionScore{
			Name:       item.key,
			PassedRuns: entry.passed,
			TotalRuns:  entry.count,
		}
		if entry.count > 0 {
			score.AverageScore = entry.total / entry.count
		}
		scores = append(scores, score)
	}
	return scores
}

func buildFinalEvaluationReport(cfg *Config, startedAt time.Time, health *GatewayHealthSnapshot, agents []GatewayAgentSnapshot, plan *EvaluationPlan, staticReport, dynamicReport *SystemEvaluationReport) *FinalEvaluationReport {
	finishedAt := time.Now()
	final := &FinalEvaluationReport{
		RunID:         plan.RunID,
		Title:         plan.Title,
		Status:        RunStatusPassed,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		Health:        health,
		OnlineAgents:  agents,
		ExecutionPlan: plan,
		StaticReport:  staticReport,
		DynamicReport: dynamicReport,
	}

	var allRuns []*TestRun
	if staticReport != nil {
		allRuns = append(allRuns, staticReport.Runs...)
	}
	if dynamicReport != nil {
		allRuns = append(allRuns, dynamicReport.Runs...)
	}
	final.DimensionScores = aggregateDimensionScores(allRuns)
	final.AgentEvaluations = buildAgentEvaluations(agents, allRuns)
	final.OverallScore = weightedOverallScore(cfg, staticReport, dynamicReport)
	final.Findings = buildFindings(staticReport, dynamicReport, plan)

	switch {
	case (staticReport != nil && staticReport.Status == RunStatusFailed) || (dynamicReport != nil && dynamicReport.Status == RunStatusFailed):
		final.Status = RunStatusFailed
	case staticReport != nil && staticReport.Status == RunStatusSkipped && (dynamicReport == nil || dynamicReport.Status == RunStatusSkipped):
		final.Status = RunStatusSkipped
	}
	return final
}

func weightedOverallScore(cfg *Config, staticReport, dynamicReport *SystemEvaluationReport) int {
	totalWeight := 0
	totalScore := 0
	if staticReport != nil && staticReport.ExecutedScenarios > 0 && cfg.Report.StaticWeight > 0 {
		totalWeight += cfg.Report.StaticWeight
		totalScore += staticReport.AverageScore * cfg.Report.StaticWeight
	}
	if dynamicReport != nil && dynamicReport.ExecutedScenarios > 0 && cfg.Report.DynamicWeight > 0 {
		totalWeight += cfg.Report.DynamicWeight
		totalScore += dynamicReport.AverageScore * cfg.Report.DynamicWeight
	}
	if totalWeight == 0 {
		return 0
	}
	return totalScore / totalWeight
}

func buildAgentEvaluations(agents []GatewayAgentSnapshot, runs []*TestRun) []AgentEvaluation {
	index := make(map[string]*AgentEvaluation, len(agents))
	scoreSum := make(map[string]int)
	for _, agent := range agents {
		copyAgent := agent
		index[agent.AgentID] = &AgentEvaluation{
			AgentID:   copyAgent.AgentID,
			AgentType: copyAgent.AgentType,
			Online:    true,
		}
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		target := strings.TrimSpace(run.TargetAgent)
		if target != "" {
			entry := index[target]
			if entry == nil {
				entry = &AgentEvaluation{AgentID: target}
				index[target] = entry
			}
			entry.TargetedRuns++
			scoreSum[target] += run.Result.Scores.Total
			if run.Result.Success {
				entry.PassedRuns++
			} else {
				entry.FailedRuns++
			}
		}
		if run.Trace == nil {
			continue
		}
		for _, agentID := range run.Trace.Agents {
			entry := index[agentID]
			if entry == nil {
				entry = &AgentEvaluation{AgentID: agentID}
				index[agentID] = entry
			}
			entry.ObservedRuns++
			entry.LastObservedTrace = run.TraceID
		}
	}
	items := make([]AgentEvaluation, 0, len(index))
	for agentID, item := range index {
		if item.TargetedRuns > 0 {
			item.AverageScore = scoreSum[agentID] / item.TargetedRuns
		}
		items = append(items, *item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].AgentID < items[j].AgentID
	})
	return items
}

func buildFindings(staticReport, dynamicReport *SystemEvaluationReport, plan *EvaluationPlan) []string {
	var findings []string
	if staticReport != nil {
		findings = append(findings, fmt.Sprintf("static collection: executed=%d passed=%d failed=%d skipped=%d avg=%d",
			staticReport.ExecutedScenarios, staticReport.PassedScenarios, staticReport.FailedScenarios, staticReport.SkippedScenarios, staticReport.AverageScore))
	}
	if dynamicReport != nil {
		findings = append(findings, fmt.Sprintf("dynamic collection: executed=%d passed=%d failed=%d skipped=%d avg=%d",
			dynamicReport.ExecutedScenarios, dynamicReport.PassedScenarios, dynamicReport.FailedScenarios, dynamicReport.SkippedScenarios, dynamicReport.AverageScore))
	}
	if plan != nil && plan.DynamicPlannerRunID == "" {
		findings = append(findings, "dynamic planner did not produce an executable suite")
	}
	if plan != nil {
		for _, note := range plan.Notes {
			if strings.TrimSpace(note) != "" {
				findings = append(findings, strings.TrimSpace(note))
			}
		}
	}
	return findings
}

func decodeGeneratedSuite(raw string) (*TestSuite, error) {
	doc := extractJSONDocument(raw)
	var suite TestSuite
	if err := json.Unmarshal([]byte(doc), &suite); err != nil {
		return nil, err
	}
	if strings.TrimSpace(suite.ID) == "" {
		suite.ID = "dynamic-evaluation"
	}
	if strings.TrimSpace(suite.Title) == "" {
		suite.Title = suite.ID
	}
	return &suite, nil
}

func extractJSONDocument(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				continue
			}
			filtered = append(filtered, line)
		}
		trimmed = strings.TrimSpace(strings.Join(filtered, "\n"))
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		candidate := strings.TrimSpace(trimmed[start : end+1])
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}
	return trimmed
}

func validateGeneratedSuite(cfg *Config, suite *TestSuite, plannerRunID string, online map[string]GatewayAgentSnapshot) (*TestSuite, []SkippedScenario) {
	if suite == nil {
		return nil, nil
	}
	valid := &TestSuite{
		ID:          firstNonEmpty(strings.TrimSpace(suite.ID), "dynamic-evaluation"),
		Title:       firstNonEmpty(strings.TrimSpace(suite.Title), "Dynamic Evaluation Collection"),
		Description: strings.TrimSpace(suite.Description),
		Collection:  CollectionTypeDynamic,
		Source:      "generated:" + plannerRunID,
		GeneratedBy: "llm-agent",
	}
	var skipped []SkippedScenario
	for i, scenario := range suite.Scenarios {
		if i >= cfg.Dynamic.MaxScenarios {
			skipped = append(skipped, SkippedScenario{
				SuiteID:        valid.ID,
				ScenarioID:     firstNonEmpty(strings.TrimSpace(scenario.ID), fmt.Sprintf("dynamic-%02d", i+1)),
				Title:          firstNonEmpty(strings.TrimSpace(scenario.Title), fmt.Sprintf("dynamic-%02d", i+1)),
				CollectionType: CollectionTypeDynamic,
				TargetAgent:    scenario.Entry.ToAgent,
				RequiredAgents: scenario.requiredAgents(),
				Reason:         "exceeds dynamic max_scenarios limit",
			})
			continue
		}
		if strings.TrimSpace(scenario.ID) == "" {
			scenario.ID = fmt.Sprintf("dynamic-%02d", i+1)
		}
		if strings.TrimSpace(scenario.Title) == "" {
			scenario.Title = scenario.ID
		}
		scenario.CollectionType = CollectionTypeDynamic
		scenario.Source = valid.Source
		scenario.GeneratedBy = valid.GeneratedBy
		if strings.TrimSpace(scenario.Category) == "" {
			scenario.Category = "dynamic"
		}
		normalizeScenarioAgentRefs(&scenario, online)
		if len(scenario.Assertions.RequireAgents) == 0 {
			scenario.Assertions.RequireAgents = scenario.requiredAgents()
		}
		if len(scenario.Assertions.RequireMsgTypes) == 0 {
			switch strings.TrimSpace(scenario.Entry.Type) {
			case EntryTypeToolCall:
				scenario.Assertions.RequireMsgTypes = []string{uap.MsgToolCall, uap.MsgToolResult}
			case EntryTypeTaskAssign:
				scenario.Assertions.RequireMsgTypes = []string{uap.MsgTaskAssign, uap.MsgTaskComplete}
			case EntryTypeNotify:
				scenario.Assertions.RequireMsgTypes = []string{uap.MsgNotify}
			}
		}
		if scenario.Assertions.TimeoutSec <= 0 {
			scenario.Assertions.TimeoutSec = cfg.Dynamic.TimeoutSec
		}
		if reason := scenarioSkipReason(&scenario, online, cfg.Dynamic.DenyToolKeywords); reason != "" {
			skipped = append(skipped, SkippedScenario{
				SuiteID:        valid.ID,
				ScenarioID:     scenario.ID,
				Title:          scenario.Title,
				CollectionType: CollectionTypeDynamic,
				TargetAgent:    scenario.Entry.ToAgent,
				RequiredAgents: scenario.requiredAgents(),
				Reason:         reason,
			})
			continue
		}
		valid.Scenarios = append(valid.Scenarios, scenario)
	}
	return valid, skipped
}

func scenarioSkipReason(scenario *TestScenario, online map[string]GatewayAgentSnapshot, denyKeywords []string) string {
	if scenario == nil {
		return "scenario is nil"
	}
	if strings.TrimSpace(scenario.Entry.Type) == "" {
		return "entry.type is required"
	}
	target := strings.TrimSpace(scenario.Entry.ToAgent)
	if target == "" {
		return "entry.to_agent is required"
	}
	target = resolveOnlineAgentRef(target, online)
	if _, ok := online[target]; !ok {
		return "target agent is offline"
	}
	for _, agentID := range scenario.requiredAgents() {
		if _, ok := online[resolveOnlineAgentRef(agentID, online)]; !ok {
			return fmt.Sprintf("required agent is offline: %s", agentID)
		}
	}
	switch strings.TrimSpace(scenario.Entry.Type) {
	case EntryTypeToolCall:
		if scenario.Entry.Tool == nil {
			return "tool entry is required"
		}
		toolName := strings.TrimSpace(scenario.Entry.Tool.ToolName)
		if toolName == "" {
			return "tool_name is required"
		}
		for _, keyword := range denyKeywords {
			if keyword != "" && strings.Contains(strings.ToLower(toolName), strings.ToLower(keyword)) {
				return "tool blocked by deny_tool_keywords"
			}
		}
		if agent, ok := online[target]; ok && len(agent.Tools) > 0 && !containsString(agent.Tools, toolName) {
			return fmt.Sprintf("tool not exposed by target agent: %s", toolName)
		}
	case EntryTypeTaskAssign:
		if scenario.Entry.Task == nil || len(scenario.Entry.Task.Payload) == 0 {
			return "task payload is required"
		}
	case EntryTypeNotify:
		if scenario.Entry.Notify == nil {
			return "notify entry is required"
		}
	default:
		return "unsupported entry type"
	}
	return ""
}

func dynamicPlannerSystemPrompt(maxScenarios int) string {
	return fmt.Sprintf(`你是 test-agent 的动态评估规划器。你的任务是输出一个严格 JSON 对象，用于描述最多 %d 个动态测试场景。

输出要求：
1. 只能输出 JSON，不要 markdown，不要代码块，不要解释。
2. 顶层结构必须是 {"id":"...","title":"...","description":"...","scenarios":[...]}。
3. 每个场景必须包含 id,title,description,category,priority,tags,entry,assertions。
4. 只能使用当前在线 agent；禁止引用未在线 agent。
5. 场景必须尽量选择低风险、可重复、可观测的路径，避免 destructive 行为。
6. 如果使用 tool_call，优先选择只读、查询、校验或显式失败预期的工具。
7. assertions 里必须尽量给出 expect_message_type、require_agents、require_msg_types、min_trace_events；如果是 task_assign，尽量给 expected_path。
8. to_agent 必须是明确 agent_id，entry.type 只能是 notify、task_assign、tool_call。`, maxScenarios)
}

func dynamicPlannerUserPrompt(agents []GatewayAgentSnapshot, staticReport *SystemEvaluationReport) string {
	var sb strings.Builder
	sb.WriteString("以下是当前在线 agents 与工具：\n")
	sort.SliceStable(agents, func(i, j int) bool {
		return agents[i].AgentID < agents[j].AgentID
	})
	for _, agent := range agents {
		sb.WriteString(fmt.Sprintf("- agent=%s type=%s tools=%s\n",
			agent.AgentID,
			firstNonEmpty(agent.AgentType, "-"),
			strings.Join(agent.Tools, ",")))
	}
	if staticReport != nil {
		sb.WriteString("\n静态评估结果摘要：\n")
		sb.WriteString(fmt.Sprintf("- executed=%d passed=%d failed=%d skipped=%d avg=%d\n",
			staticReport.ExecutedScenarios,
			staticReport.PassedScenarios,
			staticReport.FailedScenarios,
			staticReport.SkippedScenarios,
			staticReport.AverageScore))
		for _, run := range staticReport.Runs {
			sb.WriteString(fmt.Sprintf("- scenario=%s status=%s target=%s score=%d final=%s\n",
				run.ScenarioID, run.Status, run.TargetAgent, run.Result.Scores.Total,
				firstNonEmpty(run.Result.FinalStatus, run.Result.FinalError)))
		}
	}
	sb.WriteString("\n请优先补足静态评估未覆盖的在线 agent，生成协同链路和系统能力测试场景。")
	return sb.String()
}

func indexAgentsByID(items []GatewayAgentSnapshot) map[string]GatewayAgentSnapshot {
	index := make(map[string]GatewayAgentSnapshot, len(items))
	for _, item := range items {
		index[item.AgentID] = item
	}
	return index
}

func appendIfMissingString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return items
		}
	}
	return append(items, value)
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}
