package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var imageHTTPClient = &http.Client{}

type ImageClient struct {
	cfg *Config
}

func NewImageClient(cfg *Config) *ImageClient {
	return &ImageClient{cfg: cfg}
}

type DescribeImageParams struct {
	ImageBase64 string
	MimeType    string
	Prompt      string
}

type GenerateImageParams struct {
	Prompt string
	Size   string
}

func (c *ImageClient) Describe(ctx context.Context, params DescribeImageParams) (map[string]any, error) {
	provider, model, err := c.cfg.ResolveVision()
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(params.Prompt)
	if prompt == "" {
		prompt = "Please extract the visible text and then describe the image in detail."
	}
	mimeType := strings.TrimSpace(params.MimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}

	body := map[string]any{
		"model": model.Model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]any{
						"url": "data:" + mimeType + ";base64," + strings.TrimSpace(params.ImageBase64),
					}},
				},
			},
		},
	}
	if model.MaxTokens > 0 {
		body["max_tokens"] = model.MaxTokens
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodPost, joinURL(provider.BaseURL, provider.VisionPath), bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("Content-Type", "application/json")

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := doImageJSON(req, &resp); err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty vision response")
	}
	return map[string]any{
		"text":     resp.Choices[0].Message.Content,
		"model":    model.Model,
		"provider": c.cfg.ImageToText.Provider,
	}, nil
}

func (c *ImageClient) Generate(ctx context.Context, params GenerateImageParams) (map[string]any, error) {
	provider, model, err := c.cfg.ResolveGeneration()
	if err != nil {
		return nil, err
	}
	size := strings.TrimSpace(params.Size)
	if size == "" {
		size = defaultString(model.Size, "1024x1024")
	}

	body := map[string]any{
		"model":           model.Model,
		"prompt":          params.Prompt,
		"size":            size,
		"response_format": "b64_json",
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodPost, joinURL(provider.BaseURL, provider.ImageGenerationPath), bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("Content-Type", "application/json")

	var resp struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := doImageJSON(req, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("empty image generation response")
	}
	result := map[string]any{
		"model":    model.Model,
		"provider": c.cfg.TextToImage.Provider,
		"size":     size,
	}
	if strings.TrimSpace(resp.Data[0].B64JSON) != "" {
		result["image_base64"] = resp.Data[0].B64JSON
	}
	if strings.TrimSpace(resp.Data[0].URL) != "" {
		result["image_url"] = resp.Data[0].URL
	}
	return result, nil
}

func (c *ImageClient) withTimeout(ctx context.Context) context.Context {
	timeout := time.Duration(c.cfg.RequestTimeoutSec) * time.Second
	if _, ok := ctx.Deadline(); ok {
		return ctx
	}
	newCtx, _ := context.WithTimeout(ctx, timeout)
	return newCtx
}

func doImageJSON(req *http.Request, out any) error {
	resp, err := imageHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
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
