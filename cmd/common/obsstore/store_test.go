package obsstore

import "testing"

func TestJoinKey(t *testing.T) {
	got := joinKey("/root/", "/nested/path/", "file.apk")
	if got != "root/nested/path/file.apk" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := Config{
		Endpoint:  "https://obs.example.com",
		Bucket:    "bucket",
		AccessKey: "ak",
		SecretKey: "sk",
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
}
