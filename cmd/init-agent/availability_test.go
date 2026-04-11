package main

import "testing"

func TestAggregateStatusPrefersRedThenYellow(t *testing.T) {
	if got := aggregateStatus([]CheckItem{{Status: StatusGreen}, {Status: StatusYellow}}); got != StatusYellow {
		t.Fatalf("expected yellow, got %s", got)
	}
	if got := aggregateStatus([]CheckItem{{Status: StatusGreen}, {Status: StatusRed}, {Status: StatusYellow}}); got != StatusRed {
		t.Fatalf("expected red, got %s", got)
	}
}

func TestResolveGatewayHTTPUsesConfiguredPort(t *testing.T) {
	got := resolveGatewayHTTP(map[string]map[string]any{
		"gateway": {"port": float64(19000)},
	})
	if got != "http://127.0.0.1:19000" {
		t.Fatalf("unexpected gateway http url: %s", got)
	}
}
