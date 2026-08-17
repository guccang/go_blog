package exercise

import (
	"testing"
	"time"
)

func completedExercise(name string, duration int) ExerciseItem {
	return ExerciseItem{Name: name, Duration: duration, Completed: true}
}

func TestCalculateExerciseOverview(t *testing.T) {
	all := map[string]ExerciseList{
		"2026-07-24": {Date: "2026-07-24", Items: []ExerciseItem{completedExercise("步行", 20)}},
		"2026-07-25": {Date: "2026-07-25", Items: []ExerciseItem{completedExercise("慢跑", 30)}},
		"2026-07-26": {Date: "2026-07-26", Items: []ExerciseItem{{Name: "计划", Duration: 60}}},
		"2026-07-29": {Date: "2026-07-29", Items: []ExerciseItem{completedExercise("力量", 40)}},
	}
	end, _ := time.Parse("2006-01-02", "2026-07-30")

	overview := calculateExerciseOverview(all, end, 7)
	if overview.ExerciseDays != 3 {
		t.Fatalf("exercise days = %d, want 3", overview.ExerciseDays)
	}
	if overview.TotalDuration != 90 {
		t.Fatalf("total duration = %d, want 90", overview.TotalDuration)
	}
	if overview.CurrentStreak != 1 {
		t.Fatalf("current streak = %d, want 1", overview.CurrentStreak)
	}
	if len(overview.Days) != 7 || overview.Days[0].Date != "2026-07-24" || overview.Days[6].Date != "2026-07-30" {
		t.Fatalf("unexpected day range: %+v", overview.Days)
	}
}

func TestCalculateExerciseOverviewCountsStreakBeyondVisibleRange(t *testing.T) {
	all := make(map[string]ExerciseList)
	start, _ := time.Parse("2006-01-02", "2026-07-20")
	for offset := 0; offset < 10; offset++ {
		date := start.AddDate(0, 0, offset).Format("2006-01-02")
		all[date] = ExerciseList{Date: date, Items: []ExerciseItem{completedExercise("步行", 10)}}
	}
	end, _ := time.Parse("2006-01-02", "2026-07-30")

	overview := calculateExerciseOverview(all, end, 7)
	if overview.CurrentStreak != 10 {
		t.Fatalf("current streak = %d, want 10", overview.CurrentStreak)
	}
}

func TestGetDateFromTitleOnlyAcceptsDailyExerciseBlogs(t *testing.T) {
	tests := map[string]string{
		"exercise-2026-07-30":    "2026-07-30",
		"exercise-2026-07-30.md": "2026-07-30",
		"exercise-templates":     "",
		"exercise-user-profile":  "",
	}
	for title, want := range tests {
		if got := getDateFromTitle(title); got != want {
			t.Fatalf("getDateFromTitle(%q) = %q, want %q", title, got, want)
		}
	}
}
