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
		Tools:       buildImageToolDefs(cfg),
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
		client:    NewImageClient(cfg),
	}
	c.RegisterToolCallHandler(c.handleToolCall)
	return c
}

func buildImageToolDefs(cfg *Config) []uap.ToolDef {
	tools := make([]uap.ToolDef, 0, 3)
	if cfg == nil || cfg.HasVisionTool() {
		tools = append(tools, uap.ToolDef{
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
		})
	}
	tools = append(tools,
		uap.ToolDef{
			Name:        "TextToImage",
			Description: "Generate image(s) from text. Return image_base64 and app_message metadata; send app_message via app.SendRichMessage to display in Flutter and let app-agent persist it through obs-agent.",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt":           map[string]any{"type": "string", "description": "Image generation prompt"},
					"size":             map[string]any{"type": "string", "description": "Optional OpenAI-compatible size such as 1024x1024"},
					"aspect_ratio":     map[string]any{"type": "string", "description": "Optional MiniMax aspect ratio such as 1:1, 16:9, 9:16"},
					"response_format":  map[string]any{"type": "string", "description": "Optional response format: base64 or url. Use base64 when returning to Flutter."},
					"width":            map[string]any{"type": "integer", "description": "Optional MiniMax width, must be set together with height"},
					"height":           map[string]any{"type": "integer", "description": "Optional MiniMax height, must be set together with width"},
					"n":                map[string]any{"type": "integer", "description": "Optional image count. MiniMax supports 1 to 9."},
					"seed":             map[string]any{"type": "integer", "description": "Optional deterministic seed"},
					"prompt_optimizer": map[string]any{"type": "boolean", "description": "Optional MiniMax prompt optimizer switch"},
					"aigc_watermark":   map[string]any{"type": "boolean", "description": "Optional MiniMax AIGC watermark switch"},
				},
				"required": []string{"prompt"},
			}),
		},
		uap.ToolDef{
			Name:        "ImageToImage",
			Description: "Generate image(s) from text plus MiniMax subject reference image(s). Return image_base64 and app_message metadata; send app_message via app.SendRichMessage to display in Flutter and let app-agent persist it through obs-agent.",
			Parameters: agentbase.MustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{"type": "string", "description": "Image generation prompt"},
					"subject_reference": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"type":       map[string]any{"type": "string", "description": "Reference type, for example character"},
								"image_file": map[string]any{"type": "string", "description": "Reference image URL accepted by MiniMax"},
							},
							"required": []string{"image_file"},
						},
					},
					"reference_image_url": map[string]any{"type": "string", "description": "Convenience single reference image URL"},
					"reference_type":      map[string]any{"type": "string", "description": "Convenience single reference type, defaults to character"},
					"aspect_ratio":        map[string]any{"type": "string", "description": "Optional MiniMax aspect ratio such as 1:1, 16:9, 9:16"},
					"response_format":     map[string]any{"type": "string", "description": "Optional response format: base64 or url. Use base64 when returning to Flutter."},
					"width":               map[string]any{"type": "integer", "description": "Optional MiniMax width, must be set together with height"},
					"height":              map[string]any{"type": "integer", "description": "Optional MiniMax height, must be set together with width"},
					"n":                   map[string]any{"type": "integer", "description": "Optional image count. MiniMax supports 1 to 9."},
					"seed":                map[string]any{"type": "integer", "description": "Optional deterministic seed"},
					"prompt_optimizer":    map[string]any{"type": "boolean", "description": "Optional MiniMax prompt optimizer switch"},
					"aigc_watermark":      map[string]any{"type": "boolean", "description": "Optional MiniMax AIGC watermark switch"},
				},
				"required": []string{"prompt"},
			}),
		},
	)
	return tools
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
	case "ImageToImage":
		result, err = c.toolImageToImage(args)
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
	params := generationParamsFromArgs(args)
	params.Prompt = prompt
	return c.client.Generate(context.Background(), params)
}

func (c *Connection) toolImageToImage(args map[string]any) (map[string]any, error) {
	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	params := generationParamsFromArgs(args)
	params.Prompt = prompt
	params.SubjectReference = parseSubjectReferences(args)
	if len(params.SubjectReference) == 0 {
		return nil, fmt.Errorf("subject_reference or reference_image_url is required")
	}
	return c.client.Generate(context.Background(), params)
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

func generationParamsFromArgs(args map[string]any) GenerateImageParams {
	return GenerateImageParams{
		Size:            stringArg(args, "size"),
		AspectRatio:     stringArg(args, "aspect_ratio"),
		ResponseFormat:  stringArg(args, "response_format"),
		Width:           intArg(args, "width"),
		Height:          intArg(args, "height"),
		N:               intArg(args, "n"),
		Seed:            int64PtrArg(args, "seed"),
		PromptOptimizer: boolPtrArg(args, "prompt_optimizer"),
		AIGCWatermark:   boolPtrArg(args, "aigc_watermark"),
	}
}

func parseSubjectReferences(args map[string]any) []SubjectReference {
	var refs []SubjectReference
	if raw, ok := args["subject_reference"].([]any); ok {
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			imageFile := stringArg(m, "image_file")
			if imageFile == "" {
				continue
			}
			refType := stringArg(m, "type")
			if refType == "" {
				refType = "character"
			}
			refs = append(refs, SubjectReference{Type: refType, ImageFile: imageFile})
		}
	}
	if imageURL := stringArg(args, "reference_image_url"); imageURL != "" {
		refType := stringArg(args, "reference_type")
		if refType == "" {
			refType = "character"
		}
		refs = append(refs, SubjectReference{Type: refType, ImageFile: imageURL})
	}
	return refs
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, _ := args[key].(string)
	return v
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func int64PtrArg(args map[string]any, key string) *int64 {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case int:
		n := int64(v)
		return &n
	case int64:
		n := v
		return &n
	case float64:
		n := int64(v)
		return &n
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return &n
		}
	}
	return nil
}

func boolPtrArg(args map[string]any, key string) *bool {
	if args == nil {
		return nil
	}
	v, ok := args[key].(bool)
	if !ok {
		return nil
	}
	return &v
}
