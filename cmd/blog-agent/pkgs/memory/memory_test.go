package memory

import "testing"

func TestValidFile(t *testing.T) {
	valid := []string{"MEMORY", "checkpoint", "goals", "journal_2026-06-11"}
	for _, name := range valid {
		if !ValidFile(name) {
			t.Fatalf("ValidFile(%q) = false, want true", name)
		}
	}
	invalid := []string{"", "memory", "journal_", "journal_2026-13-99", "../etc", "journal_abc"}
	for _, name := range invalid {
		if ValidFile(name) {
			t.Fatalf("ValidFile(%q) = true, want false", name)
		}
	}
}

func TestScoreLine(t *testing.T) {
	terms := tokenize("锻炼 失眠")
	if len(terms) != 2 {
		t.Fatalf("tokenize returned %d terms", len(terms))
	}
	if got := scoreLine("最近失眠，晚上不想锻炼", terms); got != 2 {
		t.Fatalf("scoreLine = %d, want 2", got)
	}
	if got := scoreLine("今天天气不错", terms); got != 0 {
		t.Fatalf("scoreLine = %d, want 0", got)
	}
}
