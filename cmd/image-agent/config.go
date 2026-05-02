package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type VisionModelConfig struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

type ImageGenerationModelConfig struct {
	Model           string `json:"model"`
	Size            string `json:"size,omitempty"`
	AspectRatio     string `json:"aspect_ratio,omitempty"`
	ResponseFormat  string `json:"response_format,omitempty"`
	N               int    `json:"n,omitempty"`
	PromptOptimizer bool   `json:"prompt_optimizer,omitempty"`
	AIGCWatermark   bool   `json:"aigc_watermark,omitempty"`
}

type ImageProviderConfig struct {
	Kind                string                                `json:"kind,omitempty"`
	BaseURL             string                                `json:"base_url"`
	APIKey              string                                `json:"api_key"`
	VisionPath          string                                `json:"vision_path"`
	ImageGenerationPath string                                `json:"image_generation_path"`
	VisionModels        map[string]VisionModelConfig          `json:"vision_models,omitempty"`
	GenerationModels    map[string]ImageGenerationModelConfig `json:"generation_models,omitempty"`
}

type ImageModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type Config struct {
	ServerURL string `json:"server_url"`
	AuthToken string `json:"auth_token"`
	AgentName string `json:"agent_name"`

	MaxConcurrent     int `json:"max_concurrent"`
	RequestTimeoutSec int `json:"request_timeout_sec"`

	Providers   map[string]ImageProviderConfig `json:"providers"`
	ImageToText ImageModelRef                  `json:"image_to_text"`
	TextToImage ImageModelRef                  `json:"text_to_image"`

	ProtectedFiles []string `json:"protected_files,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		ServerURL:         "ws://127.0.0.1:10086/ws/uap",
		AgentName:         "image-agent",
		MaxConcurrent:     3,
		RequestTimeoutSec: 180,
		Providers: map[string]ImageProviderConfig{
			"openai": {
				Kind:                "openai",
				BaseURL:             "https://api.openai.com/v1",
				APIKey:              "",
				VisionPath:          "/chat/completions",
				ImageGenerationPath: "/images/generations",
				VisionModels: map[string]VisionModelConfig{
					"default": {Model: "gpt-4.1-mini", MaxTokens: 1024},
				},
				GenerationModels: map[string]ImageGenerationModelConfig{
					"default": {Model: "gpt-image-1", Size: "1024x1024"},
				},
			},
			"minimax": {
				Kind:                "minimax",
				BaseURL:             "https://api.minimaxi.com",
				APIKey:              "",
				ImageGenerationPath: "/v1/image_generation",
				GenerationModels: map[string]ImageGenerationModelConfig{
					"default": {
						Model:          "image-01",
						AspectRatio:    "1:1",
						ResponseFormat: "base64",
						N:              1,
					},
				},
			},
		},
		ImageToText:    ImageModelRef{Provider: "openai", Model: "default"},
		TextToImage:    ImageModelRef{Provider: "openai", Model: "default"},
		ProtectedFiles: []string{"image-agent.json"},
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
		cfg.AgentName = "image-agent"
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.RequestTimeoutSec <= 0 {
		cfg.RequestTimeoutSec = 180
	}
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("providers is required")
	}
	if _, _, err := cfg.ResolveGeneration(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) ResolveVision() (*ImageProviderConfig, *VisionModelConfig, error) {
	provider, ok := c.Providers[c.ImageToText.Provider]
	if !ok {
		return nil, nil, fmt.Errorf("image_to_text provider not found: %s", c.ImageToText.Provider)
	}
	if len(provider.VisionModels) == 0 {
		return nil, nil, fmt.Errorf("image_to_text provider has no vision models: %s", c.ImageToText.Provider)
	}
	model, ok := provider.VisionModels[c.ImageToText.Model]
	if !ok {
		return nil, nil, fmt.Errorf("image_to_text model not found: %s/%s", c.ImageToText.Provider, c.ImageToText.Model)
	}
	return &provider, &model, nil
}

func (c *Config) HasVisionTool() bool {
	_, _, err := c.ResolveVision()
	return err == nil
}

func (c *Config) ResolveGeneration() (*ImageProviderConfig, *ImageGenerationModelConfig, error) {
	provider, ok := c.Providers[c.TextToImage.Provider]
	if !ok {
		return nil, nil, fmt.Errorf("text_to_image provider not found: %s", c.TextToImage.Provider)
	}
	model, ok := provider.GenerationModels[c.TextToImage.Model]
	if !ok {
		return nil, nil, fmt.Errorf("text_to_image model not found: %s/%s", c.TextToImage.Provider, c.TextToImage.Model)
	}
	return &provider, &model, nil
}
