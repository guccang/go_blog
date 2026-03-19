package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// AgentConfig agent 配置
type AgentConfig struct {
	ServerURL        string   `json:"server_url"`          // gateway WebSocket 地址
	AgentName        string   `json:"agent_name"`          // agent 名称
	AgentType        string   `json:"agent_type"`          // agent 类型，默认 "acp"
	AuthToken        string   `json:"auth_token"`          // 认证令牌
	GoBackendAgentID string   `json:"go_backend_agent_id"` // go_blog-agent 在 gateway 中的 ID
	ACPAgentCmd      string   `json:"acp_agent_cmd"`       // ACP agent 命令，默认 "npx"
	ACPAgentArgs     []string `json:"acp_agent_args"`      // ACP agent 参数，默认 ["-y", "@zed-industries/claude-agent-acp@latest"]
	Workspaces       []string `json:"workspaces"`          // 项目工作区目录列表
	MaxConcurrent    int      `json:"max_concurrent"`      // 最大并发数，默认 2
	AnalysisTimeout  int      `json:"analysis_timeout"`    // ACP 分析超时（秒），默认 3600
}

// LoadConfig 从 JSON 配置文件加载配置
func LoadConfig(path string) (*AgentConfig, error) {
	cfg := &AgentConfig{
		ACPAgentCmd:     "npx",
		ACPAgentArgs:    []string{"-y", "@zed-industries/claude-agent-acp@latest"},
		MaxConcurrent:   2,
		AnalysisTimeout: 3600,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %v", err)
	}

	// 必填字段校验
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url is required")
	}
	if len(cfg.Workspaces) == 0 {
		return nil, fmt.Errorf("workspaces is required")
	}

	// 默认值填充
	if cfg.AgentName == "" {
		cfg.AgentName, _ = os.Hostname()
	}
	if cfg.AgentType == "" {
		cfg.AgentType = "acp"
	}
	if cfg.ACPAgentCmd == "" {
		cfg.ACPAgentCmd = "npx"
	}
	if len(cfg.ACPAgentArgs) == 0 {
		cfg.ACPAgentArgs = []string{"-y", "@zed-industries/claude-agent-acp@latest"}
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 2
	}
	if cfg.AnalysisTimeout <= 0 {
		cfg.AnalysisTimeout = 3600
	}
	if cfg.GoBackendAgentID == "" {
		cfg.GoBackendAgentID = "go_blog"
	}

	return cfg, nil
}
