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

func TestConfigNormalizedAddsHTTPSWhenSchemeMissing(t *testing.T) {
	cfg := Config{
		Endpoint:       "obs.cn-north-4.myhuaweicloud.com",
		PublicEndpoint: "download.example.com",
	}
	got := cfg.normalized()
	if got.Endpoint != "https://obs.cn-north-4.myhuaweicloud.com" {
		t.Fatalf("unexpected endpoint: %q", got.Endpoint)
	}
	if got.PublicEndpoint != "https://download.example.com" {
		t.Fatalf("unexpected public endpoint: %q", got.PublicEndpoint)
	}
}

func TestConfigNormalizedKeepsExplicitScheme(t *testing.T) {
	cfg := Config{
		Endpoint: "http://127.0.0.1:9000",
	}
	got := cfg.normalized()
	if got.Endpoint != "http://127.0.0.1:9000" {
		t.Fatalf("unexpected endpoint: %q", got.Endpoint)
	}
}

func TestNormalizeSignedURLForcesHTTPSOnRemoteHost(t *testing.T) {
	got := normalizeSignedURL("http://obs.example.com/demo.apk?sig=1")
	if got != "https://obs.example.com/demo.apk?sig=1" {
		t.Fatalf("unexpected signed url: %q", got)
	}
}

func TestNormalizeSignedURLKeepsLoopbackHTTP(t *testing.T) {
	tests := []string{
		"http://127.0.0.1:9000/demo.apk?sig=1",
		"http://localhost:9000/demo.apk?sig=1",
	}
	for _, input := range tests {
		if got := normalizeSignedURL(input); got != input {
			t.Fatalf("expected loopback url unchanged, got %q for %q", got, input)
		}
	}
}

func TestPublicSignedURLRewritesHostToConfiguredPublicEndpoint(t *testing.T) {
	store := &Store{
		cfg: Config{
			PublicEndpoint: "https://download.example.com",
		},
	}
	got := store.publicSignedURL("https://bucket.obs.cn-north-4.myhuaweicloud.com/demo.apk?sig=1")
	want := "https://download.example.com/demo.apk?sig=1"
	if got != want {
		t.Fatalf("unexpected rewritten signed url: %q", got)
	}
}
