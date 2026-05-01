package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// BroadcastConfig 单条播报场景配置
type BroadcastConfig struct {
	ID          string   `json:"id"`           // 唯一标识，如 "morning_greeting"
	Trigger     string   `json:"trigger"`      // 触发方式: "time_range" | "data_change" | "interval"
	TimeStart   string   `json:"time_start"`   // time_range 开始时间 "HH:MM"
	TimeEnd     string   `json:"time_end"`     // time_range 结束时间 "HH:MM"
	DataSources []string `json:"data_sources"` // data_change 需要查询的数据源，如 ["blog_new_since_last_check", "todo_count"]
	IntervalSec int      `json:"interval_sec"` // interval 触发间隔秒数
	MaxPerDay   int      `json:"max_per_day"`  // 每日最大播报次数 (0=不限)
	MinCount    int      `json:"min_count"`    // data_change 触发所需的最小变化数量
	Template    string   `json:"template"`     // 播报文案模板，如 "检测到{{new_blog_count}}篇新博客"
	Expression  string   `json:"expression"`   // Cortana 表情: happy/sad/surprised
	Motion      string   `json:"motion"`       // Cortana 动作: IdleWave/IdleAlt/Tap/Idle
}

// Config cortana-agent 配置
type Config struct {
	ServerURL            string            `json:"server_url"`             // Gateway WebSocket URL
	GatewayHTTPURL       string            `json:"gateway_http_url"`       // Gateway HTTP URL（用于 ToolCatalog 发现）
	AuthToken            string            `json:"auth_token"`             // 认证令牌
	AgentName            string            `json:"agent_name"`             // Agent 显示名称
	BlogAgentID          string            `json:"blog_agent_id"`          // blog-agent ID
	CronAgentID          string            `json:"cron_agent_id"`          // cron-agent ID
	AudioAgentID         string            `json:"audio_agent_id"`         // audio-agent ID (TTS)
	AppAgentID           string            `json:"app_agent_id"`           // app-agent ID (推送)
	CheckIntervalSec     int               `json:"check_interval_sec"`     // 数据检查间隔秒数
	BroadcastCooldownSec int               `json:"broadcast_cooldown_sec"` // 播报冷却秒数
	QuietHoursStart      string            `json:"quiet_hours_start"`      // 免打扰开始 "HH:MM"
	QuietHoursEnd        string            `json:"quiet_hours_end"`        // 免打扰结束 "HH:MM"
	Broadcasts           []BroadcastConfig `json:"broadcasts"`             // 播报场景配置（语音库）
	ProtectedFiles       []string          `json:"protected_files,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:            "ws://127.0.0.1:10086/ws/uap",
		GatewayHTTPURL:       "http://127.0.0.1:10086",
		AgentName:            "cortana-agent",
		BlogAgentID:          "blog-agent",
		CronAgentID:          "cron-agent",
		AudioAgentID:         "audio-agent",
		AppAgentID:           "app-agent",
		CheckIntervalSec:     120,
		BroadcastCooldownSec: 600,
		ProtectedFiles:       []string{"cortana-agent.json"},
		Broadcasts: []BroadcastConfig{
			{
				ID:         "morning_greeting",
				Trigger:    "time_range",
				TimeStart:  "06:00",
				TimeEnd:    "10:00",
				MaxPerDay:  1,
				Template:   "早上好！又是充满活力的一天。",
				Expression: "happy",
				Motion:     "IdleWave",
			},
			{
				ID:         "evening_summary",
				Trigger:    "time_range",
				TimeStart:  "20:00",
				TimeEnd:    "23:00",
				MaxPerDay:  1,
				Template:   "晚上好！今天过得怎么样？别忘了回顾一下今天的收获。",
				Expression: "happy",
				Motion:     "IdleAlt",
			},
			{
				ID:          "new_blog_alert",
				Trigger:     "data_change",
				DataSources: []string{"blog_new_count"},
				MinCount:    1,
				MaxPerDay:   5,
				Template:    "主人，检测到{{blog_new_count}}篇新博客文章，要去看看吗？",
				Expression:  "surprised",
				Motion:      "Tap",
			},
			{
				ID:          "todo_overdue_reminder",
				Trigger:     "data_change",
				DataSources: []string{"todo_overdue_count"},
				MinCount:    1,
				MaxPerDay:   3,
				Template:    "你还有{{todo_overdue_count}}个待办事项需要处理哦。",
				Expression:  "sad",
				Motion:      "IdleAlt",
			},
			{
				ID:         "exercise_reminder",
				Trigger:    "time_range",
				TimeStart:  "17:00",
				TimeEnd:    "20:00",
				MaxPerDay:  1,
				Template:   "今天的运动时间到了！别忘了保持健康。",
				Expression: "surprised",
				Motion:     "Tap",
			},
			{
				ID:          "agent_offline_alert",
				Trigger:     "data_change",
				DataSources: []string{"agent_offline"},
				MinCount:    1,
				MaxPerDay:   10,
				Template:    "{{offline_agent_name}}服务已离线，请注意检查。",
				Expression:  "sad",
				Motion:      "Idle",
			},
		},
	}
}

// LoadConfig 从 JSON 文件加载配置
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if cfg.AgentName == "" {
		cfg.AgentName = "cortana-agent"
	}
	if cfg.BlogAgentID == "" {
		cfg.BlogAgentID = "blog-agent"
	}
	if cfg.CronAgentID == "" {
		cfg.CronAgentID = "cron-agent"
	}
	if cfg.AudioAgentID == "" {
		cfg.AudioAgentID = "audio-agent"
	}
	if cfg.AppAgentID == "" {
		cfg.AppAgentID = "app-agent"
	}
	if cfg.CheckIntervalSec <= 0 {
		cfg.CheckIntervalSec = 120
	}
	if cfg.BroadcastCooldownSec <= 0 {
		cfg.BroadcastCooldownSec = 600
	}
	if cfg.GatewayHTTPURL == "" {
		cfg.GatewayHTTPURL = deriveGatewayHTTPURL(cfg.ServerURL)
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "ws://127.0.0.1:10086/ws/uap"
	}
	if cfg.GatewayHTTPURL == "" {
		cfg.GatewayHTTPURL = "http://127.0.0.1:10086"
	}

	return cfg, nil
}

func deriveGatewayHTTPURL(serverURL string) string {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return ""
	}
	u, err := url.Parse(serverURL)
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return ""
	}
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	default:
		u.Scheme = "http"
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}
