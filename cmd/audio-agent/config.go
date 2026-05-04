package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type SpeechToTextModelConfig struct {
	Model string `json:"model"`
}

type TextToSpeechModelConfig struct {
	Model                string   `json:"model"`
	DefaultVoice         string   `json:"default_voice,omitempty"`
	ResponseFormat       string   `json:"response_format,omitempty"`
	LanguageBoost        string   `json:"language_boost,omitempty"`
	Speed                float64  `json:"speed,omitempty"`
	Volume               float64  `json:"volume,omitempty"`
	Pitch                int      `json:"pitch,omitempty"`
	EnglishNormalization *bool    `json:"english_normalization,omitempty"`
	SampleRate           int      `json:"sample_rate,omitempty"`
	Bitrate              int      `json:"bitrate,omitempty"`
	Channel              int      `json:"channel,omitempty"`
	PronunciationTone    []string `json:"pronunciation_tone,omitempty"`
}

type MusicGenerationModelConfig struct {
	Model           string `json:"model"`
	ResponseFormat  string `json:"response_format,omitempty"`
	SampleRate      int    `json:"sample_rate,omitempty"`
	Bitrate         int    `json:"bitrate,omitempty"`
	IsInstrumental  bool   `json:"is_instrumental,omitempty"`
	LyricsOptimizer bool   `json:"lyrics_optimizer,omitempty"`
	AIGCWatermark   *bool  `json:"aigc_watermark,omitempty"`
}

type AudioProviderConfig struct {
	BaseURL             string                                `json:"base_url"`
	APIKey              string                                `json:"api_key"`
	SpeechToTextPath    string                                `json:"speech_to_text_path"`
	TextToSpeechPath    string                                `json:"text_to_speech_path"`
	MusicGenerationPath string                                `json:"music_generation_path,omitempty"`
	STTModels           map[string]SpeechToTextModelConfig    `json:"speech_to_text_models,omitempty"`
	TTSModels           map[string]TextToSpeechModelConfig    `json:"text_to_speech_models,omitempty"`
	MusicModels         map[string]MusicGenerationModelConfig `json:"music_generation_models,omitempty"`
}

type AudioModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type Config struct {
	ServerURL string `json:"server_url"`
	AuthToken string `json:"auth_token"`
	AgentName string `json:"agent_name"`

	MaxConcurrent     int `json:"max_concurrent"`
	RequestTimeoutSec int `json:"request_timeout_sec"`

	Providers    map[string]AudioProviderConfig `json:"providers"`
	SpeechToText AudioModelRef                  `json:"speech_to_text"`
	TextToSpeech AudioModelRef                  `json:"text_to_speech"`
	TextToMusic  AudioModelRef                  `json:"text_to_music,omitempty"`

	ProtectedFiles []string `json:"protected_files,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		ServerURL:         "ws://127.0.0.1:10086/ws/uap",
		AgentName:         "audio-agent",
		MaxConcurrent:     3,
		RequestTimeoutSec: 180,
		Providers: map[string]AudioProviderConfig{
			"openai": {
				BaseURL:          "https://api.openai.com/v1",
				APIKey:           "",
				SpeechToTextPath: "/audio/transcriptions",
				TextToSpeechPath: "/audio/speech",
				STTModels: map[string]SpeechToTextModelConfig{
					"default": {Model: "gpt-4o-mini-transcribe"},
				},
				TTSModels: map[string]TextToSpeechModelConfig{
					"default": {Model: "gpt-4o-mini-tts", DefaultVoice: "alloy", ResponseFormat: "mp3"},
				},
			},
			"minimax": {
				BaseURL:             "https://api.minimaxi.com",
				APIKey:              "",
				TextToSpeechPath:    "/v1/t2a_v2",
				MusicGenerationPath: "/v1/music_generation",
				TTSModels: map[string]TextToSpeechModelConfig{
					"default": {
						Model:          "speech-2.8-hd",
						DefaultVoice:   "female-tianmei",
						ResponseFormat: "mp3",
						LanguageBoost:  "Chinese",
						Speed:          1,
						Volume:         1,
						Pitch:          0,
						SampleRate:     32000,
						Bitrate:        128000,
						Channel:        1,
					},
				},
				MusicModels: map[string]MusicGenerationModelConfig{
					"default": {
						Model:           "music-2.6",
						ResponseFormat:  "mp3",
						SampleRate:      44100,
						Bitrate:         256000,
						LyricsOptimizer: true,
					},
				},
			},
		},
		SpeechToText:   AudioModelRef{Provider: "openai", Model: "default"},
		TextToSpeech:   AudioModelRef{Provider: "minimax", Model: "default"},
		TextToMusic:    AudioModelRef{Provider: "minimax", Model: "default"},
		ProtectedFiles: []string{"audio-agent.json"},
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
	applyConfigDefaults(cfg)
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url is required")
	}
	if cfg.AgentName == "" {
		cfg.AgentName = "audio-agent"
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
	if _, _, err := cfg.ResolveSTT(); err != nil {
		return nil, err
	}
	if _, _, err := cfg.ResolveTTS(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyConfigDefaults(cfg *Config) {
	defaults := DefaultConfig()
	if strings.TrimSpace(cfg.TextToMusic.Provider) == "" {
		cfg.TextToMusic = defaults.TextToMusic
	} else if strings.TrimSpace(cfg.TextToMusic.Model) == "" {
		cfg.TextToMusic.Model = defaults.TextToMusic.Model
	}
	if cfg.Providers == nil {
		cfg.Providers = defaults.Providers
		return
	}
	for name, defaultProvider := range defaults.Providers {
		provider, ok := cfg.Providers[name]
		if !ok {
			cfg.Providers[name] = defaultProvider
			continue
		}
		if strings.TrimSpace(provider.BaseURL) == "" {
			provider.BaseURL = defaultProvider.BaseURL
		}
		if strings.TrimSpace(provider.SpeechToTextPath) == "" {
			provider.SpeechToTextPath = defaultProvider.SpeechToTextPath
		}
		if strings.TrimSpace(provider.TextToSpeechPath) == "" {
			provider.TextToSpeechPath = defaultProvider.TextToSpeechPath
		}
		if strings.TrimSpace(provider.MusicGenerationPath) == "" {
			provider.MusicGenerationPath = defaultProvider.MusicGenerationPath
		}
		if provider.STTModels == nil {
			provider.STTModels = defaultProvider.STTModels
		} else {
			for modelName, model := range defaultProvider.STTModels {
				if _, exists := provider.STTModels[modelName]; !exists {
					provider.STTModels[modelName] = model
				}
			}
		}
		if provider.TTSModels == nil {
			provider.TTSModels = defaultProvider.TTSModels
		} else {
			for modelName, model := range defaultProvider.TTSModels {
				if _, exists := provider.TTSModels[modelName]; !exists {
					provider.TTSModels[modelName] = model
				}
			}
		}
		if provider.MusicModels == nil {
			provider.MusicModels = defaultProvider.MusicModels
		} else {
			for modelName, model := range defaultProvider.MusicModels {
				if _, exists := provider.MusicModels[modelName]; !exists {
					provider.MusicModels[modelName] = model
				}
			}
		}
		cfg.Providers[name] = provider
	}
}

func (c *Config) ResolveSTT() (*AudioProviderConfig, *SpeechToTextModelConfig, error) {
	provider, ok := c.Providers[c.SpeechToText.Provider]
	if !ok {
		return nil, nil, fmt.Errorf("speech_to_text provider not found: %s", c.SpeechToText.Provider)
	}
	model, ok := provider.STTModels[c.SpeechToText.Model]
	if !ok {
		return nil, nil, fmt.Errorf("speech_to_text model not found: %s/%s", c.SpeechToText.Provider, c.SpeechToText.Model)
	}
	return &provider, &model, nil
}

func (c *Config) ResolveTTS() (*AudioProviderConfig, *TextToSpeechModelConfig, error) {
	provider, ok := c.Providers[c.TextToSpeech.Provider]
	if !ok {
		return nil, nil, fmt.Errorf("text_to_speech provider not found: %s", c.TextToSpeech.Provider)
	}
	model, ok := provider.TTSModels[c.TextToSpeech.Model]
	if !ok {
		return nil, nil, fmt.Errorf("text_to_speech model not found: %s/%s", c.TextToSpeech.Provider, c.TextToSpeech.Model)
	}
	return &provider, &model, nil
}

func (c *Config) ResolveMusic() (*AudioProviderConfig, *MusicGenerationModelConfig, error) {
	provider, ok := c.Providers[c.TextToMusic.Provider]
	if !ok {
		return nil, nil, fmt.Errorf("text_to_music provider not found: %s", c.TextToMusic.Provider)
	}
	model, ok := provider.MusicModels[c.TextToMusic.Model]
	if !ok {
		return nil, nil, fmt.Errorf("text_to_music model not found: %s/%s", c.TextToMusic.Provider, c.TextToMusic.Model)
	}
	return &provider, &model, nil
}
