package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type VideoModelConfig struct {
	Model            string `json:"model"`
	Duration         int    `json:"duration,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	PromptOptimizer  *bool  `json:"prompt_optimizer,omitempty"`
	FastPretreatment *bool  `json:"fast_pretreatment,omitempty"`
	AIGCWatermark    *bool  `json:"aigc_watermark,omitempty"`
}

type VideoProviderConfig struct {
	BaseURL             string                      `json:"base_url"`
	APIKey              string                      `json:"api_key"`
	VideoGenerationPath string                      `json:"video_generation_path"`
	QueryPath           string                      `json:"query_path"`
	FileRetrievePath    string                      `json:"file_retrieve_path"`
	Models              map[string]VideoModelConfig `json:"models,omitempty"`
}

type VideoModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type Config struct {
	ServerURL string `json:"server_url"`
	AuthToken string `json:"auth_token"`
	AgentName string `json:"agent_name"`

	MaxConcurrent     int `json:"max_concurrent"`
	RequestTimeoutSec int `json:"request_timeout_sec"`
	PollIntervalSec   int `json:"poll_interval_sec"`
	MaxPollAttempts   int `json:"max_poll_attempts"`
	MaxDownloadBytes  int `json:"max_download_bytes"`

	Providers       map[string]VideoProviderConfig `json:"providers"`
	VideoGeneration VideoModelRef                  `json:"video_generation"`

	ProtectedFiles []string `json:"protected_files,omitempty"`
}

func DefaultConfig() *Config {
	promptOptimizer := true
	fastPretreatment := false
	aigcWatermark := false
	return &Config{
		ServerURL:         "ws://127.0.0.1:10086/ws/uap",
		AgentName:         "video-agent",
		MaxConcurrent:     1,
		RequestTimeoutSec: 900,
		PollIntervalSec:   5,
		MaxPollAttempts:   120,
		MaxDownloadBytes:  200 << 20,
		Providers: map[string]VideoProviderConfig{
			"minimax": {
				BaseURL:             "https://api.minimaxi.com",
				APIKey:              "",
				VideoGenerationPath: "/v1/video_generation",
				QueryPath:           "/v1/query/video_generation",
				FileRetrievePath:    "/v1/files/retrieve",
				Models: map[string]VideoModelConfig{
					"default": {
						Model:            "MiniMax-Hailuo-2.3",
						Duration:         6,
						Resolution:       "768P",
						PromptOptimizer:  &promptOptimizer,
						FastPretreatment: &fastPretreatment,
						AIGCWatermark:    &aigcWatermark,
					},
					"fast": {
						Model:            "MiniMax-Hailuo-2.3-Fast",
						Duration:         6,
						Resolution:       "768P",
						PromptOptimizer:  &promptOptimizer,
						FastPretreatment: &fastPretreatment,
						AIGCWatermark:    &aigcWatermark,
					},
				},
			},
		},
		VideoGeneration: VideoModelRef{Provider: "minimax", Model: "default"},
		ProtectedFiles:  []string{"video-agent.json"},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %v", err)
	}
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url is required")
	}
	if cfg.AgentName == "" {
		cfg.AgentName = "video-agent"
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 1
	}
	if cfg.RequestTimeoutSec <= 0 {
		cfg.RequestTimeoutSec = 900
	}
	if cfg.PollIntervalSec <= 0 {
		cfg.PollIntervalSec = 5
	}
	if cfg.MaxPollAttempts <= 0 {
		cfg.MaxPollAttempts = 120
	}
	if cfg.MaxDownloadBytes <= 0 {
		cfg.MaxDownloadBytes = 200 << 20
	}
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("providers is required")
	}
	if _, _, err := cfg.ResolveVideo(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) ResolveVideo() (*VideoProviderConfig, *VideoModelConfig, error) {
	providerName := strings.TrimSpace(c.VideoGeneration.Provider)
	modelName := strings.TrimSpace(c.VideoGeneration.Model)
	provider, ok := c.Providers[providerName]
	if !ok {
		return nil, nil, fmt.Errorf("video_generation provider not found: %s", providerName)
	}
	model, ok := provider.Models[modelName]
	if !ok {
		return nil, nil, fmt.Errorf("video_generation model not found: %s/%s", providerName, modelName)
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return nil, nil, fmt.Errorf("video provider api_key is required: %s", providerName)
	}
	if strings.TrimSpace(model.Model) == "" {
		return nil, nil, fmt.Errorf("video model name is required: %s/%s", providerName, modelName)
	}
	return &provider, &model, nil
}
