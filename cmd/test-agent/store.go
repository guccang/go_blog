package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RunStore 负责把测试过程持续落盘，方便定位执行路径问题。
type RunStore struct {
	baseDir string
	mu      sync.Mutex
}

func NewRunStore(baseDir string) *RunStore {
	return &RunStore{baseDir: baseDir}
}

func (s *RunStore) SaveScenario(run *TestRun, scenario *TestScenario) error {
	if s == nil || run == nil || scenario == nil {
		return nil
	}
	dir, err := s.ensureRunDir(run)
	if err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(dir, "scenario.json"), scenario)
}

func (s *RunStore) SaveRun(run *TestRun) error {
	if s == nil || run == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.ensureRunDir(run)
	if err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(dir, "run.json"), run); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(dir, "timeline.json"), run.Steps); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(dir, "messages.json"), run.ObservedMessages); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(dir, "result.json"), run.Result); err != nil {
		return err
	}
	if run.Trace != nil {
		if err := writeJSONFile(filepath.Join(dir, "gateway_trace.json"), run.Trace); err != nil {
			return err
		}
	}
	if run.LLMTrace != nil {
		if err := writeJSONFile(filepath.Join(dir, "llm_trace.json"), run.LLMTrace); err != nil {
			return err
		}
	}
	if err := writeTextFile(filepath.Join(dir, "trace_summary.md"), buildTraceSummary(run)); err != nil {
		return err
	}
	return nil
}

func (s *RunStore) SaveSuiteReport(report *SystemEvaluationReport) error {
	if s == nil || report == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dayDir := filepath.Join(s.baseDir, time.Now().Format("2006-01-02"), report.RunID)
	if err := os.MkdirAll(dayDir, 0755); err != nil {
		return fmt.Errorf("create suite dir: %w", err)
	}
	if err := writeJSONFile(filepath.Join(dayDir, "suite_report.json"), report); err != nil {
		return err
	}
	return writeTextFile(filepath.Join(dayDir, "suite_report.md"), buildSuiteSummary(report))
}

func (s *RunStore) SaveEvaluationPlan(runID string, plan *EvaluationPlan) error {
	if s == nil || plan == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.ensureEvaluationDir(runID)
	if err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(dir, "execution_plan.json"), plan)
}

func (s *RunStore) SaveAvailability(runID string, health *GatewayHealthSnapshot, agents []GatewayAgentSnapshot) error {
	if s == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.ensureEvaluationDir(runID)
	if err != nil {
		return err
	}
	if health != nil {
		if err := writeJSONFile(filepath.Join(dir, "gateway_health.json"), health); err != nil {
			return err
		}
	}
	return writeJSONFile(filepath.Join(dir, "online_agents.json"), agents)
}

func (s *RunStore) SaveCollectionReport(report *SystemEvaluationReport) error {
	if s == nil || report == nil || strings.TrimSpace(report.EvaluationID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.ensureEvaluationDir(report.EvaluationID)
	if err != nil {
		return err
	}
	collectionID := firstNonEmpty(strings.TrimSpace(report.CollectionType), strings.TrimSpace(report.SuiteID), "collection")
	fileBase := sanitizeFileName(collectionID) + "_report"
	if err := writeJSONFile(filepath.Join(dir, fileBase+".json"), report); err != nil {
		return err
	}
	return writeTextFile(filepath.Join(dir, fileBase+".md"), buildSuiteSummary(report))
}

func (s *RunStore) SaveFinalReport(report *FinalEvaluationReport) error {
	if s == nil || report == nil || strings.TrimSpace(report.RunID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.ensureEvaluationDir(report.RunID)
	if err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(dir, "final_report.json"), report); err != nil {
		return err
	}
	return writeTextFile(filepath.Join(dir, "final_report.md"), buildFinalSummary(report))
}

func (s *RunStore) ensureRunDir(run *TestRun) (string, error) {
	baseDir := filepath.Join(s.baseDir, run.StartedAt.Format("2006-01-02"))
	if strings.TrimSpace(run.EvaluationID) != "" {
		baseDir = filepath.Join(baseDir, run.EvaluationID, "runs")
	}
	dayDir := filepath.Join(baseDir, run.RunID)
	if err := os.MkdirAll(dayDir, 0755); err != nil {
		return "", fmt.Errorf("create run dir: %w", err)
	}
	return dayDir, nil
}

func (s *RunStore) ensureEvaluationDir(runID string) (string, error) {
	dayDir := filepath.Join(s.baseDir, time.Now().Format("2006-01-02"), runID)
	if err := os.MkdirAll(dayDir, 0755); err != nil {
		return "", fmt.Errorf("create evaluation dir: %w", err)
	}
	return dayDir, nil
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return writeBytesFile(path, data)
}

func writeTextFile(path, text string) error {
	return writeBytesFile(path, []byte(text))
}

func writeBytesFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "collection"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return replacer.Replace(name)
}
