package piagent

import "testing"

func TestParseGoalTaskDraftsFiltersAndNormalizes(t *testing.T) {
	content := "```json\n{\"tasks\":[" +
		"{\"title\":\" 已有 任务 \",\"priority\":\"high\"}," +
		"{\"title\":\"实现接口\",\"priority\":\"urgent\"}," +
		"{\"title\":\"验证页面\",\"priority\":\"low\"}," +
		"{\"title\":\"补充测试\",\"priority\":\"medium\"}," +
		"{\"title\":\"不应保留\",\"priority\":\"medium\"}]}\n```"
	tasks, err := parseGoalTaskDrafts(content, GoalTaskContext{
		ExistingTasks: []GoalTaskReference{{Title: "已有任务"}},
	})
	if err != nil {
		t.Fatalf("parseGoalTaskDrafts() error = %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("len(tasks) = %d, want 3", len(tasks))
	}
	if tasks[0].Title != "实现接口" || tasks[0].Priority != "medium" {
		t.Fatalf("tasks[0] = %#v", tasks[0])
	}
}

func TestParseGoalTaskDraftsRejectsEmptyResult(t *testing.T) {
	if _, err := parseGoalTaskDrafts(`{"tasks":[]}`, GoalTaskContext{}); err == nil {
		t.Fatal("empty generated task list should return an error")
	}
}

func TestParseDailyGoalTaskDraftsUsesOnlyTwoSlots(t *testing.T) {
	content := `{"tasks":[
		{"title":"任务一","source_task_title":"上午方向"},
		{"title":"任务二","source_task_title":"下午方向"},
		{"title":"任务三","source_task_title":"下午方向"}
	]}`
	tasks, err := parseGoalTaskDrafts(content, GoalTaskContext{
		CurrentLevel: "daily",
		ParentTasks: []GoalTaskReference{
			{ID: "morning-source", Title: "上午方向", Importance: 5, Schedules: []GoalExecutionSlot{{TimeSlot: "morning"}}},
			{ID: "afternoon-source", Title: "下午方向", Importance: 2, Schedules: []GoalExecutionSlot{{TimeSlot: "afternoon"}}},
		},
	})
	if err != nil {
		t.Fatalf("parseGoalTaskDrafts() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0].Schedules[0].TimeSlot != "morning" || tasks[1].Schedules[0].TimeSlot != "afternoon" {
		t.Fatalf("daily schedules = %#v", tasks)
	}
	if tasks[0].Importance != 5 || tasks[0].Priority != "high" {
		t.Fatalf("task importance was not inherited: %#v", tasks[0])
	}
	if tasks[0].SourceTaskID != "morning-source" {
		t.Fatalf("source task id = %q", tasks[0].SourceTaskID)
	}
}

func TestParseWeeklyGoalTaskDraftsAllocatesMoreTimeByImportance(t *testing.T) {
	content := `{"tasks":[
		{"title":"核心任务","source_task_title":"核心方向"},
		{"title":"次要任务","source_task_title":"次要方向"}
	]}`
	tasks, err := parseGoalTaskDrafts(content, GoalTaskContext{
		CurrentLevel: "weekly",
		ParentTasks: []GoalTaskReference{
			{ID: "core-source", Title: "核心方向", Importance: 5},
			{ID: "minor-source", Title: "次要方向", Importance: 2},
		},
	})
	if err != nil {
		t.Fatalf("parseGoalTaskDrafts() error = %v", err)
	}
	if len(tasks[0].Schedules) != 5 || len(tasks[1].Schedules) != 2 {
		t.Fatalf("weekly allocation = %d and %d slots, want 5 and 2", len(tasks[0].Schedules), len(tasks[1].Schedules))
	}
	for index, schedule := range tasks[0].Schedules {
		if schedule.Weekday != index+1 || schedule.TimeSlot != "morning" {
			t.Fatalf("core task schedules should continue through weekday mornings: %#v", tasks[0].Schedules)
		}
	}
}

func TestWeeklySlotBudgetsPreserveEveryActiveDirection(t *testing.T) {
	tasks := []GoalTaskDraft{
		{Importance: 5}, {Importance: 5}, {Importance: 5}, {Importance: 5}, {Importance: 5},
	}
	budgets := weeklySlotBudgets(tasks, 14)
	total := 0
	for i, budget := range budgets {
		if budget < 1 {
			t.Fatalf("budget[%d] = %d, every selected direction needs continuity", i, budget)
		}
		total += budget
	}
	if total != 14 {
		t.Fatalf("total budget = %d, want 14", total)
	}
}
