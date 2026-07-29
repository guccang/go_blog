package persistence

import (
	"testing"
	"time"
)

func TestBuildHookInsightsAggregatesAndBuildsPaths(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.Local)
	events := []BlogHookEvent{
		{ID: 4, SessionID: "s1", EventType: "ai.answered", Feature: "pi_agent", ResultJSON: `{"status":"error"}`, CreatedAt: "2026-07-28 21:00:00"},
		{ID: 1, SessionID: "s1", EventType: "page.opened", Feature: "content_workspace", ResultJSON: `{"status":"success"}`, CreatedAt: "2026-07-29 09:00:00"},
		{ID: 2, SessionID: "s1", EventType: "feature.used", Feature: "fts_search", Query: "SQLite", ResultJSON: `{}`, CreatedAt: "2026-07-29 09:03:00"},
		{ID: 3, SessionID: "s1", EventType: "page.opened", Feature: "blog_reader", Title: "SQLite迁移", ResultJSON: `{"status":"success"}`, CreatedAt: "2026-07-29 09:06:00"},
	}

	insights := buildHookInsights(events, 7, HookInsightFilter{}, now)
	if insights.Summary.TotalEvents != 4 || insights.Summary.TodayEvents != 3 {
		t.Fatalf("unexpected summary: %+v", insights.Summary)
	}
	if insights.Summary.ActiveDays != 2 {
		t.Fatalf("active days = %d, want 2", insights.Summary.ActiveDays)
	}
	if insights.Summary.SuccessRate != 200.0/3.0 {
		t.Fatalf("success rate = %v, want %v", insights.Summary.SuccessRate, 200.0/3.0)
	}
	if len(insights.Paths) != 1 || len(insights.Paths[0].Steps) != 3 {
		t.Fatalf("unexpected paths: %+v", insights.Paths)
	}
	if insights.Timeline[0].ID != 3 {
		t.Fatalf("timeline is not newest first: %+v", insights.Timeline)
	}
}

func TestBuildHookInsightsAppliesStatusFilterButKeepsOptions(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.Local)
	events := []BlogHookEvent{
		{ID: 1, EventType: "page.opened", Feature: "blog_reader", ResultJSON: `{"status":"success"}`, CreatedAt: "2026-07-29 09:00:00"},
		{ID: 2, EventType: "ai.answered", Feature: "pi_agent", ResultJSON: `{"status":"error"}`, CreatedAt: "2026-07-29 09:01:00"},
	}
	insights := buildHookInsights(events, 7, HookInsightFilter{Status: "error"}, now)
	if insights.Summary.TotalEvents != 1 || len(insights.Features) != 1 || insights.Features[0].Feature != "pi_agent" {
		t.Fatalf("status filter not applied: %+v", insights)
	}
	if len(insights.AvailableFeatures) != 2 || len(insights.AvailableEvents) != 2 {
		t.Fatalf("filter options were narrowed: features=%v events=%v", insights.AvailableFeatures, insights.AvailableEvents)
	}
}

func TestBuildHookPathsSplitsAfterThirtyMinutes(t *testing.T) {
	base := time.Date(2026, 7, 29, 9, 0, 0, 0, time.Local)
	events := []hookInsightEvent{
		{BlogHookEvent: BlogHookEvent{Feature: "main", SessionID: "s1"}, Time: base},
		{BlogHookEvent: BlogHookEvent{Feature: "search", SessionID: "s1"}, Time: base.Add(5 * time.Minute)},
		{BlogHookEvent: BlogHookEvent{Feature: "reader", SessionID: "s1"}, Time: base.Add(40 * time.Minute)},
	}
	paths := buildHookPaths(events)
	if len(paths) != 1 || len(paths[0].Steps) != 2 {
		t.Fatalf("unexpected paths after inactivity split: %+v", paths)
	}
}
