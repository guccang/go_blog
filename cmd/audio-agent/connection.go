package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"agentbase"
	"uap"
)

type Connection struct {
	*agentbase.AgentBase
	cfg    *Config
	client *AudioClient
}

func NewConnection(cfg *Config, agentID string) *Connection {
	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "audio_agent",
		AgentName:   cfg.AgentName,
		Description: "Audio multimodal agent for speech-to-text and text-to-speech",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildAudioToolDefs(),
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
		client:    NewAudioClient(cfg),
	}
	c.RegisterToolCallHandler(c.handleToolCall)
	return c
}

func buildAudioToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "AudioToText",
			Description: "Convert audio content to text with a configured speech-to-text model",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"audio_base64": map[string]any{"type": "string", "description": "Base64 encoded audio bytes"},
					"audio_format": map[string]any{"type": "string", "description": "Optional file extension such as mp3, wav, m4a"},
					"file_name":    map[string]any{"type": "string", "description": "Optional file name for multipart upload"},
					"prompt":       map[string]any{"type": "string", "description": "Optional transcription prompt"},
					"language":     map[string]any{"type": "string", "description": "Optional language hint"},
				},
				"required": []string{"audio_base64"},
			}),
		},
		{
			Name:        "TextToAudio",
			Description: "Convert text to speech with a configured text-to-speech model",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text":         map[string]any{"type": "string", "description": "Input text"},
					"voice":        map[string]any{"type": "string", "description": "Optional voice override"},
					"audio_format": map[string]any{"type": "string", "description": "Optional output format such as mp3 or wav"},
				},
				"required": []string{"text"},
			}),
		},
	}
}

func (c *Connection) handleToolCall(msg *uap.Message) {
	start := time.Now()
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[AudioAgent] invalid tool_call payload from=%s msgID=%s err=%v", msg.From, msg.ID, err)
		c.sendToolError(msg.From, msg.ID, "invalid tool_call payload")
		return
	}

	var args map[string]any
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[AudioAgent] invalid tool_call args from=%s msgID=%s tool=%s err=%v", msg.From, msg.ID, payload.ToolName, err)
			c.sendToolError(msg.From, msg.ID, "invalid arguments: "+err.Error())
			return
		}
	} else {
		args = make(map[string]any)
	}

	log.Printf("[AudioAgent] tool_call received from=%s msgID=%s tool=%s args=%s",
		msg.From, msg.ID, payload.ToolName, summarizeToolArgs(payload.ToolName, args))

	var (
		result map[string]any
		err    error
	)

	switch payload.ToolName {
	case "AudioToText":
		result, err = c.toolAudioToText(args)
	case "TextToAudio":
		result, err = c.toolTextToAudio(args)
	default:
		log.Printf("[AudioAgent] unknown tool from=%s msgID=%s tool=%s", msg.From, msg.ID, payload.ToolName)
		c.sendToolError(msg.From, msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName))
		return
	}

	if err != nil {
		log.Printf("[AudioAgent] tool_call failed from=%s msgID=%s tool=%s duration=%v err=%v", msg.From, msg.ID, payload.ToolName, time.Since(start), err)
		c.sendToolError(msg.From, msg.ID, err.Error())
		return
	}
	log.Printf("[AudioAgent] tool_call succeeded from=%s msgID=%s tool=%s duration=%v result=%s", msg.From, msg.ID, payload.ToolName, time.Since(start), summarizeToolResult(payload.ToolName, result))
	c.sendToolResult(msg.From, msg.ID, result)
}

func (c *Connection) toolAudioToText(args map[string]any) (map[string]any, error) {
	audioBase64, _ := args["audio_base64"].(string)
	if audioBase64 == "" {
		return nil, fmt.Errorf("audio_base64 is required")
	}
	audioFormat, _ := args["audio_format"].(string)
	fileName, _ := args["file_name"].(string)
	prompt, _ := args["prompt"].(string)
	language, _ := args["language"].(string)

	result, err := c.client.Transcribe(context.Background(), TranscribeParams{
		AudioBase64: audioBase64,
		FileName:    fileName,
		Format:      audioFormat,
		Prompt:      prompt,
		Language:    language,
	})
	if err != nil {
		return nil, err
	}
	if _, exists := result["provider"]; !exists {
		result["provider"] = c.cfg.SpeechToText.Provider
	}
	return result, nil
}

func (c *Connection) toolTextToAudio(args map[string]any) (map[string]any, error) {
	text, _ := args["text"].(string)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	voice, _ := args["voice"].(string)
	audioFormat, _ := args["audio_format"].(string)

	result, err := c.client.Synthesize(context.Background(), SynthesizeParams{
		Text:   text,
		Voice:  voice,
		Format: audioFormat,
	})
	if err != nil {
		return nil, err
	}
	result["provider"] = c.cfg.TextToSpeech.Provider
	return result, nil
}

func (c *Connection) sendToolResult(target, requestID string, result map[string]any) {
	data, _ := json.Marshal(result)
	if err := c.Client.SendTo(target, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: requestID,
		Success:   true,
		Result:    string(data),
	}); err != nil {
		log.Printf("[AudioAgent] send tool_result failed: %v", err)
	}
}

func (c *Connection) sendToolError(target, requestID, message string) {
	if err := c.Client.SendTo(target, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: requestID,
		Success:   false,
		Error:     message,
	}); err != nil {
		log.Printf("[AudioAgent] send tool_error failed: %v", err)
	}
}

func summarizeToolArgs(toolName string, args map[string]any) string {
	switch toolName {
	case "AudioToText":
		audioBase64, _ := args["audio_base64"].(string)
		audioFormat, _ := args["audio_format"].(string)
		fileName, _ := args["file_name"].(string)
		return fmt.Sprintf("audio_bytes≈%d format=%s file=%s", base64DecodedLen(audioBase64), strings.TrimSpace(audioFormat), strings.TrimSpace(fileName))
	case "TextToAudio":
		text, _ := args["text"].(string)
		voice, _ := args["voice"].(string)
		audioFormat, _ := args["audio_format"].(string)
		return fmt.Sprintf("text_len=%d voice=%s format=%s preview=%q", len(text), strings.TrimSpace(voice), strings.TrimSpace(audioFormat), truncateForLog(text, 48))
	default:
		raw, _ := json.Marshal(args)
		return string(raw)
	}
}

func summarizeToolResult(toolName string, result map[string]any) string {
	switch toolName {
	case "AudioToText":
		text := firstStringField(result, "text", "transcript", "content")
		provider := firstStringField(result, "provider")
		return fmt.Sprintf("provider=%s text_len=%d preview=%q", provider, len(text), truncateForLog(text, 48))
	case "TextToAudio":
		audioBase64, _ := result["audio_base64"].(string)
		audioFormat, _ := result["audio_format"].(string)
		provider := firstStringField(result, "provider")
		traceID := firstStringField(result, "trace_id")
		return fmt.Sprintf("provider=%s format=%s audio_bytes≈%d trace_id=%s", provider, strings.TrimSpace(audioFormat), base64DecodedLen(audioBase64), strings.TrimSpace(traceID))
	default:
		raw, _ := json.Marshal(result)
		return string(raw)
	}
}

func firstStringField(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, _ := data[key].(string); strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncateForLog(text string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func base64DecodedLen(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return len(s) * 3 / 4
}
