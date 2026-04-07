package main

import (
	"strings"
	"testing"
)

func TestBuildCronNotifyTargetsDualDelivery(t *testing.T) {
	targets, errs := buildCronNotifyTargets("ztt", "ztt", "wechat-wechat-agent", "app-app-agent")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].Channel != "wechat" || targets[0].To != "ztt" {
		t.Fatalf("unexpected wechat target: %+v", targets[0])
	}
	if targets[1].Channel != "app" || targets[1].To != "ztt" {
		t.Fatalf("unexpected app target: %+v", targets[1])
	}
}

func TestBuildCronNotifyTargetsFallbackAppUserFromWechat(t *testing.T) {
	targets, errs := buildCronNotifyTargets("", "ztt", "wechat-wechat-agent", "app-app-agent")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	if targets[1].Channel != "app" || targets[1].To != "ztt" {
		t.Fatalf("expected app fallback to wechat user, got %+v", targets[1])
	}
}

func TestBuildCronNotifyTargetsMissingAppAgent(t *testing.T) {
	targets, errs := buildCronNotifyTargets("ztt", "ztt", "wechat-wechat-agent", "")
	if len(targets) != 1 {
		t.Fatalf("expected only wechat target, got %d", len(targets))
	}
	if targets[0].Channel != "wechat" {
		t.Fatalf("expected wechat target, got %+v", targets[0])
	}
	if len(errs) != 1 || !strings.Contains(errs[0], "no app-agent online") {
		t.Fatalf("expected app-agent error, got %v", errs)
	}
}

func TestBuildCronNotifyTargetsNoTarget(t *testing.T) {
	targets, errs := buildCronNotifyTargets("", "", "wechat-wechat-agent", "app-app-agent")
	if len(targets) != 0 {
		t.Fatalf("expected no targets, got %d", len(targets))
	}
	if len(errs) != 1 || errs[0] != "no delivery target" {
		t.Fatalf("expected no delivery target error, got %v", errs)
	}
}
