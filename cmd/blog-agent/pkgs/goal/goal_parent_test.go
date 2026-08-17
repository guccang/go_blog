package goal

import "testing"

func TestResolveParentPeriod(t *testing.T) {
	tests := []struct {
		name       string
		level      string
		period     string
		wantLevel  string
		wantPeriod string
	}{
		{name: "日目标映射到所在周", level: LevelDaily, period: "2026-07-27", wantLevel: LevelWeekly, wantPeriod: "2026-W31"},
		{name: "跨年日期使用 ISO 周年", level: LevelDaily, period: "2026-01-01", wantLevel: LevelWeekly, wantPeriod: "2026-W01"},
		{name: "周目标映射到周一所在月", level: LevelWeekly, period: "2026-W31", wantLevel: LevelMonthly, wantPeriod: "2026-07"},
		{name: "月目标映射到所在年", level: LevelMonthly, period: "2026-07", wantLevel: LevelYearly, wantPeriod: "2026"},
		{name: "年目标没有上层", level: LevelYearly, period: "2026"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, gotPeriod, err := resolveParentPeriod(tt.level, tt.period)
			if err != nil {
				t.Fatalf("resolveParentPeriod() error = %v", err)
			}
			if gotLevel != tt.wantLevel || gotPeriod != tt.wantPeriod {
				t.Fatalf("resolveParentPeriod() = (%q, %q), want (%q, %q)", gotLevel, gotPeriod, tt.wantLevel, tt.wantPeriod)
			}
		})
	}
}

func TestResolveParentPeriodRejectsInvalidPeriod(t *testing.T) {
	if _, _, err := resolveParentPeriod(LevelDaily, "2026-07"); err == nil {
		t.Fatal("无效日周期应返回错误")
	}
}

func TestResolveParentPeriodsDailyFallsBackToMonthly(t *testing.T) {
	got, err := resolveParentPeriods(LevelDaily, "2026-08-04")
	if err != nil {
		t.Fatalf("resolveParentPeriods() error = %v", err)
	}
	want := []parentPeriod{
		{level: LevelWeekly, period: "2026-W32"},
		{level: LevelMonthly, period: "2026-08"},
	}
	if len(got) != len(want) {
		t.Fatalf("resolveParentPeriods() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveParentPeriods()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestResolveParentPeriodsWeeklyDoesNotSkipMonthly(t *testing.T) {
	got, err := resolveParentPeriods(LevelWeekly, "2026-W32")
	if err != nil {
		t.Fatalf("resolveParentPeriods() error = %v", err)
	}
	if len(got) != 1 || got[0] != (parentPeriod{level: LevelMonthly, period: "2026-08"}) {
		t.Fatalf("resolveParentPeriods() = %#v", got)
	}
}
