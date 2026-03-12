package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"runtime"
)

// Config execute-code-agent 配置
type Config struct {
	ServerURL        string           `json:"server_url"`          // ws://127.0.0.1:10086/ws/uap
	GatewayHTTP      string           `json:"gateway_http"`        // http://127.0.0.1:10086
	AuthToken        string           `json:"auth_token"`
	AgentName        string           `json:"agent_name"`          // "execute-code"
	GoBackendAgentID string           `json:"go_backend_agent_id"` // "go_blog"
	MaxConcurrent    int              `json:"max_concurrent"`      // 默认 3
	PythonPath       string           `json:"python_path"`         // 默认 "python3"
	MaxExecTimeSec   int              `json:"max_exec_time_sec"`   // 默认 120
	MaxOutputSize    int              `json:"max_output_size"`     // 默认 50000 字符
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:      "ws://127.0.0.1:10086/ws/uap",
		GatewayHTTP:    "http://127.0.0.1:10086",
		AgentName:      "execute-code",
		GoBackendAgentID: "go_blog",
		MaxConcurrent:  3,
		PythonPath:     detectPython(),
		MaxExecTimeSec: 120,
		MaxOutputSize:  50000,
	}
}

// LoadConfig 从 JSON 文件加载配置
func LoadConfig(path string) *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[Config] 配置文件 %s 不存在，使用默认配置", path)
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		log.Printf("[Config] 解析配置文件失败: %v，使用默认配置", err)
		return DefaultConfig()
	}

	// 填充默认值
	if cfg.AgentName == "" {
		cfg.AgentName = "execute-code"
	}
	if cfg.GoBackendAgentID == "" {
		cfg.GoBackendAgentID = "go_blog"
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.PythonPath == "" {
		cfg.PythonPath = detectPython()
	}
	if cfg.MaxExecTimeSec <= 0 {
		cfg.MaxExecTimeSec = 120
	}
	if cfg.MaxOutputSize <= 0 {
		cfg.MaxOutputSize = 50000
	}

	return cfg
}

// detectPython 自动检测可用的 Python 命令
func detectPython() string {
	// Windows 上优先 python，Linux/macOS 优先 python3
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	} else {
		candidates = []string{"python3", "python"}
	}
	for _, cmd := range candidates {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd
		}
	}
	// 兜底
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}
