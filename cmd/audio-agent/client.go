package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

var audioHTTPClient = &http.Client{}

type AudioClient struct {
	cfg *Config
}

func NewAudioClient(cfg *Config) *AudioClient {
	return &AudioClient{cfg: cfg}
}

type TranscribeParams struct {
	AudioBase64 string
	FileName    string
	Format      string
	Prompt      string
	Language    string
}

type SynthesizeParams struct {
	Text   string
	Voice  string
	Format string
}

func (c *AudioClient) Transcribe(ctx context.Context, params TranscribeParams) (map[string]any, error) {
	provider, model, err := c.cfg.ResolveSTT()
	if err != nil {
		return nil, err
	}
	start := time.Now()

	audioBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(params.AudioBase64))
	if err != nil {
		return nil, fmt.Errorf("decode audio_base64: %w", err)
	}
	fileName := strings.TrimSpace(params.FileName)
	if fileName == "" {
		fileName = "audio." + defaultString(strings.TrimSpace(params.Format), "mp3")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", model.Model)
	if strings.TrimSpace(params.Prompt) != "" {
		_ = writer.WriteField("prompt", strings.TrimSpace(params.Prompt))
	}
	if strings.TrimSpace(params.Language) != "" {
		_ = writer.WriteField("language", strings.TrimSpace(params.Language))
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("create multipart file: %w", err)
	}
	if _, err := part.Write(audioBytes); err != nil {
		return nil, fmt.Errorf("write audio file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodPost, joinURL(provider.BaseURL, provider.SpeechToTextPath), &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	log.Printf("[AudioClient] STT request provider=%s model=%s format=%s file=%s bytes=%d", c.cfg.SpeechToText.Provider, model.Model, strings.TrimSpace(params.Format), fileName, len(audioBytes))

	var result map[string]any
	if err := doJSON(req, &result); err != nil {
		log.Printf("[AudioClient] STT request failed provider=%s model=%s duration=%v err=%v", c.cfg.SpeechToText.Provider, model.Model, time.Since(start), err)
		return nil, err
	}
	log.Printf("[AudioClient] STT request done provider=%s model=%s duration=%v text_len=%d", c.cfg.SpeechToText.Provider, model.Model, time.Since(start), len(firstStringField(result, "text", "transcript", "content")))
	return result, nil
}

func (c *AudioClient) Synthesize(ctx context.Context, params SynthesizeParams) (map[string]any, error) {
	provider, model, err := c.cfg.ResolveTTS()
	if err != nil {
		return nil, err
	}

	voice, err := c.resolveConfiguredTTSVoice(model, params.Voice)
	if err != nil {
		return nil, err
	}
	format := strings.TrimSpace(params.Format)
	if format == "" {
		format = defaultString(model.ResponseFormat, "mp3")
	}

	if strings.EqualFold(c.cfg.TextToSpeech.Provider, "minimax") {
		return c.synthesizeMiniMaxHTTP(ctx, provider, model, params, voice, format)
	}
	return c.synthesizeStandardHTTP(ctx, provider, model, params, voice, format)
}

func (c *AudioClient) resolveConfiguredTTSVoice(model *TextToSpeechModelConfig, requestedVoice string) (string, error) {
	configuredVoice := strings.TrimSpace(model.DefaultVoice)
	requestedVoice = strings.TrimSpace(requestedVoice)

	if requestedVoice != "" {
		log.Printf("[AudioClient][ERROR] external TTS voice override ignored provider=%s model=%s requested_voice=%s configured_voice=%s",
			c.cfg.TextToSpeech.Provider, model.Model, requestedVoice, configuredVoice)
	}
	if configuredVoice == "" {
		return "", fmt.Errorf("text_to_speech default_voice is required for provider=%s model=%s", c.cfg.TextToSpeech.Provider, model.Model)
	}
	return configuredVoice, nil
}

func (c *AudioClient) synthesizeStandardHTTP(ctx context.Context, provider *AudioProviderConfig, model *TextToSpeechModelConfig, params SynthesizeParams, voice string, format string) (map[string]any, error) {
	start := time.Now()
	body := map[string]any{
		"model":           model.Model,
		"input":           params.Text,
		"voice":           voice,
		"response_format": format,
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodPost, joinURL(provider.BaseURL, provider.TextToSpeechPath), bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("Content-Type", "application/json")
	log.Printf("[AudioClient] TTS request provider=%s model=%s voice=%s format=%s text_len=%d", c.cfg.TextToSpeech.Provider, model.Model, voice, format, len(params.Text))

	resp, err := audioHTTPClient.Do(req)
	if err != nil {
		log.Printf("[AudioClient] TTS request failed provider=%s model=%s duration=%v err=%v", c.cfg.TextToSpeech.Provider, model.Model, time.Since(start), err)
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 32768))
		log.Printf("[AudioClient] TTS api error provider=%s model=%s duration=%v status=%d body=%q", c.cfg.TextToSpeech.Provider, model.Model, time.Since(start), resp.StatusCode, strings.TrimSpace(string(data)))
		return nil, fmt.Errorf("api error status=%d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	log.Printf("[AudioClient] TTS request done provider=%s model=%s duration=%v audio_bytes=%d", c.cfg.TextToSpeech.Provider, model.Model, time.Since(start), len(audioBytes))

	return map[string]any{
		"audio_base64": base64.StdEncoding.EncodeToString(audioBytes),
		"audio_format": format,
		"voice":        voice,
		"model":        model.Model,
		"transport":    "http",
	}, nil
}

func (c *AudioClient) synthesizeMiniMaxHTTP(ctx context.Context, provider *AudioProviderConfig, model *TextToSpeechModelConfig, params SynthesizeParams, voice string, format string) (map[string]any, error) {
	start := time.Now()
	body := map[string]any{
		"model":  model.Model,
		"text":   params.Text,
		"stream": false,
		"voice_setting": map[string]any{
			"voice_id": voice,
			"speed":    defaultFloat(model.Speed, 1),
			"vol":      defaultFloat(model.Volume, 1),
			"pitch":    model.Pitch,
		},
		"audio_setting": map[string]any{
			"sample_rate": defaultInt(model.SampleRate, 32000),
			"bitrate":     defaultInt(model.Bitrate, 128000),
			"format":      format,
			"channel":     defaultInt(model.Channel, 1),
		},
		"output_format":   "hex",
		"subtitle_enable": false,
	}
	if strings.TrimSpace(model.LanguageBoost) != "" {
		body["language_boost"] = strings.TrimSpace(model.LanguageBoost)
	}
	if model.EnglishNormalization != nil {
		body["voice_setting"].(map[string]any)["english_normalization"] = *model.EnglishNormalization
	}
	if len(model.PronunciationTone) > 0 {
		body["pronunciation_dict"] = map[string]any{
			"tone": model.PronunciationTone,
		}
	}

	bodyJSON, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodPost, joinURL(provider.BaseURL, provider.TextToSpeechPath), bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("Content-Type", "application/json")
	log.Printf("[AudioClient] MiniMax TTS request model=%s voice=%s format=%s text_len=%d endpoint=%s auth_len=%d", model.Model, voice, format, len(params.Text), joinURL(provider.BaseURL, provider.TextToSpeechPath), len(strings.TrimSpace(provider.APIKey)))

	var result struct {
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
		ExtraInfo map[string]any `json:"extra_info"`
		TraceID   string         `json:"trace_id"`
		BaseResp  struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := doJSON(req, &result); err != nil {
		log.Printf("[AudioClient] MiniMax TTS request failed model=%s duration=%v err=%v", model.Model, time.Since(start), err)
		return nil, err
	}
	if result.BaseResp.StatusCode != 0 {
		log.Printf("[AudioClient] MiniMax TTS api error model=%s duration=%v status_code=%d status_msg=%q trace_id=%s", model.Model, time.Since(start), result.BaseResp.StatusCode, strings.TrimSpace(result.BaseResp.StatusMsg), strings.TrimSpace(result.TraceID))
		return nil, fmt.Errorf("minimax status_code=%d: %s", result.BaseResp.StatusCode, strings.TrimSpace(result.BaseResp.StatusMsg))
	}
	if strings.TrimSpace(result.Data.Audio) == "" {
		log.Printf("[AudioClient] MiniMax TTS empty audio model=%s duration=%v trace_id=%s", model.Model, time.Since(start), strings.TrimSpace(result.TraceID))
		return nil, fmt.Errorf("minimax response missing audio data")
	}

	audioBytes, err := decodeHexString(strings.TrimSpace(result.Data.Audio))
	if err != nil {
		return nil, fmt.Errorf("decode minimax audio hex: %w", err)
	}
	log.Printf("[AudioClient] MiniMax TTS done model=%s duration=%v audio_bytes=%d trace_id=%s", model.Model, time.Since(start), len(audioBytes), strings.TrimSpace(result.TraceID))

	return map[string]any{
		"audio_base64": base64.StdEncoding.EncodeToString(audioBytes),
		"audio_format": format,
		"voice":        voice,
		"model":        model.Model,
		"transport":    "http",
		"trace_id":     result.TraceID,
		"extra_info":   result.ExtraInfo,
	}, nil
}

func (c *AudioClient) withTimeout(ctx context.Context) context.Context {
	timeout := time.Duration(c.cfg.RequestTimeoutSec) * time.Second
	if _, ok := ctx.Deadline(); ok {
		return ctx
	}
	newCtx, _ := context.WithTimeout(ctx, timeout)
	return newCtx
}

func doJSON(req *http.Request, out any) error {
	resp, err := audioHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("api error status=%d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

func joinURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

func defaultFloat(v, fallback float64) float64 {
	if v == 0 {
		return fallback
	}
	return v
}

func decodeHexString(input string) ([]byte, error) {
	if len(input)%2 != 0 {
		return nil, fmt.Errorf("invalid hex length")
	}
	dst := make([]byte, len(input)/2)
	for i := 0; i < len(dst); i++ {
		n, err := parseHexByte(input[i*2], input[i*2+1])
		if err != nil {
			return nil, err
		}
		dst[i] = n
	}
	return dst, nil
}

func parseHexByte(a, b byte) (byte, error) {
	hi, err := parseHexNibble(a)
	if err != nil {
		return 0, err
	}
	lo, err := parseHexNibble(b)
	if err != nil {
		return 0, err
	}
	return hi<<4 | lo, nil
}

func parseHexNibble(b byte) (byte, error) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', nil
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, nil
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, nil
	default:
		return 0, fmt.Errorf("invalid hex char: %q", b)
	}
}
