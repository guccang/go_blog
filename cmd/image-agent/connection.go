package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"agentbase"
	"uap"
)

type Connection struct {
	*agentbase.AgentBase
	cfg    *Config
	client *ImageClient
}

func NewConnection(cfg *Config, agentID string) *Connection {
	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "image_agent",
		AgentName:   cfg.AgentName,
		Description: "Image multimodal agent for image-to-text and text-to-image",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildImageToolDefs(),
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
		client:    NewImageClient(cfg),
	}
	c.RegisterToolCallHandler(c.handleToolCall)
	return c
}

func buildImageToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "ImageToText",
			Description: "Convert image content to text with a configured vision model",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"image_base64": map[string]any{"type": "string", "description": "Base64 encoded image bytes"},
					"mime_type":    map[string]any{"type": "string", "description": "Optional mime type such as image/png"},
					"prompt":       map[string]any{"type": "string", "description": "Optional OCR or vision instruction"},
				},
				"required": []string{"image_base64"},
			}),
		},
		{
			Name:        "TextToImage",
			Description: "Generate an image from text with a configured image generation model",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{"type": "string", "description": "Image generation prompt"},
					"size":   map[string]any{"type": "string", "description": "Optional size such as 1024x1024"},
				},
				"required": []string{"prompt"},
			}),
		},
	}
}

func (c *Connection) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendToolError(msg.From, msg.ID, "invalid tool_call payload")
		return
	}

	var args map[string]any
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			c.sendToolError(msg.From, msg.ID, "invalid arguments: "+err.Error())
			return
		}
	} else {
		args = make(map[string]any)
	}

	var (
		result map[string]any
		err    error
	)

	switch payload.ToolName {
	case "ImageToText":
		result, err = c.toolImageToText(args)
	case "TextToImage":
		result, err = c.toolTextToImage(args)
	default:
		c.sendToolError(msg.From, msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName))
		return
	}

	if err != nil {
		c.sendToolError(msg.From, msg.ID, err.Error())
		return
	}
	c.sendToolResult(msg.From, msg.ID, result)
}

func (c *Connection) toolImageToText(args map[string]any) (map[string]any, error) {
	imageBase64, _ := args["image_base64"].(string)
	if imageBase64 == "" {
		return nil, fmt.Errorf("image_base64 is required")
	}
	mimeType, _ := args["mime_type"].(string)
	prompt, _ := args["prompt"].(string)
	return c.client.Describe(context.Background(), DescribeImageParams{
		ImageBase64: imageBase64,
		MimeType:    mimeType,
		Prompt:      prompt,
	})
}

func (c *Connection) toolTextToImage(args map[string]any) (map[string]any, error) {
	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	size, _ := args["size"].(string)
	return c.client.Generate(context.Background(), GenerateImageParams{
		Prompt: prompt,
		Size:   size,
	})
}

func (c *Connection) sendToolResult(target, requestID string, result map[string]any) {
	data, _ := json.Marshal(result)
	if err := c.Client.SendTo(target, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: requestID,
		Success:   true,
		Result:    string(data),
	}); err != nil {
		log.Printf("[ImageAgent] send tool_result failed: %v", err)
	}
}

func (c *Connection) sendToolError(target, requestID, message string) {
	if err := c.Client.SendTo(target, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: requestID,
		Success:   false,
		Error:     message,
	}); err != nil {
		log.Printf("[ImageAgent] send tool_error failed: %v", err)
	}
}
