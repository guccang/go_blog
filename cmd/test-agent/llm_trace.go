package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func loadRecentLLMTrace(traceDir string, scenario *TestScenario, startedAt time.Time, taskID string) (*LLMTraceSnapshot, error) {
	traceDir = strings.TrimSpace(traceDir)
	if traceDir == "" {
		return nil, nil
	}
	hint := strings.TrimSpace(scenarioHintText(scenario))
	type candidate struct {
		path string
		info os.FileInfo
	}
	var files []candidate
	if err := filepath.Walk(traceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasPrefix(filepath.Base(path), "trace_") || filepath.Ext(path) != ".json" {
			return nil
		}
		if info.ModTime().Before(startedAt.Add(-2 * time.Second)) {
			return nil
		}
		files = append(files, candidate{path: path, info: info})
		return nil
	}); err != nil {
		return nil, err
	}
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].info.ModTime().After(files[j].info.ModTime())
	})
	for _, file := range files {
		data, err := os.ReadFile(file.path)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		taskIDValue := stringValue(raw["task_id"])
		queryValue := stringValue(raw["query"])
		descriptionValue := stringValue(raw["description"])
		if taskID != "" && taskIDValue == taskID {
			return buildLLMTraceSnapshot(file.path, raw), nil
		}
		if hint != "" && (strings.Contains(queryValue, hint) || strings.Contains(descriptionValue, hint)) {
			return buildLLMTraceSnapshot(file.path, raw), nil
		}
	}
	return nil, nil
}

func buildLLMTraceSnapshot(path string, raw map[string]any) *LLMTraceSnapshot {
	return &LLMTraceSnapshot{
		FilePath:  path,
		RootID:    stringValue(raw["root_id"]),
		SessionID: stringValue(raw["session_id"]),
		TaskID:    stringValue(raw["task_id"]),
		Query:     stringValue(raw["query"]),
		Trace:     raw,
	}
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
