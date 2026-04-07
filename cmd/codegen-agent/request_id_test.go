package main

import "testing"

func TestTaskPayloadCarriesRequestID(t *testing.T) {
	task := TaskAssignPayload{
		SessionID: "tc_1",
		Project:   "demo",
		Prompt:    "hello",
		RequestID: "req_123",
	}
	if task.RequestID != "req_123" {
		t.Fatalf("unexpected request id: %q", task.RequestID)
	}
}
