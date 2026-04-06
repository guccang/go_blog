package main

import (
	"encoding/json"
	"fmt"
	"os"
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

type AudioProviderConfig struct {
	BaseURL          string                             `json:"base_url"`
	APIKey           string                             `json:"api_key"`
	SpeechToTextPath string                             `json:"speech_to_text_path"`
	TextToSpeechPath string                             `json:"text_to_speech_path"`
	STTModels        map[string]SpeechToTextModelConfig `json:"speech_to_text_models,omitempty"`
	TTSModels        map[string]TextToSpeechModelConfig `json:"text_to_speech_models,omitempty"`
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
				BaseURL:          "https://api.minimaxi.com",
				APIKey:           "",
				TextToSpeechPath: "/v1/t2a_v2",
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
			},
		},
		SpeechToText:   AudioModelRef{Provider: "openai", Model: "default"},
		TextToSpeech:   AudioModelRef{Provider: "minimax", Model: "default"},
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
