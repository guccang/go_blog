package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SystemPromptContext 保存稳定提示词快照，避免把动态上下文和 system prompt 混在一起。
type SystemPromptContext struct {
	Account       string          `json:"account,omitempty"`
	Source        string          `json:"source,omitempty"`
	SystemPrompt  string          `json:"system_prompt"`
	UserContext   string          `json:"user_context,omitempty"`
	SystemContext string          `json:"system_context,omitempty"`
	Sections      []PromptSection `json:"sections,omitempty"`
}

// PromptContext 保留旧名称，兼容现有调用。
type PromptContext = SystemPromptContext

func (b *Bridge) buildAssistantPromptContext(account string) SystemPromptContext {
	prompt, sections := b.buildAssistantSystemPrompt(account)
	return SystemPromptContext{
		Account:      strings.TrimSpace(account),
		SystemPrompt: prompt,
		Sections:     clonePromptSections(sections),
	}
}

func clonePromptSections(sections []PromptSection) []PromptSection {
	cloned := make([]PromptSection, len(sections))
	copy(cloned, sections)
	return cloned
}

type AttachmentKind string

// RuntimeAttachmentKind 保留旧名称，兼容旧调用方。
type RuntimeAttachmentKind = AttachmentKind

const (
	AttachmentKindDependencyResult AttachmentKind = "dependency_result"
	AttachmentKindTaskNotification AttachmentKind = "task_notification"
	AttachmentKindSteer            AttachmentKind = "steer"
	AttachmentKindResume           AttachmentKind = "resume_instruction"
	AttachmentKindSystemHint       AttachmentKind = "system_hint"
)

const (
	RuntimeAttachmentDependencyResult = AttachmentKindDependencyResult
	RuntimeAttachmentTaskNotification = AttachmentKindTaskNotification
	RuntimeAttachmentSteer            = AttachmentKindSteer
	RuntimeAttachmentResume           = AttachmentKindResume
	RuntimeAttachmentSystemHint       = AttachmentKindSystemHint
)

// Attachment 对齐 Claude Code 的 queued attachment 思路：
// 动态上下文不回写 system prompt，而是按轮次注入消息流。
type Attachment struct {
	ID              string            `json:"id,omitempty"`
	Kind            AttachmentKind    `json:"kind"`
	Title           string            `json:"title,omitempty"`
	Content         string            `json:"content"`
	SourceSessionID string            `json:"source_session_id,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// RuntimeAttachment 保留旧名称，兼容现有调用。
type RuntimeAttachment = Attachment

func newAttachment(kind AttachmentKind, title, content, sourceSessionID string, meta map[string]string) Attachment {
	return Attachment{
		ID:              newSessionID(),
		Kind:            kind,
		Title:           strings.TrimSpace(title),
		Content:         strings.TrimSpace(content),
		SourceSessionID: strings.TrimSpace(sourceSessionID),
		Meta:            cloneStringMap(meta),
		CreatedAt:       time.Now(),
	}
}

func newRuntimeAttachment(kind AttachmentKind, title, content, sourceSessionID string, meta map[string]string) Attachment {
	return newAttachment(kind, title, content, sourceSessionID, meta)
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func formatAttachment(att Attachment) string {
	var sb strings.Builder
	kind := strings.TrimSpace(string(att.Kind))
	if kind == "" {
		kind = "runtime_context"
	}

	sb.WriteString(fmt.Sprintf("[运行时上下文/%s]", kind))
	if title := strings.TrimSpace(att.Title); title != "" {
		sb.WriteString(" ")
		sb.WriteString(title)
	}
	sb.WriteString("\n")

	if len(att.Meta) > 0 {
		keys := make([]string, 0, len(att.Meta))
		for k := range att.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, att.Meta[k]))
		}
	}

	content := strings.TrimSpace(att.Content)
	if content != "" {
		sb.WriteString(content)
	}
	return strings.TrimSpace(sb.String())
}

func formatRuntimeAttachment(att Attachment) string {
	return formatAttachment(att)
}

func createAttachmentMessage(att Attachment) Message {
	return Message{
		Role:    "user",
		Content: formatAttachment(att),
	}
}

func (att Attachment) ToMessage() Message {
	return createAttachmentMessage(att)
}

func cloneAttachments(attachments []Attachment) []Attachment {
	cloned := make([]Attachment, len(attachments))
	for i, att := range attachments {
		cloned[i] = Attachment{
			ID:              att.ID,
			Kind:            att.Kind,
			Title:           att.Title,
			Content:         att.Content,
			SourceSessionID: att.SourceSessionID,
			Meta:            cloneStringMap(att.Meta),
			CreatedAt:       att.CreatedAt,
		}
	}
	return cloned
}

func attachmentsFromMailbox(messages []MailboxEntry) []Attachment {
	if len(messages) == 0 {
		return nil
	}
	attachments := make([]Attachment, 0, len(messages))
	for _, msg := range messages {
		attachments = append(attachments, Attachment{
			ID:              msg.ID,
			Kind:            AttachmentKind(strings.TrimSpace(msg.Kind)),
			Title:           strings.TrimSpace(msg.Title),
			Content:         strings.TrimSpace(msg.Content),
			SourceSessionID: strings.TrimSpace(msg.SourceSessionID),
			Meta:            cloneStringMap(msg.Meta),
			CreatedAt:       msg.CreatedAt,
		})
	}
	return attachments
}

// RuntimeSnapshot 保存一个 session 当前轮的可恢复状态。
type RuntimeSnapshot struct {
	RootID         string                   `json:"root_id"`
	SessionID      string                   `json:"session_id"`
	Query          string                   `json:"query,omitempty"`
	Status         string                   `json:"status"`
	PromptContext  SystemPromptContext      `json:"prompt_context,omitempty"`
	Attachments    []Attachment             `json:"attachments,omitempty"`
	CompactHistory []RuntimeCompactMetadata `json:"compact_history,omitempty"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

// RuntimeStateSnapshot 保留旧名称，兼容现有调用。
type RuntimeStateSnapshot = RuntimeSnapshot
