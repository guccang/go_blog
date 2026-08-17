package goal

import (
	"encoding/json"
	"testing"
)

func TestValidateTaskScheduleRejectsDuplicateWeeklySlot(t *testing.T) {
	weekly := &Goal{Level: LevelWeekly, Tasks: []Task{{
		ID:               "existing",
		ScheduledWeekday: 1,
		TimeSlot:         "morning",
	}}}
	err := validateTaskSchedule(weekly, Task{ScheduledWeekday: 1, TimeSlot: "morning"}, "", true)
	if err == nil {
		t.Fatal("duplicate weekly execution slot should be rejected")
	}
}

func TestValidateTaskScheduleAllowsMorningAndAfternoon(t *testing.T) {
	daily := &Goal{Level: LevelDaily, Tasks: []Task{{ID: "morning", TimeSlot: "morning"}}}
	if err := validateTaskSchedule(daily, Task{TimeSlot: "afternoon"}, "", true); err != nil {
		t.Fatalf("afternoon task should fit after morning task: %v", err)
	}
}

func TestAssignFirstAvailableDailyScheduleKeepsLegacyCallersWorking(t *testing.T) {
	daily := &Goal{Level: LevelDaily, Tasks: []Task{{ID: "morning", TimeSlot: "morning"}}}
	task := Task{Importance: 3}
	assignImportanceSchedules(daily, &task)
	if task.TimeSlot != "afternoon" {
		t.Fatalf("assigned time slot = %q, want afternoon", task.TimeSlot)
	}
}

func TestAssignWeeklySchedulesUsesImportanceAsTimeBudget(t *testing.T) {
	weekly := &Goal{Level: LevelWeekly}
	task := Task{Importance: 5}
	assignImportanceSchedules(weekly, &task)
	if len(task.Schedules) != 5 {
		t.Fatalf("schedule count = %d, want 5", len(task.Schedules))
	}
	if task.Priority != "high" {
		t.Fatalf("priority = %q, want high", task.Priority)
	}
}

func TestTaskImportanceAndSchedulesJSONRoundTrip(t *testing.T) {
	original := Task{
		Title:      "持续推进核心目标",
		Importance: 5,
		Schedules: []ExecutionSlot{
			{Weekday: 1, TimeSlot: "morning"},
			{Weekday: 2, TimeSlot: "morning"},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Importance != 5 || len(decoded.Schedules) != 2 {
		t.Fatalf("decoded planning data = %#v", decoded)
	}
}
