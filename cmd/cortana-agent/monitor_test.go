package main

import (
	"testing"
	"time"
)

func newTestMonitorEngine() *MonitorEngine {
	cfg := DefaultConfig()
	cfg.BroadcastCooldownSec = 0
	cfg.Broadcasts = []BroadcastConfig{
		{
			ID:        "morning",
			Trigger:   "time_range",
			TimeStart: "00:00",
			TimeEnd:   "23:59",
			MaxPerDay: 10,
			Template:  "早上好",
		},
	}
	return NewMonitorEngine(cfg, nil, NewBroadcastDecider(cfg), NewAccountRegistry())
}

func TestMonitorRunCheckCycleBroadcastsPerRegisteredAccount(t *testing.T) {
	engine := newTestMonitorEngine()
	engine.RegisterAccount("ztt")
	engine.RegisterAccount("alice")

	var checked []string
	var broadcasted []string
	engine.OnPerformCheck = func(account string) *MonitorResult {
		checked = append(checked, account)
		return &MonitorResult{
			Account:   account,
			CheckedAt: time.Now(),
		}
	}
	engine.OnExecuteBroadcast = func(decision BroadcastDecision) {
		broadcasted = append(broadcasted, decision.Account)
	}

	engine.runCheckCycle()

	if len(checked) != 2 || checked[0] != "alice" || checked[1] != "ztt" {
		t.Fatalf("unexpected checked accounts: %#v", checked)
	}
	if len(broadcasted) != 2 || broadcasted[0] != "alice" || broadcasted[1] != "ztt" {
		t.Fatalf("unexpected broadcast accounts: %#v", broadcasted)
	}
}

func TestMonitorRunCheckCycleSkipsBroadcastWithoutRegisteredAccounts(t *testing.T) {
	engine := newTestMonitorEngine()

	performCount := 0
	broadcastCount := 0
	engine.OnPerformCheck = func(account string) *MonitorResult {
		performCount++
		return &MonitorResult{
			Account:   account,
			CheckedAt: time.Now(),
		}
	}
	engine.OnExecuteBroadcast = func(decision BroadcastDecision) {
		broadcastCount++
	}

	engine.runCheckCycle()

	if performCount != 1 {
		t.Fatalf("expected one internal check when registry empty, got %d", performCount)
	}
	if broadcastCount != 0 {
		t.Fatalf("expected no broadcasts when registry empty, got %d", broadcastCount)
	}
}
