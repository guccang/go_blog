package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Config execute-code-agent 配置
type Config struct {
	ServerURL        string `json:"server_url"`   // ws://127.0.0.1:10086/ws/uap
	GatewayHTTP      string `json:"gateway_http"` // http://127.0.0.1:10086
	AuthToken        string `json:"auth_token"`
	AgentName        string `json:"agent_name"`          // "execute-code"
	GoBackendAgentID string `json:"go_backend_agent_id"` // "blog-agent"
	MaxConcurrent    int    `json:"max_concurrent"`      // 默认 3
	PythonPath       string `json:"python_path"`         // 默认 "python3"
	MaxExecTimeSec   int    `json:"max_exec_time_sec"`   // 默认 120
	MaxOutputSize    int    `json:"max_output_size"`     // 默认 50000 字符

	// 部署保护文件（deploy-agent 增量部署时跳过这些文件）
	ProtectedFiles []string `json:"protected_files,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:        "ws://127.0.0.1:10086/ws/uap",
		GatewayHTTP:      "http://127.0.0.1:10086",
		AgentName:        "execute-code",
		GoBackendAgentID: "blog-agent",
		MaxConcurrent:    3,
		PythonPath:       detectPython(),
		MaxExecTimeSec:   120,
		MaxOutputSize:    50000,

		ProtectedFiles: []string{"execute-code-agent.json"},
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
		cfg.GoBackendAgentID = "blog-agent"
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

// CheckPythonVersion 检查 Python 版本是否满足最低要求（3.6+，f-string 依赖）
// 返回版本字符串和错误，错误非 nil 时应拒绝启动
func CheckPythonVersion(pythonPath string) (string, error) {
	out, err := exec.Command(pythonPath, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("无法执行 %s --version: %v", pythonPath, err)
	}

	// 输出格式: "Python 3.10.12" 或 "Python 2.7.18"
	version := strings.TrimSpace(string(out))
	version = strings.TrimPrefix(version, "Python ")

	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version, fmt.Errorf("无法解析 Python 版本: %s", version)
	}

	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return version, fmt.Errorf("无法解析 Python 版本号: %s", version)
	}

	// 最低要求 Python 3.6（f-string、json.JSONDecodeError 等）
	if major < 3 || (major == 3 && minor < 6) {
		return version, fmt.Errorf("Python 版本过低: %s（最低要求 3.6，当前代码使用了 f-string 等 3.6+ 特性）", version)
	}

	return version, nil
}
