package projectmgmt

import "testing"

func TestNormalizeTags(t *testing.T) {
	got := normalizeTags([]string{" alpha ", "Beta", "alpha", "", "BETA"})
	if len(got) != 2 {
		t.Fatalf("expected 2 tags, got %d: %#v", len(got), got)
	}
	if got[0] != "alpha" || got[1] != "Beta" {
		t.Fatalf("unexpected tags: %#v", got)
	}
}

func TestValidateProject(t *testing.T) {
	project := &Project{
		ID:        "p1",
		Name:      "Launch",
		Status:    "active",
		Priority:  "high",
		StartDate: "2026-04-01",
		EndDate:   "2026-04-30",
	}
	if err := validateProject(project); err != nil {
		t.Fatalf("validateProject returned error: %v", err)
	}
}
