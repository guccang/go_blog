package http

import "testing"

func TestParseGoalParentID(t *testing.T) {
	level, period, err := parseGoalParentID("daily", "monthly|2026-08")
	if err != nil {
		t.Fatalf("parseGoalParentID() error = %v", err)
	}
	if level != "monthly" || period != "2026-08" {
		t.Fatalf("parseGoalParentID() = (%q, %q)", level, period)
	}
	if _, _, err := parseGoalParentID("daily", ""); err == nil {
		t.Fatal("empty parent ID should return an error")
	}
	if _, _, err := parseGoalParentID("weekly", "yearly|2026"); err == nil {
		t.Fatal("invalid parent level should return an error")
	}
}
