package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"uap"
)

func (s *WechatSink) AudioSent() bool { return s.audioSent }

func (s *WechatSink) trySendAudioReply(raw, fallbackText string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}

	var result struct {
		AudioBase64 string `json:"audio_base64"`
		AudioFormat string `json:"audio_format"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		log.Printf("[WechatSink] parse audio_reply failed: %v", err)
		return false
	}
	if strings.TrimSpace(result.AudioBase64) == "" {
		return false
	}

	audioFormat := strings.TrimSpace(result.AudioFormat)
	if audioFormat == "" {
		audioFormat = "mp3"
	}
	return s.sendVoiceNotify(result.AudioBase64, audioFormat, fallbackText)
}

func (s *WechatSink) trySendAudioReplyFromToolCalls(toolCalls []ToolCall) bool {
	for _, tc := range toolCalls {
		if !s.trySendAudioReplyFromToolCall(tc) {
			continue
		}
		return true
	}
	return false
}

func (s *WechatSink) trySendAudioReplyFromToolCall(tc ToolCall) bool {
	if s.bridge.resolveToolName(tc.Function.Name) != "TextToAudio" {
		return false
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		log.Printf("[WechatSink] parse legacy TextToAudio args failed: %v", err)
		return false
	}

	text := firstNonEmptyMapString(args, "text", "content", "input")
	if text == "" {
		log.Printf("[WechatSink] legacy TextToAudio args missing text")
		return false
	}

	payload := map[string]any{
		"text": text,
	}
	if voice := firstNonEmptyMapString(args, "voice", "voice_id"); voice != "" {
		payload["voice"] = voice
	}
	if audioFormat := firstNonEmptyMapString(args, "audio_format", "format"); audioFormat != "" {
		payload["audio_format"] = audioFormat
	}

	agentID, ok := s.bridge.getToolAgent("TextToAudio")
	if !ok {
		log.Printf("[WechatSink] TextToAudio tool not found for leaked legacy tool call")
		return false
	}

	rawArgs, _ := json.Marshal(payload)
	result, err := s.bridge.callRemoteAgent(context.Background(), "TextToAudio", agentID, rawArgs, nil)
	if err != nil {
		log.Printf("[WechatSink] execute leaked TextToAudio tool call failed: %v", err)
		return false
	}
	if result == nil || strings.TrimSpace(result.Result) == "" {
		return false
	}
	if !s.trySendAudioReply(result.Result, text) {
		return false
	}

	s.audioSent = true
	log.Printf("[WechatSink] leaked textual TextToAudio tool call recovered for to=%s", s.wechatUser)
	return true
}

func (s *WechatSink) trySendInlineAudioFromText(text string) bool {
	matches := inlineAudioTagPattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(matches) != 3 {
		return false
	}

	audioFormat := normalizeAudioFormat(matches[1])
	audioBase64 := strings.TrimSpace(matches[2])
	if audioBase64 == "" {
		return false
	}

	fallbackText := strings.TrimSpace(inlineAudioTagPattern.ReplaceAllString(text, ""))
	return s.sendVoiceNotify(audioBase64, audioFormat, fallbackText)
}

func (s *WechatSink) sendVoiceNotify(audioBase64, audioFormat, fallbackText string) bool {
	fallbackText = strings.TrimSpace(fallbackText)
	if fallbackText == "" {
		fallbackText = "[voice reply]"
	}

	if err := s.bridge.client.SendTo(s.fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel:     "wechat",
		To:          s.wechatUser,
		Content:     fallbackText,
		MessageType: "voice",
		Meta: map[string]any{
			"audio_base64": audioBase64,
			"audio_format": audioFormat,
			"input_mode":   "tts_reply",
		},
	}); err != nil {
		log.Printf("[WechatSink] send voice notify failed: %v", err)
		return false
	}

	log.Printf("[WechatSink] voice notify sent to=%s format=%s", s.wechatUser, audioFormat)
	return true
}
