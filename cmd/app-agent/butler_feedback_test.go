package main

import (
	"path/filepath"
	"testing"
)

func TestApplyButlerFeedbackDeltas(t *testing.T) {
	cur := defaultButlerAffinity()
	if cur.Score != butlerAffinityStart {
		t.Fatalf("默认养成度应为 %d, got %d", butlerAffinityStart, cur.Score)
	}

	cur = applyButlerFeedback(cur, "helpful")
	if cur.Score != butlerAffinityStart+butlerDeltaHelpful || cur.Helpful != 1 {
		t.Fatalf("helpful 反馈未正确累加: %+v", cur)
	}
	cur = applyButlerFeedback(cur, "too_frequent")
	if cur.TooFrequent != 1 || cur.Score != butlerAffinityStart+butlerDeltaHelpful+butlerDeltaTooFreq {
		t.Fatalf("too_frequent 反馈未正确累加: %+v", cur)
	}
	cur = applyButlerFeedback(cur, "bad_timing")
	if cur.BadTiming != 1 {
		t.Fatalf("bad_timing 计数错误: %+v", cur)
	}
	cur = applyButlerFeedback(cur, "seen")
	if cur.BroadcastsSeen != 1 {
		t.Fatalf("seen 计数错误: %+v", cur)
	}
}

func TestAffinityClamp(t *testing.T) {
	cur := ButlerAffinity{Score: butlerAffinityMax}
	cur = applyButlerFeedback(cur, "helpful")
	if cur.Score != butlerAffinityMax {
		t.Fatalf("上限不应被突破: %d", cur.Score)
	}
	cur = ButlerAffinity{Score: butlerAffinityMin + 1}
	cur = applyButlerFeedback(cur, "too_frequent")
	if cur.Score != butlerAffinityMin {
		t.Fatalf("下限应钳到 %d, got %d", butlerAffinityMin, cur.Score)
	}
}

func TestAffinityStorePersistAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "affinity.json")
	store := NewButlerAffinityStore(path)

	store.Apply("alice", "helpful")
	store.Apply("alice", "too_frequent")
	got := store.Get("alice")
	if got.Helpful != 1 || got.TooFrequent != 1 {
		t.Fatalf("内存状态错误: %+v", got)
	}

	reloaded := NewButlerAffinityStore(path)
	again := reloaded.Get("alice")
	if again.Helpful != 1 || again.TooFrequent != 1 || again.Score != got.Score {
		t.Fatalf("重载后状态不一致: %+v vs %+v", again, got)
	}

	if reloaded.Get("bob").Score != butlerAffinityStart {
		t.Fatalf("未知账号应回落默认值")
	}
}

func TestButlerFeedbackKindLabels(t *testing.T) {
	for kind, label := range butlerFeedbackKinds {
		if label == "" {
			t.Fatalf("kind %s 缺少标签", kind)
		}
	}
	if _, ok := butlerFeedbackKinds["nope"]; ok {
		t.Fatal("未知 kind 不应存在")
	}
}
