package main

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// StorageConfig 存储配置
type StorageConfig struct {
	Type      string `json:"type"`       // "json"
	Path      string `json:"path"`       // 任务数据文件路径
	BackupDir string `json:"backup_dir"` // 备份目录
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	MaxConcurrent int    `json:"max_concurrent"` // 最大并发任务数
	Timezone      string `json:"timezone"`       // 时区
}

// LLMAgentConfig llm-agent集成配置
type LLMAgentConfig struct {
	AgentID   string `json:"agent_id"`   // llm-agent的agent ID
	Timeout   int    `json:"timeout_sec"` // 任务超时秒数
}

// NotificationsConfig 通知配置
type NotificationsConfig struct {
	Enabled        bool   `json:"enabled"`
	Channel        string `json:"channel"`          // "wechat"
	SuccessTemplate string `json:"success_template"` // 成功通知模板
	FailureTemplate string `json:"failure_template"` // 失败通知模板
}

// Config corn-agent 配置
type Config struct {
	GatewayURL    string              `json:"gateway_url"`    // ws://127.0.0.1:9000/ws/uap
	AuthToken     string              `json:"auth_token"`
	AgentID       string              `json:"agent_id"`       // corn-agent
	Storage       StorageConfig       `json:"storage"`
	Scheduler     SchedulerConfig     `json:"scheduler"`
	LLMAgent      LLMAgentConfig      `json:"llm_agent"`
	Notifications NotificationsConfig `json:"notifications"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		GatewayURL: "ws://127.0.0.1:9000/ws/uap",
		AgentID:    "corn-agent",
		Storage: StorageConfig{
			Type:      "json",
			Path:      "./data/tasks.json",
			BackupDir: "./data/backups",
		},
		Scheduler: SchedulerConfig{
			MaxConcurrent: 5,
			Timezone:      "Asia/Shanghai",
		},
		LLMAgent: LLMAgentConfig{
			AgentID: "llm-agent",
			Timeout: 300,
		},
		Notifications: NotificationsConfig{
			Enabled:        true,
			Channel:        "wechat",
			SuccessTemplate: "任务执行成功",
			FailureTemplate: "任务执行失败",
		},
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
	if cfg.GatewayURL == "" {
		cfg.GatewayURL = "ws://127.0.0.1:9000/ws/uap"
	}
	if cfg.AgentID == "" {
		cfg.AgentID = "corn-agent"
	}
	if cfg.Storage.Type == "" {
		cfg.Storage.Type = "json"
	}
	if cfg.Storage.Path == "" {
		cfg.Storage.Path = "./data/tasks.json"
	}
	if cfg.Storage.BackupDir == "" {
		cfg.Storage.BackupDir = "./data/backups"
	}
	if cfg.Scheduler.MaxConcurrent <= 0 {
		cfg.Scheduler.MaxConcurrent = 5
	}
	if cfg.Scheduler.Timezone == "" {
		cfg.Scheduler.Timezone = "Asia/Shanghai"
	}
	if cfg.LLMAgent.AgentID == "" {
		cfg.LLMAgent.AgentID = "llm-agent"
	}
	if cfg.LLMAgent.Timeout <= 0 {
		cfg.LLMAgent.Timeout = 300
	}
	if cfg.Notifications.Channel == "" {
		cfg.Notifications.Channel = "wechat"
	}
	if cfg.Notifications.SuccessTemplate == "" {
		cfg.Notifications.SuccessTemplate = "任务执行成功"
	}
	if cfg.Notifications.FailureTemplate == "" {
		cfg.Notifications.FailureTemplate = "任务执行失败"
	}

	return cfg
}

// GetLocation 获取时区位置
func (c *Config) GetLocation() *time.Location {
	loc, err := time.LoadLocation(c.Scheduler.Timezone)
	if err != nil {
		log.Printf("[Config] 时区 %s 无效，使用本地时区: %v", c.Scheduler.Timezone, err)
		return time.Local
	}
	return loc
}