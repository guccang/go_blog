package main

import (
	"fmt"
	"sort"
	"strings"
)

func buildTraceSummary(run *TestRun) string {
	if run == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# 测试执行记录\n\n")
	sb.WriteString("## 基本信息\n")
	sb.WriteString(fmt.Sprintf("- run_id: `%s`\n", run.RunID))
	if run.EvaluationID != "" {
		sb.WriteString(fmt.Sprintf("- evaluation_id: `%s`\n", run.EvaluationID))
	}
	sb.WriteString(fmt.Sprintf("- scenario_id: `%s`\n", run.ScenarioID))
	sb.WriteString(fmt.Sprintf("- status: `%s`\n", run.Status))
	if run.CollectionType != "" {
		sb.WriteString(fmt.Sprintf("- collection_type: `%s`\n", run.CollectionType))
	}
	sb.WriteString(fmt.Sprintf("- entry_type: `%s`\n", run.EntryType))
	sb.WriteString(fmt.Sprintf("- target_agent: `%s`\n", run.TargetAgent))
	sb.WriteString(fmt.Sprintf("- trace_id: `%s`\n", run.TraceID))
	if run.TaskID != "" {
		sb.WriteString(fmt.Sprintf("- task_id: `%s`\n", run.TaskID))
	}
	sb.WriteString(fmt.Sprintf("- started_at: `%s`\n", run.StartedAt.Format("2006-01-02 15:04:05")))
	if run.FinishedAt != nil {
		sb.WriteString(fmt.Sprintf("- finished_at: `%s`\n", run.FinishedAt.Format("2006-01-02 15:04:05")))
	}
	sb.WriteString("\n")

	if len(run.Steps) > 0 {
		sb.WriteString("## 执行步骤\n")
		for idx, step := range run.Steps {
			sb.WriteString(fmt.Sprintf("%d. `%s` - `%s`", idx+1, step.Name, step.Status))
			if step.Detail != "" {
				sb.WriteString(": ")
				sb.WriteString(step.Detail)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(run.Result.Outcomes) > 0 {
		sb.WriteString("## 断言结果\n")
		for _, outcome := range run.Result.Outcomes {
			flag := "PASS"
			if !outcome.Success {
				flag = "FAIL"
			}
			sb.WriteString(fmt.Sprintf("- %s `%s`", flag, outcome.Name))
			if outcome.Detail != "" {
				sb.WriteString(": ")
				sb.WriteString(outcome.Detail)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 评分\n")
	sb.WriteString(fmt.Sprintf("- completion: %d\n", run.Result.Scores.CompletionScore))
	sb.WriteString(fmt.Sprintf("- routing: %d\n", run.Result.Scores.RoutingScore))
	sb.WriteString(fmt.Sprintf("- tool_usage: %d\n", run.Result.Scores.ToolUsageScore))
	sb.WriteString(fmt.Sprintf("- recovery: %d\n", run.Result.Scores.RecoveryScore))
	sb.WriteString(fmt.Sprintf("- final_answer: %d\n", run.Result.Scores.FinalAnswerScore))
	sb.WriteString(fmt.Sprintf("- total: %d\n\n", run.Result.Scores.Total))

	if run.Trace != nil {
		sb.WriteString("## Gateway Trace\n")
		sb.WriteString(fmt.Sprintf("- trace_status: `%s`\n", run.Trace.Status))
		sb.WriteString(fmt.Sprintf("- duration_ms: `%d`\n", run.Trace.DurationMs))
		if len(run.Trace.Agents) > 0 {
			sb.WriteString(fmt.Sprintf("- agents: %s\n", strings.Join(run.Trace.Agents, " -> ")))
		}
		if len(run.Trace.MessageTypes) > 0 {
			sb.WriteString(fmt.Sprintf("- msg_types: %s\n", strings.Join(run.Trace.MessageTypes, ", ")))
		}
		sb.WriteString("\n")
		if len(run.Trace.Events) > 0 {
			sb.WriteString("| Seq | Kind | MsgType | From | To | Summary |\n")
			sb.WriteString("| --- | --- | --- | --- | --- | --- |\n")
			for _, event := range run.Trace.Events {
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s |\n",
					event.Seq, safeCell(event.Kind), safeCell(event.MsgType), safeCell(event.From), safeCell(event.To), safeCell(event.PayloadSummary)))
			}
			sb.WriteString("\n")
		}
	}

	if run.LLMTrace != nil {
		sb.WriteString("## LLM Trace\n")
		sb.WriteString(fmt.Sprintf("- file: `%s`\n", run.LLMTrace.FilePath))
		if run.LLMTrace.RootID != "" {
			sb.WriteString(fmt.Sprintf("- root_id: `%s`\n", run.LLMTrace.RootID))
		}
		if run.LLMTrace.SessionID != "" {
			sb.WriteString(fmt.Sprintf("- session_id: `%s`\n", run.LLMTrace.SessionID))
		}
		if run.LLMTrace.Query != "" {
			sb.WriteString(fmt.Sprintf("- query: `%s`\n", run.LLMTrace.Query))
		}
		sb.WriteString("\n")
	}

	if run.Result.FinalResult != "" {
		sb.WriteString("## 最终结果\n")
		sb.WriteString(run.Result.FinalResult)
		sb.WriteString("\n")
	}
	if run.Result.FinalError != "" {
		sb.WriteString("\n## 最终错误\n")
		sb.WriteString(run.Result.FinalError)
		sb.WriteString("\n")
	}
	return sb.String()
}

func buildSuiteSummary(report *SystemEvaluationReport) string {
	if report == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# 套件评估结果\n\n")
	sb.WriteString(fmt.Sprintf("- run_id: `%s`\n", report.RunID))
	sb.WriteString(fmt.Sprintf("- suite_id: `%s`\n", report.SuiteID))
	sb.WriteString(fmt.Sprintf("- status: `%s`\n", report.Status))
	if report.CollectionType != "" {
		sb.WriteString(fmt.Sprintf("- collection_type: `%s`\n", report.CollectionType))
	}
	sb.WriteString(fmt.Sprintf("- total_scenarios: %d\n", report.TotalScenarios))
	if report.ExecutedScenarios > 0 {
		sb.WriteString(fmt.Sprintf("- executed_scenarios: %d\n", report.ExecutedScenarios))
	}
	if report.SkippedScenarios > 0 {
		sb.WriteString(fmt.Sprintf("- skipped_scenarios: %d\n", report.SkippedScenarios))
	}
	sb.WriteString(fmt.Sprintf("- passed_scenarios: %d\n", report.PassedScenarios))
	sb.WriteString(fmt.Sprintf("- failed_scenarios: %d\n", report.FailedScenarios))
	sb.WriteString(fmt.Sprintf("- average_score: %d\n\n", report.AverageScore))
	if len(report.SourceFiles) > 0 {
		sb.WriteString("- source_files:\n")
		for _, source := range report.SourceFiles {
			sb.WriteString(fmt.Sprintf("  - `%s`\n", strings.TrimSpace(source)))
		}
		sb.WriteString("\n")
	}
	if len(report.DimensionScores) > 0 {
		sb.WriteString("## 维度评分\n")
		for _, item := range report.DimensionScores {
			sb.WriteString(fmt.Sprintf("- %s: avg=%d pass=%d/%d\n",
				item.Name, item.AverageScore, item.PassedRuns, item.TotalRuns))
		}
		sb.WriteString("\n")
	}
	if len(report.Skipped) > 0 {
		sb.WriteString("## 跳过场景\n")
		for _, item := range report.Skipped {
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", item.ScenarioID, item.Reason))
		}
		sb.WriteString("\n")
	}
	if len(report.Runs) > 0 {
		sb.WriteString("| Scenario | Status | Score | Final |\n")
		sb.WriteString("| --- | --- | --- | --- |\n")
		runs := append([]*TestRun(nil), report.Runs...)
		sort.SliceStable(runs, func(i, j int) bool {
			return runs[i].ScenarioID < runs[j].ScenarioID
		})
		for _, run := range runs {
			final := firstNonEmpty(run.Result.FinalStatus, run.Result.FinalMessageType, run.Result.FinalError)
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n",
				safeCell(run.ScenarioID), safeCell(run.Status), run.Result.Scores.Total, safeCell(final)))
		}
	}
	return sb.String()
}

func buildFinalSummary(report *FinalEvaluationReport) string {
	if report == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# 系统评估总览\n\n")
	sb.WriteString(fmt.Sprintf("- run_id: `%s`\n", report.RunID))
	sb.WriteString(fmt.Sprintf("- status: `%s`\n", report.Status))
	sb.WriteString(fmt.Sprintf("- overall_score: %d\n", report.OverallScore))
	sb.WriteString(fmt.Sprintf("- started_at: `%s`\n", report.StartedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("- finished_at: `%s`\n\n", report.FinishedAt.Format("2006-01-02 15:04:05")))

	if report.StaticReport != nil {
		sb.WriteString("## 静态评估集\n")
		sb.WriteString(fmt.Sprintf("- status: `%s`\n", report.StaticReport.Status))
		sb.WriteString(fmt.Sprintf("- total: %d\n", report.StaticReport.TotalScenarios))
		sb.WriteString(fmt.Sprintf("- executed: %d\n", report.StaticReport.ExecutedScenarios))
		sb.WriteString(fmt.Sprintf("- passed: %d\n", report.StaticReport.PassedScenarios))
		sb.WriteString(fmt.Sprintf("- failed: %d\n", report.StaticReport.FailedScenarios))
		sb.WriteString(fmt.Sprintf("- avg_score: %d\n\n", report.StaticReport.AverageScore))
	}

	if report.DynamicReport != nil {
		sb.WriteString("## 动态评估集\n")
		sb.WriteString(fmt.Sprintf("- status: `%s`\n", report.DynamicReport.Status))
		sb.WriteString(fmt.Sprintf("- total: %d\n", report.DynamicReport.TotalScenarios))
		sb.WriteString(fmt.Sprintf("- executed: %d\n", report.DynamicReport.ExecutedScenarios))
		sb.WriteString(fmt.Sprintf("- skipped: %d\n", report.DynamicReport.SkippedScenarios))
		sb.WriteString(fmt.Sprintf("- passed: %d\n", report.DynamicReport.PassedScenarios))
		sb.WriteString(fmt.Sprintf("- failed: %d\n", report.DynamicReport.FailedScenarios))
		sb.WriteString(fmt.Sprintf("- avg_score: %d\n\n", report.DynamicReport.AverageScore))
	}

	if len(report.DimensionScores) > 0 {
		sb.WriteString("## 综合维度评分\n")
		for _, item := range report.DimensionScores {
			sb.WriteString(fmt.Sprintf("- %s: avg=%d pass=%d/%d\n",
				item.Name, item.AverageScore, item.PassedRuns, item.TotalRuns))
		}
		sb.WriteString("\n")
	}

	if len(report.AgentEvaluations) > 0 {
		sb.WriteString("## Agent 评估\n")
		for _, item := range report.AgentEvaluations {
			sb.WriteString(fmt.Sprintf("- `%s`: targeted=%d observed=%d passed=%d failed=%d avg=%d\n",
				item.AgentID, item.TargetedRuns, item.ObservedRuns, item.PassedRuns, item.FailedRuns, item.AverageScore))
		}
		sb.WriteString("\n")
	}

	if len(report.Findings) > 0 {
		sb.WriteString("## 关键结论\n")
		for _, finding := range report.Findings {
			sb.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(finding)))
		}
	}
	return sb.String()
}

func safeCell(text string) string {
	return strings.ReplaceAll(strings.TrimSpace(text), "|", "\\|")
}
