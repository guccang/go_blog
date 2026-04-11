package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config test-agent 配置。
type Config struct {
	GatewayURL        string        `json:"gateway_url"`
	GatewayHTTP       string        `json:"gateway_http"`
	AuthToken         string        `json:"auth_token"`
	AgentID           string        `json:"agent_id"`
	AgentName         string        `json:"agent_name"`
	OutputDir         string        `json:"output_dir"`
	SuiteDir          string        `json:"suite_dir"`
	StaticSuiteDir    string        `json:"static_suite_dir,omitempty"`
	LLMTraceDir       string        `json:"llm_trace_dir,omitempty"`
	DefaultTimeoutSec int           `json:"default_timeout_sec"`
	PollIntervalMs    int           `json:"poll_interval_ms"`
	SettleAfterMs     int           `json:"settle_after_ms"`
	CaptureAgents     bool          `json:"capture_agents"`
	CaptureHealth     bool          `json:"capture_health"`
	Dynamic           DynamicConfig `json:"dynamic"`
	Report            ReportConfig  `json:"report"`
}

// DynamicConfig 动态评估集配置。
type DynamicConfig struct {
	Enabled          bool     `json:"enabled"`
	GeneratorAgent   string   `json:"generator_agent"`
	Account          string   `json:"account,omitempty"`
	TimeoutSec       int      `json:"timeout_sec"`
	MaxScenarios     int      `json:"max_scenarios"`
	DenyToolKeywords []string `json:"deny_tool_keywords,omitempty"`
}

// ReportConfig 最终评分权重。
type ReportConfig struct {
	StaticWeight  int `json:"static_weight"`
	DynamicWeight int `json:"dynamic_weight"`
}

func DefaultConfig() *Config {
	return &Config{
		GatewayURL:        "ws://127.0.0.1:10086/ws/uap",
		GatewayHTTP:       "http://127.0.0.1:10086",
		AgentID:           "test-agent",
		AgentName:         "test-agent",
		OutputDir:         "runtime/test_runs",
		SuiteDir:          "suites",
		StaticSuiteDir:    "suites",
		DefaultTimeoutSec: 20,
		PollIntervalMs:    500,
		SettleAfterMs:     300,
		CaptureAgents:     true,
		CaptureHealth:     true,
		Dynamic: DynamicConfig{
			Enabled:        true,
			GeneratorAgent: "llm-agent",
			Account:        "test-agent",
			TimeoutSec:     30,
			MaxScenarios:   3,
			DenyToolKeywords: []string{
				"deploy",
				"delete",
				"destroy",
				"remove",
				"publish",
				"restart",
				"shutdown",
				"reboot",
				"ssh",
				"write",
			},
		},
		Report: ReportConfig{
			StaticWeight:  70,
			DynamicWeight: 30,
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if strings.TrimSpace(cfg.GatewayURL) == "" {
		return nil, fmt.Errorf("gateway_url is required")
	}
	if strings.TrimSpace(cfg.GatewayHTTP) == "" {
		return nil, fmt.Errorf("gateway_http is required")
	}
	if strings.TrimSpace(cfg.AgentID) == "" {
		cfg.AgentID = "test-agent"
	}
	if strings.TrimSpace(cfg.AgentName) == "" {
		cfg.AgentName = cfg.AgentID
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = "runtime/test_runs"
	}
	if strings.TrimSpace(cfg.SuiteDir) == "" {
		cfg.SuiteDir = "suites"
	}
	if strings.TrimSpace(cfg.StaticSuiteDir) == "" {
		cfg.StaticSuiteDir = cfg.SuiteDir
	}
	if cfg.DefaultTimeoutSec <= 0 {
		cfg.DefaultTimeoutSec = 20
	}
	if cfg.PollIntervalMs <= 0 {
		cfg.PollIntervalMs = 500
	}
	if cfg.SettleAfterMs < 0 {
		cfg.SettleAfterMs = 0
	}
	if strings.TrimSpace(cfg.Dynamic.GeneratorAgent) == "" {
		cfg.Dynamic.GeneratorAgent = "llm-agent"
	}
	if strings.TrimSpace(cfg.Dynamic.Account) == "" {
		cfg.Dynamic.Account = cfg.AgentID
	}
	if cfg.Dynamic.TimeoutSec <= 0 {
		cfg.Dynamic.TimeoutSec = 30
	}
	if cfg.Dynamic.MaxScenarios <= 0 {
		cfg.Dynamic.MaxScenarios = 3
	}
	if len(cfg.Dynamic.DenyToolKeywords) == 0 {
		cfg.Dynamic.DenyToolKeywords = append([]string(nil), DefaultConfig().Dynamic.DenyToolKeywords...)
	}
	if cfg.Report.StaticWeight < 0 {
		cfg.Report.StaticWeight = 0
	}
	if cfg.Report.DynamicWeight < 0 {
		cfg.Report.DynamicWeight = 0
	}
	if cfg.Report.StaticWeight == 0 && cfg.Report.DynamicWeight == 0 {
		cfg.Report.StaticWeight = 70
		cfg.Report.DynamicWeight = 30
	}
	return cfg, nil
}

func WriteDefaultConfig(path string, cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
