package goal

import (
	"encoding/json"
	"testing"
)

// TestGoalJudgeFieldRoundTrip 验证 judge 判据字段的 JSON 序列化与 Summary 透传。
func TestGoalJudgeFieldRoundTrip(t *testing.T) {
	g := &Goal{
		Level:    LevelMonthly,
		Period:   "2026-06",
		Overview: "三个月减重 4kg",
		Judge:    "体重记录 ≤72kg",
		Status:   "active",
	}

	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal goal: %v", err)
	}
	var back Goal
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal goal: %v", err)
	}
	if back.Judge != "体重记录 ≤72kg" {
		t.Fatalf("judge 字段未保留: %q", back.Judge)
	}

	summary := g.Summary()
	if summary.Judge != g.Judge {
		t.Fatalf("Summary 未透传 judge: %q != %q", summary.Judge, g.Judge)
	}
}

// TestGoalJudgeOmitemptyWhenEmpty 判据为空时 JSON 省略，兼容旧数据。
func TestGoalJudgeOmitemptyWhenEmpty(t *testing.T) {
	g := &Goal{Level: LevelDaily, Period: "2026-06-12"}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(data); containsJudgeKey(got) {
		t.Fatalf("空 judge 不应出现在 JSON: %s", got)
	}
}

func containsJudgeKey(s string) bool {
	for i := 0; i+7 <= len(s); i++ {
		if s[i:i+7] == "\"judge\"" {
			return true
		}
	}
	return false
}
