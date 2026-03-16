package agentbase

import (
	"os"
	"testing"
)

func TestLoadKeyValueConfig(t *testing.T) {
	// 创建临时配置文件
	content := `# Test config
server_url=ws://127.0.0.1:10086/ws
agent_name=test-agent
max_concurrent=5

# Comment line
auth_token=test-token-123
`
	tmpfile := t.TempDir() + "/test.conf"
	if err := writeFile(tmpfile, content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	config, err := LoadKeyValueConfig(tmpfile)
	if err != nil {
		t.Fatalf("LoadKeyValueConfig failed: %v", err)
	}

	// 验证解析结果
	tests := []struct {
		key      string
		expected string
	}{
		{"server_url", "ws://127.0.0.1:10086/ws"},
		{"agent_name", "test-agent"},
		{"max_concurrent", "5"},
		{"auth_token", "test-token-123"},
	}

	for _, tt := range tests {
		if got := config[tt.key]; got != tt.expected {
			t.Errorf("config[%s] = %s, want %s", tt.key, got, tt.expected)
		}
	}
}

func TestGetString(t *testing.T) {
	config := map[string]string{
		"key1": "value1",
		"key2": "",
	}

	if got := GetString(config, "key1", "default"); got != "value1" {
		t.Errorf("GetString(key1) = %s, want value1", got)
	}

	if got := GetString(config, "key2", "default"); got != "" {
		t.Errorf("GetString(key2) = %s, want empty", got)
	}

	if got := GetString(config, "missing", "default"); got != "default" {
		t.Errorf("GetString(missing) = %s, want default", got)
	}
}

func TestGetInt(t *testing.T) {
	config := map[string]string{
		"port":    "8080",
		"invalid": "abc",
	}

	if got := GetInt(config, "port", 0); got != 8080 {
		t.Errorf("GetInt(port) = %d, want 8080", got)
	}

	if got := GetInt(config, "invalid", 9999); got != 9999 {
		t.Errorf("GetInt(invalid) = %d, want 9999", got)
	}

	if got := GetInt(config, "missing", 3000); got != 3000 {
		t.Errorf("GetInt(missing) = %d, want 3000", got)
	}
}

func TestGetBool(t *testing.T) {
	config := map[string]string{
		"enabled":  "true",
		"disabled": "false",
		"yes":      "yes",
		"no":       "no",
		"one":      "1",
		"zero":     "0",
		"invalid":  "maybe",
	}

	tests := []struct {
		key      string
		expected bool
	}{
		{"enabled", true},
		{"disabled", false},
		{"yes", true},
		{"no", false},
		{"one", true},
		{"zero", false},
		{"invalid", false}, // 默认值
		{"missing", true},  // 默认值
	}

	for _, tt := range tests {
		defaultVal := true
		if tt.key == "invalid" {
			defaultVal = false // 使用 false 作为默认值以测试无效值回退
		}
		if got := GetBool(config, tt.key, defaultVal); got != tt.expected {
			t.Errorf("GetBool(%s) = %v, want %v", tt.key, got, tt.expected)
		}
	}
}

func TestMustGetString(t *testing.T) {
	config := map[string]string{
		"key1": "value1",
		"key2": "",
	}

	if _, err := MustGetString(config, "key1"); err != nil {
		t.Errorf("MustGetString(key1) unexpected error: %v", err)
	}

	if _, err := MustGetString(config, "key2"); err == nil {
		t.Error("MustGetString(key2) expected error for empty value")
	}

	if _, err := MustGetString(config, "missing"); err == nil {
		t.Error("MustGetString(missing) expected error")
	}
}

func TestMustGetInt(t *testing.T) {
	config := map[string]string{
		"port":    "8080",
		"invalid": "abc",
	}

	if got, err := MustGetInt(config, "port"); err != nil || got != 8080 {
		t.Errorf("MustGetInt(port) = %d, %v; want 8080, nil", got, err)
	}

	if _, err := MustGetInt(config, "invalid"); err == nil {
		t.Error("MustGetInt(invalid) expected error")
	}

	if _, err := MustGetInt(config, "missing"); err == nil {
		t.Error("MustGetInt(missing) expected error")
	}
}

// writeFile 辅助函数
func writeFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
