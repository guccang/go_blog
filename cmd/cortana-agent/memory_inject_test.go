package main

import (
	"strings"
	"testing"
)

func TestBuildMemoryQuery(t *testing.T) {
	event := CortanaTriggerEventPayload{
		Summary:       "用户提到最近失眠",
		Content:       "我这几天睡不好",
		TriggerReason: "chat_message",
	}
	result := &MonitorResult{OverdueTodo: 2, ExerciseCnt: 0}

	query := buildMemoryQuery(event, result)
	if query == "" {
		t.Fatal("query 不应为空")
	}
	for _, want := range []string{"失眠", "待办", "锻炼"} {
		if !strings.Contains(query, want) {
			t.Errorf("query 缺少关键词 %q: %s", want, query)
		}
	}
}

func TestBuildMemoryQueryEmpty(t *testing.T) {
	if q := buildMemoryQuery(CortanaTriggerEventPayload{}, &MonitorResult{ExerciseCnt: 3}); q != "" {
		t.Errorf("无信号时 query 应为空, got %q", q)
	}
}

func TestTruncateRunes(t *testing.T) {
	s := strings.Repeat("一", 10)
	if got := truncateRunes(s, 20); got != s {
		t.Errorf("未超长不应截断")
	}
	got := truncateRunes(s, 5)
	if !strings.HasPrefix(got, strings.Repeat("一", 5)) || !strings.Contains(got, "截断") {
		t.Errorf("超长应截断并标记: %s", got)
	}
}
