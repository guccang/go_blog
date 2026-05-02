package agentbase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractLineTime_GoStandard(t *testing.T) {
	tm, ok := extractLineTime("2006/01/02 15:04:05 ERROR something happened")
	if !ok {
		t.Fatal("expected to parse Go standard log format")
	}
	if tm.Year() != 2006 || tm.Month() != 1 || tm.Day() != 2 {
		t.Errorf("unexpected date: %s", tm)
	}
}

func TestExtractLineTime_ISO8601(t *testing.T) {
	tm, ok := extractLineTime("2026-01-15 14:30:00 INFO server started")
	if !ok {
		t.Fatal("expected to parse ISO 8601 date+space format")
	}
	if tm.Year() != 2026 || tm.Month() != 1 || tm.Day() != 15 {
		t.Errorf("unexpected date: %s", tm)
	}
}

func TestExtractLineTime_RFC3339(t *testing.T) {
	tm, ok := extractLineTime("2026-01-15T14:30:00Z request received")
	if !ok {
		t.Fatal("expected to parse RFC3339 format")
	}
	if tm.Year() != 2026 {
		t.Errorf("unexpected year: %d", tm.Year())
	}
}

func TestExtractLineTime_Invalid(t *testing.T) {
	_, ok := extractLineTime("not a timestamp at all")
	if ok {
		t.Error("expected false for invalid timestamp")
	}
}

func TestParseLogTime_FullFormat(t *testing.T) {
	tm, err := ParseLogTime("2026-01-15 14:30:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.Year() != 2026 || tm.Month() != 1 || tm.Day() != 15 {
		t.Errorf("unexpected date: %s", tm)
	}
}

func TestParseLogTime_TimeOnly(t *testing.T) {
	tm, err := ParseLogTime("14:30:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Now()
	if tm.Year() != now.Year() || tm.Month() != now.Month() || tm.Day() != now.Day() {
		t.Errorf("expected today's date, got: %s", tm)
	}
	if tm.Hour() != 14 || tm.Minute() != 30 {
		t.Errorf("unexpected time: %s", tm)
	}
}

func TestParseLogTime_Invalid(t *testing.T) {
	_, err := ParseLogTime("invalid time")
	if err == nil {
		t.Error("expected error for invalid time")
	}
}

func TestReadLogFile_Regex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.log"), []byte("INFO request from 10.0.0.1\nERROR timeout\nINFO request from 10.0.0.2\nFATAL crash\n"), 0644)

	result := ReadLogFile(filepath.Join(dir, "test.log"), ReadLogOptions{
		Lines: 100,
		Regex: "ERROR|FATAL",
	})
	if !strings.Contains(result, "timeout") {
		t.Errorf("expected timeout in result: %s", result)
	}
	if !strings.Contains(result, "crash") {
		t.Errorf("expected crash in result: %s", result)
	}
	if strings.Contains(result, "10.0.0.1") {
		t.Errorf("expected INFO line to be filtered out: %s", result)
	}
}

func TestReadLogFile_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.log"), []byte("INFO ok\nError something\nERROR else\nWARN done\n"), 0644)

	result := ReadLogFile(filepath.Join(dir, "test.log"), ReadLogOptions{
		Lines:           100,
		Keyword:         "error",
		CaseInsensitive: true,
	})
	if !strings.Contains(result, "Error something") {
		t.Errorf("expected 'Error something' in case-insensitive result: %s", result)
	}
	if !strings.Contains(result, "ERROR else") {
		t.Errorf("expected 'ERROR else' in case-insensitive result: %s", result)
	}
}
