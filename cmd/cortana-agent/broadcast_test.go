package main

import (
	"testing"
	"time"
)

func TestBroadcastDecider_EvaluateTimeRange(t *testing.T) {
	cfg := DefaultConfig()
	// 使用一个总是在当前时间的范围
	cfg.Broadcasts = []BroadcastConfig{
		{
			ID:         "test_greeting",
			Trigger:    "time_range",
			TimeStart:  "00:00",
			TimeEnd:    "23:59",
			MaxPerDay:  10,
			Template:   "测试播报：博客数 {{blog_count}}，待办 {{todo_count}}",
			Expression: "happy",
			Motion:     "IdleWave",
		},
	}
	cfg.BroadcastCooldownSec = 0 // 无冷却

	decider := NewBroadcastDecider(cfg)

	result := &MonitorResult{
		Account:   "test_user",
		CheckedAt: time.Now(),
		BlogCount: 42,
		TodoCount: 7,
	}

	decision := decider.Evaluate(result)
	if !decision.ShouldBroadcast {
		t.Fatalf("expected broadcast, got skip: %s", decision.SkipReason)
	}
	if decision.BroadcastID != "test_greeting" {
		t.Fatalf("expected broadcast_id 'test_greeting', got '%s'", decision.BroadcastID)
	}
	if decision.Text != "测试播报：博客数 42，待办 7" {
		t.Fatalf("unexpected rendered text: %s", decision.Text)
	}
	if decision.Expression != "happy" {
		t.Fatalf("expected expression 'happy', got '%s'", decision.Expression)
	}
	if decision.Motion != "IdleWave" {
		t.Fatalf("expected motion 'IdleWave', got '%s'", decision.Motion)
	}
}

func TestBroadcastDecider_EvaluateDataChange(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Broadcasts = []BroadcastConfig{
		{
			ID:          "new_blog_alert",
			Trigger:     "data_change",
			DataSources: []string{"blog_new_count"},
			MinCount:    1,
			MaxPerDay:   10,
			Template:    "检测到{{blog_new_count}}篇新博客",
			Expression:  "surprised",
			Motion:      "Tap",
		},
		{
			ID:          "todo_alert",
			Trigger:     "data_change",
			DataSources: []string{"todo_overdue_count"},
			MinCount:    3,
			MaxPerDay:   10,
			Template:    "你有{{todo_overdue_count}}个过期待办",
			Expression:  "sad",
			Motion:      "IdleAlt",
		},
	}
	cfg.BroadcastCooldownSec = 0

	decider := NewBroadcastDecider(cfg)

	// 测试有3篇新博客 -> 应该触发 new_blog_alert
	result := &MonitorResult{
		Account:     "test_user",
		CheckedAt:   time.Now(),
		NewBlogs:    3,
		OverdueTodo: 1,
	}

	decision := decider.Evaluate(result)
	if !decision.ShouldBroadcast {
		t.Fatalf("expected broadcast, got skip: %s", decision.SkipReason)
	}
	if decision.BroadcastID != "new_blog_alert" {
		t.Fatalf("expected broadcast_id 'new_blog_alert', got '%s'", decision.BroadcastID)
	}
	if decision.Text != "检测到3篇新博客" {
		t.Fatalf("unexpected text: %s", decision.Text)
	}

	// 记录播报后再次评估 -> 第二个场景 todo_alert 不应触发（只有1个过期待办，min=3）
	decider.RecordBroadcast("test_user", "new_blog_alert")
	result2 := &MonitorResult{
		Account:     "test_user",
		CheckedAt:   time.Now(),
		NewBlogs:    0,
		OverdueTodo: 2,
	}

	decision2 := decider.Evaluate(result2)
	if decision2.ShouldBroadcast {
		t.Fatalf("expected no broadcast, got broadcast_id=%s", decision2.BroadcastID)
	}
}

func TestBroadcastDecider_Cooldown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Broadcasts = []BroadcastConfig{
		{
			ID:         "test_greeting",
			Trigger:    "time_range",
			TimeStart:  "00:00",
			TimeEnd:    "23:59",
			MaxPerDay:  100,
			Template:   "测试播报",
			Expression: "happy",
			Motion:     "Idle",
		},
	}
	cfg.BroadcastCooldownSec = 3600 // 1小时冷却

	decider := NewBroadcastDecider(cfg)

	result := &MonitorResult{
		Account:   "test_user",
		CheckedAt: time.Now(),
	}

	// 第一次应该触发
	decision1 := decider.Evaluate(result)
	if !decision1.ShouldBroadcast {
		t.Fatalf("first broadcast should trigger, got skip: %s", decision1.SkipReason)
	}

	// 记录播报
	decider.RecordBroadcast("test_user", "test_greeting")

	// 第二次应该被冷却阻止
	decision2 := decider.Evaluate(result)
	if decision2.ShouldBroadcast {
		t.Fatalf("second broadcast should be blocked by cooldown")
	}
	if decision2.SkipReason == "" {
		t.Fatalf("expected skip reason")
	}
}

func TestBroadcastDecider_DailyLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Broadcasts = []BroadcastConfig{
		{
			ID:         "test_greeting",
			Trigger:    "time_range",
			TimeStart:  "00:00",
			TimeEnd:    "23:59",
			MaxPerDay:  3,
			Template:   "测试播报",
			Expression: "happy",
			Motion:     "Idle",
		},
	}
	cfg.BroadcastCooldownSec = 0

	decider := NewBroadcastDecider(cfg)
	result := &MonitorResult{
		Account:   "test_user",
		CheckedAt: time.Now(),
	}

	// 前3次应该触发
	for i := 0; i < 3; i++ {
		decision := decider.Evaluate(result)
		if !decision.ShouldBroadcast {
			t.Fatalf("broadcast %d should trigger, got skip: %s", i+1, decision.SkipReason)
		}
		decider.RecordBroadcast("test_user", "test_greeting")
	}

	// 第4次应该被每日限制阻止
	decision := decider.Evaluate(result)
	if decision.ShouldBroadcast {
		t.Fatalf("4th broadcast should be blocked by daily limit")
	}
}

func TestBroadcastDecider_TemplateRendering(t *testing.T) {
	cfg := DefaultConfig()
	bd := NewBroadcastDecider(cfg)

	result := &MonitorResult{
		Account:     "test_user",
		CheckedAt:   time.Now(),
		BlogCount:   15,
		NewBlogs:    3,
		TodoCount:   8,
		OverdueTodo: 2,
		ExerciseMin: 45,
		ExerciseCnt: 3,
	}

	tests := []struct {
		template string
		expected string
	}{
		{"博客：{{blog_count}}篇", "博客：15篇"},
		{"新博客：{{blog_new_count}}篇", "新博客：3篇"},
		{"待办：{{todo_count}}项", "待办：8项"},
		{"过期：{{todo_overdue_count}}项", "过期：2项"},
		{"运动：{{exercise_min}}分钟", "运动：45分钟"},
		{"完成：{{exercise_completed}}项", "完成：3项"},
	}

	for _, tt := range tests {
		rendered := bd.renderTemplate(tt.template, result)
		if rendered != tt.expected {
			t.Errorf("template %q: expected %q, got %q", tt.template, tt.expected, rendered)
		}
	}
}

func TestParseTimeMinutes(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		valid    bool
	}{
		{"06:00", 360, true},
		{"23:59", 1439, true},
		{"00:00", 0, true},
		{"12:30", 750, true},
		{"", 0, false},
		{"25:00", 0, false},
		{"12:60", 0, false},
		{"invalid", 0, false},
	}

	for _, tt := range tests {
		result, ok := parseTimeMinutes(tt.input)
		if ok != tt.valid {
			t.Errorf("parseTimeMinutes(%q): expected valid=%v, got valid=%v", tt.input, tt.valid, ok)
		}
		if ok && result != tt.expected {
			t.Errorf("parseTimeMinutes(%q): expected %d, got %d", tt.input, tt.expected, result)
		}
	}
}

func TestAllStates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BroadcastCooldownSec = 0
	decider := NewBroadcastDecider(cfg)

	states := decider.AllStates()
	if len(states) != len(cfg.Broadcasts) {
		t.Fatalf("expected %d states, got %d", len(cfg.Broadcasts), len(states))
	}

	for _, bc := range cfg.Broadcasts {
		if _, ok := states[bc.ID]; !ok {
			t.Errorf("missing state for broadcast %q", bc.ID)
		}
	}
}

func TestBroadcastDeciderRecordBroadcastIsAccountScoped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Broadcasts = []BroadcastConfig{
		{
			ID:         "test_greeting",
			Trigger:    "time_range",
			TimeStart:  "00:00",
			TimeEnd:    "23:59",
			MaxPerDay:  1,
			Template:   "测试播报",
			Expression: "happy",
			Motion:     "Idle",
		},
	}
	cfg.BroadcastCooldownSec = 3600

	decider := NewBroadcastDecider(cfg)
	decider.RecordBroadcast("ztt", "test_greeting")

	aliceDecision := decider.Evaluate(&MonitorResult{
		Account:   "alice",
		CheckedAt: time.Now(),
	})
	if !aliceDecision.ShouldBroadcast {
		t.Fatalf("expected alice to remain eligible, got skip: %s", aliceDecision.SkipReason)
	}

	zttDecision := decider.Evaluate(&MonitorResult{
		Account:   "ztt",
		CheckedAt: time.Now(),
	})
	if zttDecision.ShouldBroadcast {
		t.Fatalf("expected ztt to be blocked by account-scoped cooldown")
	}
}
