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
	client *VideoClient
}

func NewConnection(cfg *Config, agentID string) *Connection {
	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "video_agent",
		AgentName:   cfg.AgentName,
		Description: "Video generation agent for MiniMax Hailuo text-to-video and image-to-video",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildVideoToolDefs(),
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
		client:    NewVideoClient(cfg),
	}
	c.RegisterToolCallHandler(c.handleToolCall)
	return c
}

func buildVideoToolDefs() []uap.ToolDef {
	commonProperties := map[string]any{
		"prompt":            map[string]any{"type": "string", "description": "Video prompt, max 2000 characters. Hailuo camera directives like [推进] are supported."},
		"model_alias":       map[string]any{"type": "string", "description": "Optional configured model alias, for example default or fast"},
		"duration":          map[string]any{"type": "integer", "description": "Optional duration in seconds, usually 6 or 10"},
		"resolution":        map[string]any{"type": "string", "description": "Optional resolution: 768P or 1080P"},
		"prompt_optimizer":  map[string]any{"type": "boolean", "description": "Optional MiniMax prompt optimization switch"},
		"fast_pretreatment": map[string]any{"type": "boolean", "description": "Optional faster prompt pretreatment switch"},
		"aigc_watermark":    map[string]any{"type": "boolean", "description": "Optional generated-video watermark switch"},
	}
	imageProperties := cloneSchemaProperties(commonProperties)
	imageProperties["first_frame_image"] = map[string]any{"type": "string", "description": "First frame image URL or MiniMax-supported image string"}

	return []uap.ToolDef{
		{
			Name:        "TextToVideo",
			Description: "Generate an MP4 video from text using MiniMax Hailuo. Returns video_base64 and app_message for app.SendRichMessage with message_type=video.",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type":       "object",
				"properties": commonProperties,
				"required":   []string{"prompt"},
			}),
		},
		{
			Name:        "ImageToVideo",
			Description: "Generate an MP4 video from a first-frame image and prompt using MiniMax Hailuo. Returns video_base64 and app_message for app.SendRichMessage with message_type=video.",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type":       "object",
				"properties": imageProperties,
				"required":   []string{"prompt", "first_frame_image"},
			}),
		},
	}
}

func (c *Connection) handleToolCall(msg *uap.Message) {
	start := time.Now()
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[VideoAgent] invalid tool_call payload from=%s msgID=%s err=%v", msg.From, msg.ID, err)
		c.sendToolError(msg.From, msg.ID, "invalid tool_call payload")
		return
	}

	args := make(map[string]any)
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[VideoAgent] invalid tool_call args from=%s msgID=%s tool=%s err=%v", msg.From, msg.ID, payload.ToolName, err)
			c.sendToolError(msg.From, msg.ID, "invalid arguments: "+err.Error())
			return
		}
	}

	log.Printf("[VideoAgent] tool_call received from=%s msgID=%s tool=%s args=%s", msg.From, msg.ID, payload.ToolName, summarizeToolArgs(payload.ToolName, args))

	var (
		result map[string]any
		err    error
	)
	switch payload.ToolName {
	case "TextToVideo":
		result, err = c.toolGenerateVideo(args, false)
	case "ImageToVideo":
		result, err = c.toolGenerateVideo(args, true)
	default:
		c.sendToolError(msg.From, msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName))
		return
	}
	if err != nil {
		log.Printf("[VideoAgent] tool_call failed from=%s msgID=%s tool=%s duration=%v err=%v", msg.From, msg.ID, payload.ToolName, time.Since(start), err)
		c.sendToolError(msg.From, msg.ID, err.Error())
		return
	}
	log.Printf("[VideoAgent] tool_call succeeded from=%s msgID=%s tool=%s duration=%v result=%s", msg.From, msg.ID, payload.ToolName, time.Since(start), summarizeToolResult(result))
	c.sendToolResult(msg.From, msg.ID, result)
}

func (c *Connection) toolGenerateVideo(args map[string]any, requireImage bool) (map[string]any, error) {
	prompt := strings.TrimSpace(stringArg(args, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	firstFrame := strings.TrimSpace(stringArg(args, "first_frame_image"))
	if requireImage && firstFrame == "" {
		return nil, fmt.Errorf("first_frame_image is required")
	}
	return c.client.Generate(context.Background(), GenerateVideoParams{
		Prompt:           prompt,
		FirstFrameImage:  firstFrame,
		ModelAlias:       stringArg(args, "model_alias"),
		Duration:         intArg(args, "duration"),
		Resolution:       stringArg(args, "resolution"),
		PromptOptimizer:  boolArg(args, "prompt_optimizer"),
		FastPretreatment: boolArg(args, "fast_pretreatment"),
		AIGCWatermark:    boolArg(args, "aigc_watermark"),
	})
}

func (c *Connection) sendToolResult(target, requestID string, result map[string]any) {
	data, _ := json.Marshal(result)
	if err := c.Client.SendTo(target, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: requestID,
		Success:   true,
		Result:    string(data),
	}); err != nil {
		log.Printf("[VideoAgent] send tool_result failed: %v", err)
	}
}

func (c *Connection) sendToolError(target, requestID, message string) {
	if err := c.Client.SendTo(target, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: requestID,
		Success:   false,
		Error:     message,
	}); err != nil {
		log.Printf("[VideoAgent] send tool_error failed: %v", err)
	}
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func intArg(args map[string]any, key string) int {
	switch value := args[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		i, _ := value.Int64()
		return int(i)
	default:
		return 0
	}
}

func boolArg(args map[string]any, key string) *bool {
	value, exists := args[key]
	if !exists {
		return nil
	}
	switch v := value.(type) {
	case bool:
		return &v
	default:
		return nil
	}
}

func summarizeToolArgs(toolName string, args map[string]any) string {
	prompt := strings.TrimSpace(stringArg(args, "prompt"))
	modelAlias := strings.TrimSpace(stringArg(args, "model_alias"))
	resolution := strings.TrimSpace(stringArg(args, "resolution"))
	image := strings.TrimSpace(stringArg(args, "first_frame_image"))
	return fmt.Sprintf("tool=%s prompt_len=%d model_alias=%s resolution=%s has_image=%v preview=%q", toolName, len(prompt), modelAlias, resolution, image != "", truncateForLog(prompt, 64))
}

func summarizeToolResult(result map[string]any) string {
	videoBase64, _ := result["video_base64"].(string)
	return fmt.Sprintf("model=%s task_id=%s video_bytes≈%d", firstStringField(result, "model"), firstStringField(result, "task_id"), base64DecodedLen(videoBase64))
}

func cloneSchemaProperties(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
