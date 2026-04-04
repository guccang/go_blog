package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

const appMessageJSONPrefix = "APP_MESSAGE_JSON:"

type appInboundMessage struct {
	Kind        string                `json:"kind"`
	UserID      string                `json:"user_id"`
	MessageType string                `json:"message_type"`
	Content     string                `json:"content"`
	Scope       string                `json:"scope"`
	GroupID     string                `json:"group_id,omitempty"`
	Attachment  *appInboundAttachment `json:"attachment,omitempty"`
	Meta        map[string]any        `json:"meta,omitempty"`
}

type appInboundAttachment struct {
	MessageType string         `json:"message_type"`
	FileID      string         `json:"file_id,omitempty"`
	FileName    string         `json:"file_name,omitempty"`
	FilePath    string         `json:"file_path,omitempty"`
	FileSize    int            `json:"file_size,omitempty"`
	Format      string         `json:"format,omitempty"`
	MIMEType    string         `json:"mime_type,omitempty"`
	DurationMS  int            `json:"duration_ms,omitempty"`
	SpeechText  string         `json:"speech_text,omitempty"`
	InputMode   string         `json:"input_mode,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

func parseAppInboundMessage(content string) (*appInboundMessage, bool) {
	raw := strings.TrimSpace(content)
	if !strings.HasPrefix(raw, appMessageJSONPrefix) {
		return nil, false
	}
	body := strings.TrimSpace(strings.TrimPrefix(raw, appMessageJSONPrefix))
	if body == "" {
		return nil, false
	}

	var msg appInboundMessage
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		log.Printf("[AppPreprocess] parse app message failed: %v", err)
		return nil, false
	}
	return &msg, true
}

func (b *Bridge) preprocessAppMessage(ctx context.Context, fromAgent, appUser, content string) string {
	msg, ok := parseAppInboundMessage(content)
	if !ok || msg == nil {
		return content
	}

	normalized := strings.TrimSpace(msg.Content)
	messageType := strings.TrimSpace(strings.ToLower(msg.MessageType))
	if messageType == "" && msg.Attachment != nil {
		messageType = strings.TrimSpace(strings.ToLower(msg.Attachment.MessageType))
	}

	var sections []string
	if normalized != "" {
		sections = append(sections, normalized)
	}
	if msg.Scope == "group" && strings.TrimSpace(msg.GroupID) != "" {
		sections = append(sections, fmt.Sprintf("[群聊消息] group_id=%s", msg.GroupID))
	}

	if msg.Attachment != nil {
		attachmentText := b.preprocessAppAttachment(ctx, fromAgent, appUser, messageType, msg.Attachment)
		if strings.TrimSpace(attachmentText) != "" {
			sections = append(sections, attachmentText)
		}
	}

	if len(sections) == 0 {
		return content
	}
	return strings.Join(sections, "\n\n")
}

func (b *Bridge) preprocessAppAttachment(ctx context.Context, fromAgent, appUser, messageType string, attachment *appInboundAttachment) string {
	if attachment == nil {
		return ""
	}

	switch messageType {
	case "audio":
		if fromAgent != "" {
			b.sendApp(fromAgent, appUser, "收到语音，正在转写...")
		}
		text, source := b.transcribeAppAudio(ctx, attachment)
		if text == "" {
			return "用户发送了一段语音，但当前无法获得转写结果。"
		}
		return fmt.Sprintf("用户发送的是语音消息。转写结果(%s):\n%s", source, text)
	case "image":
		if fromAgent != "" {
			b.sendApp(fromAgent, appUser, "收到图片，正在识别...")
		}
		text, source := b.describeAppImage(ctx, attachment)
		if text == "" {
			return "用户发送了一张图片，但当前无法获得识别结果。"
		}
		return fmt.Sprintf("用户发送的是图片消息。识别结果(%s):\n%s", source, text)
	default:
		fileName := strings.TrimSpace(attachment.FileName)
		if fileName == "" {
			fileName = "unknown"
		}
		return fmt.Sprintf("用户发送了一个%s附件：%s", defaultNonEmpty(messageType, "file"), fileName)
	}
}

func (b *Bridge) transcribeAppAudio(ctx context.Context, attachment *appInboundAttachment) (string, string) {
	if attachment == nil {
		return "", ""
	}
	fallback := strings.TrimSpace(attachment.SpeechText)
	data, err := os.ReadFile(strings.TrimSpace(attachment.FilePath))
	if err != nil || len(data) == 0 {
		if fallback != "" {
			return fallback, "app"
		}
		if err != nil {
			log.Printf("[AppPreprocess] read audio attachment failed: %v", err)
		}
		return "", ""
	}

	agentID, ok := b.getToolAgent("AudioToText")
	if !ok {
		if fallback != "" {
			return fallback, "app"
		}
		log.Printf("[AppPreprocess] AudioToText tool not found")
		return "", ""
	}

	args, _ := json.Marshal(map[string]any{
		"audio_base64": base64.StdEncoding.EncodeToString(data),
		"audio_format": strings.TrimSpace(attachment.Format),
		"file_name":    strings.TrimSpace(attachment.FileName),
	})
	result, err := b.callRemoteAgent(ctx, "AudioToText", agentID, args, nil)
	if err != nil {
		log.Printf("[AppPreprocess] AudioToText failed: %v", err)
		if fallback != "" {
			return fallback, "app"
		}
		return "", ""
	}

	text := extractTextField(result.Result, "text", "transcript", "content")
	if text == "" && fallback != "" {
		return fallback, "app"
	}
	return text, "audio-agent"
}

func (b *Bridge) describeAppImage(ctx context.Context, attachment *appInboundAttachment) (string, string) {
	if attachment == nil {
		return "", ""
	}
	data, err := os.ReadFile(strings.TrimSpace(attachment.FilePath))
	if err != nil || len(data) == 0 {
		if err != nil {
			log.Printf("[AppPreprocess] read image attachment failed: %v", err)
		}
		return "", ""
	}

	agentID, ok := b.getToolAgent("ImageToText")
	if !ok {
		log.Printf("[AppPreprocess] ImageToText tool not found")
		return "", ""
	}

	mimeType := strings.TrimSpace(attachment.MIMEType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	args, _ := json.Marshal(map[string]any{
		"image_base64": base64.StdEncoding.EncodeToString(data),
		"mime_type":    mimeType,
	})
	result, err := b.callRemoteAgent(ctx, "ImageToText", agentID, args, nil)
	if err != nil {
		log.Printf("[AppPreprocess] ImageToText failed: %v", err)
		return "", ""
	}

	return extractTextField(result.Result, "text", "content", "result"), "image-agent"
}

func extractTextField(raw string, keys ...string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return raw
	}
	for _, key := range keys {
		if v, ok := data[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func defaultNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
