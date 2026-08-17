package goal

import (
	"encoding/json"
	"testing"
)

func TestTaskSourceIDRoundTrip(t *testing.T) {
	task := Task{ID: "child", SourceTaskID: "parent"}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SourceTaskID != "parent" {
		t.Fatalf("source task id = %q", decoded.SourceTaskID)
	}
}
