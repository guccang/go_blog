package main

import (
	"strings"
	"testing"
)

func TestValidateLogPipeline_Valid(t *testing.T) {
	tests := []string{
		"grep ERROR",
		"grep ERROR | wc -l",
		"grep -i error | awk '{print $1}' | sort | uniq -c | sort -rn | head -10",
		"cat | awk '{print $1,$2}' | sort | uniq -c",
		"head -100",
		"sed 's/foo/bar/g'",
		"cut -d' ' -f1 | sort | uniq -c",
	}
	for _, cmd := range tests {
		if err := validateLogPipeline(cmd); err != nil {
			t.Errorf("expected valid, got error for %q: %v", cmd, err)
		}
	}
}

func TestValidateLogPipeline_InvalidCommand(t *testing.T) {
	tests := []string{
		"python script.py",
		"rm -rf /",
		"curl http://example.com",
		"find . -name '*.log'",
	}
	for _, cmd := range tests {
		if err := validateLogPipeline(cmd); err == nil {
			t.Errorf("expected error for %q, got nil", cmd)
		}
	}
}

func TestValidateLogPipeline_DangerousPatterns(t *testing.T) {
	tests := []string{
		"grep ERROR; rm -rf /",
		"grep ERROR && echo done",
		"grep ERROR || echo fail",
		"echo `whoami`",
		"echo $(whoami)",
		"grep ERROR >> /etc/passwd",
	}
	for _, cmd := range tests {
		if err := validateLogPipeline(cmd); err == nil {
			t.Errorf("expected error for dangerous pattern %q, got nil", cmd)
		}
	}
}

func TestValidateLogPipeline_EmptyCommand(t *testing.T) {
	if err := validateLogPipeline(""); err == nil {
		t.Error("expected error for empty command")
	}
}

func TestBuildAnalyzeCommand_TopErrors(t *testing.T) {
	cmd, err := buildAnalyzeCommand("top_errors", "", 0, 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cmd, "grep") {
		t.Errorf("expected grep in command: %s", cmd)
	}
	if !strings.Contains(cmd, "sort | uniq -c | sort -rn") {
		t.Errorf("expected sort|uniq|sort pipeline: %s", cmd)
	}
}

func TestBuildAnalyzeCommand_TopValues(t *testing.T) {
	cmd, err := buildAnalyzeCommand("top_values", "ERROR", 3, 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cmd, "grep -i 'ERROR'") {
		t.Errorf("expected keyword filter: %s", cmd)
	}
	if !strings.Contains(cmd, "awk '{print $3}'") {
		t.Errorf("expected field extraction: %s", cmd)
	}
}

func TestBuildAnalyzeCommand_Rate(t *testing.T) {
	cmd, err := buildAnalyzeCommand("rate", "ERROR", 0, 0, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cmd, "matched_lines") {
		t.Errorf("expected matched_lines stat in: %s", cmd)
	}
}

func TestBuildAnalyzeCommand_Summary(t *testing.T) {
	cmd, err := buildAnalyzeCommand("summary", "", 0, 0, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cmd, "total_lines") {
		t.Errorf("expected total_lines in summary: %s", cmd)
	}
}

func TestBuildAnalyzeCommand_InvalidAnalysis(t *testing.T) {
	_, err := buildAnalyzeCommand("unknown", "", 0, 0, "")
	if err == nil {
		t.Error("expected error for unknown analysis type")
	}
}
