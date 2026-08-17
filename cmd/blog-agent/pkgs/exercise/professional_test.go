package exercise

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func testProfessionalLevels() map[string]int {
	return map[string]int{
		"pushup": 2, "squat": 3, "pullup": 4,
		"legraise": 5, "bridge": 6, "handstand": 7,
	}
}

func TestProfessionalCatalogHasSixMovementsAndTenLevels(t *testing.T) {
	catalog := ProfessionalCatalogData()
	if len(catalog.Movements) != 6 {
		t.Fatalf("movement count = %d, want 6", len(catalog.Movements))
	}
	for _, movement := range catalog.Movements {
		if len(movement.Levels) != 10 {
			t.Fatalf("%s level count = %d, want 10", movement.ID, len(movement.Levels))
		}
		for index, level := range movement.Levels {
			if level.Level != index+1 || level.MovementID != movement.ID {
				t.Fatalf("unexpected level metadata: %+v", level)
			}
		}
	}
}

func TestPreviewProfessionalPlanFrequencies(t *testing.T) {
	tests := []struct {
		days    int
		offsets []int
		counts  []int
	}{
		{2, []int{1, 4}, []int{3, 3}},
		{3, []int{1, 3, 5}, []int{2, 2, 2}},
		{4, []int{1, 2, 4, 6}, []int{2, 1, 2, 1}},
	}
	for _, test := range tests {
		plan, err := PreviewProfessionalPlan(ProfessionalPlanRequest{
			StartDate: "2026-08-03", DaysPerWeek: test.days, Levels: testProfessionalLevels(),
		})
		if err != nil {
			t.Fatalf("preview %d days failed: %v", test.days, err)
		}
		if plan.EndDate != "2026-08-09" || len(plan.Sessions) != len(test.offsets) {
			t.Fatalf("unexpected plan range/sessions: %+v", plan)
		}
		for index, session := range plan.Sessions {
			if session.Day != test.offsets[index] || len(session.Items) != test.counts[index] {
				t.Fatalf("%d-day session %d = %+v", test.days, index, session)
			}
		}
	}
}

func TestPreviewProfessionalPlanRejectsInvalidInput(t *testing.T) {
	_, err := PreviewProfessionalPlan(ProfessionalPlanRequest{
		StartDate: "2026/08/03", DaysPerWeek: 5, Levels: testProfessionalLevels(),
	})
	if err == nil {
		t.Fatal("expected invalid plan request to fail")
	}
	levels := testProfessionalLevels()
	levels["pushup"] = 11
	_, err = PreviewProfessionalPlan(ProfessionalPlanRequest{
		StartDate: "2026-08-03", DaysPerWeek: 3, Levels: levels,
	})
	if err == nil {
		t.Fatal("expected invalid progression level to fail")
	}
	levels = testProfessionalLevels()
	levels["unknown"] = 1
	_, err = PreviewProfessionalPlan(ProfessionalPlanRequest{
		StartDate: "2026-08-03", DaysPerWeek: 3, Levels: levels,
	})
	if err == nil {
		t.Fatal("expected unknown movement to fail")
	}
}

func TestApplyProfessionalPlanConflictAndReplace(t *testing.T) {
	plan, err := PreviewProfessionalPlan(ProfessionalPlanRequest{
		StartDate: "2026-08-03", DaysPerWeek: 3, Levels: testProfessionalLevels(),
	})
	if err != nil {
		t.Fatal(err)
	}
	lists := map[string]ExerciseList{}
	for _, date := range professionalPlanDates(plan) {
		lists[date] = ExerciseList{Date: date}
	}
	first, err := applyProfessionalPlanToLists(lists, plan, false, time.Unix(100, 0))
	if err != nil || first.Created != 6 {
		t.Fatalf("first apply = %+v, %v", first, err)
	}
	if _, err := applyProfessionalPlanToLists(lists, plan, false, time.Unix(200, 0)); !errors.Is(err, ErrProfessionalPlanConflict) {
		t.Fatalf("second apply error = %v, want conflict", err)
	}

	firstDate := plan.Sessions[0].Date
	firstList := lists[firstDate]
	firstList.Items[0].Completed = true
	firstList.Items = append(firstList.Items, ExerciseItem{ID: "manual", Name: "散步", Completed: true})
	lists[firstDate] = firstList

	replaced, err := applyProfessionalPlanToLists(lists, plan, true, time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Preserved != 1 || replaced.Skipped != 1 || replaced.Created != 5 {
		t.Fatalf("replace result = %+v", replaced)
	}
	manualFound := false
	for _, item := range lists[firstDate].Items {
		if item.ID == "manual" {
			manualFound = true
		}
	}
	if !manualFound {
		t.Fatal("manual exercise was removed while replacing professional plan")
	}
}

func TestProfessionalPlanConflictCoversEntireSevenDayRange(t *testing.T) {
	plan, err := PreviewProfessionalPlan(ProfessionalPlanRequest{
		StartDate: "2026-08-03", DaysPerWeek: 3, Levels: testProfessionalLevels(),
	})
	if err != nil {
		t.Fatal(err)
	}
	lists := map[string]ExerciseList{}
	for _, date := range professionalPlanDates(plan) {
		lists[date] = ExerciseList{Date: date}
	}
	unscheduledDate := "2026-08-04"
	lists[unscheduledDate] = ExerciseList{
		Date:  unscheduledDate,
		Items: []ExerciseItem{{ID: "old-plan", Source: professionalSource, MovementID: "squat"}},
	}
	if _, err := applyProfessionalPlanToLists(lists, plan, false, time.Now()); !errors.Is(err, ErrProfessionalPlanConflict) {
		t.Fatalf("conflict outside new session dates was not detected: %v", err)
	}
}

func TestExerciseItemLegacyJSONCompatibility(t *testing.T) {
	var item ExerciseItem
	if err := json.Unmarshal([]byte(`{"id":"old","name":"慢跑","type":"cardio","duration":20,"completed":true}`), &item); err != nil {
		t.Fatal(err)
	}
	if item.Name != "慢跑" || item.Source != "" || item.PlanID != "" || item.ProgressionLevel != 0 {
		t.Fatalf("legacy item changed unexpectedly: %+v", item)
	}
}
