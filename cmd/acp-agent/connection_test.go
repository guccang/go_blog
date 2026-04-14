package main

import "testing"

func TestInterpretToolResultTreatsStructuredFailureAsFailure(t *testing.T) {
	ok, errText := interpretToolResult(`{"success":false,"session_id":"acp_1","error":"default settings file not found"}`)
	if ok {
		t.Fatalf("expected structured failure to be treated as failure")
	}
	if errText != "default settings file not found" {
		t.Fatalf("unexpected error text: %q", errText)
	}
}

func TestInterpretToolResultKeepsStructuredSuccess(t *testing.T) {
	ok, errText := interpretToolResult(`{"success":true,"message":"done","data":{"session_id":"acp_1"}}`)
	if !ok {
		t.Fatalf("expected structured success, got error=%q", errText)
	}
}

func TestInterpretToolResultIgnoresPlainText(t *testing.T) {
	ok, errText := interpretToolResult("plain text output")
	if !ok || errText != "" {
		t.Fatalf("expected plain text output to pass through, got ok=%v err=%q", ok, errText)
	}
}
