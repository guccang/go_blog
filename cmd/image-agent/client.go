package main

import (
	"bytes"
	"context"
	"encoding/base64"
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
	Prompt           string
	Size             string
	AspectRatio      string
	ResponseFormat   string
	Width            int
	Height           int
	N                int
	Seed             *int64
	PromptOptimizer  *bool
	AIGCWatermark    *bool
	SubjectReference []SubjectReference
}

type SubjectReference struct {
	Type      string `json:"type,omitempty"`
	ImageFile string `json:"image_file,omitempty"`
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
	if provider.isMiniMax() {
		return c.generateMiniMax(ctx, provider, model, params)
	}
	if len(params.SubjectReference) > 0 {
		return nil, fmt.Errorf("configured image generation provider does not support image-to-image: %s", defaultString(provider.Kind, "openai"))
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
	images := make([]GeneratedImage, 0, len(resp.Data))
	for _, item := range resp.Data {
		images = append(images, GeneratedImage{
			Base64: strings.TrimSpace(item.B64JSON),
			URL:    strings.TrimSpace(item.URL),
			Format: "png",
		})
	}
	if err := hydrateGeneratedImageBase64(ctx, images); err != nil {
		return nil, err
	}
	return buildGenerationResult(c.cfg.TextToImage.Provider, model.Model, map[string]any{"size": size}, images), nil
}

type GeneratedImage struct {
	Base64 string
	URL    string
	Format string
}

func (c *ImageClient) generateMiniMax(ctx context.Context, provider *ImageProviderConfig, model *ImageGenerationModelConfig, params GenerateImageParams) (map[string]any, error) {
	responseFormat := defaultString(params.ResponseFormat, defaultString(model.ResponseFormat, "base64"))
	aspectRatio := defaultString(params.AspectRatio, model.AspectRatio)
	n := params.N
	if n <= 0 {
		n = model.N
	}
	if n <= 0 {
		n = 1
	}
	if n > 9 {
		return nil, fmt.Errorf("minimax n must be between 1 and 9")
	}

	body := map[string]any{
		"model":           model.Model,
		"prompt":          params.Prompt,
		"response_format": responseFormat,
		"n":               n,
	}
	if aspectRatio != "" {
		body["aspect_ratio"] = aspectRatio
	}
	if params.Width > 0 && params.Height > 0 {
		body["width"] = params.Width
		body["height"] = params.Height
	}
	if params.Seed != nil {
		body["seed"] = *params.Seed
	}
	if params.PromptOptimizer != nil {
		body["prompt_optimizer"] = *params.PromptOptimizer
	} else if model.PromptOptimizer {
		body["prompt_optimizer"] = true
	}
	if params.AIGCWatermark != nil {
		body["aigc_watermark"] = *params.AIGCWatermark
	} else if model.AIGCWatermark {
		body["aigc_watermark"] = true
	}
	if len(params.SubjectReference) > 0 {
		body["subject_reference"] = params.SubjectReference
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodPost, joinURL(provider.BaseURL, provider.ImageGenerationPath), bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("Content-Type", "application/json")

	var resp struct {
		ID   string `json:"id"`
		Data struct {
			ImageURLs   []string `json:"image_urls"`
			ImageBase64 []string `json:"image_base64"`
		} `json:"data"`
		Metadata map[string]any `json:"metadata"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := doImageJSON(req, &resp); err != nil {
		return nil, err
	}
	if resp.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("minimax image generation failed: %d %s", resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg)
	}
	images := make([]GeneratedImage, 0, len(resp.Data.ImageBase64)+len(resp.Data.ImageURLs))
	for _, item := range resp.Data.ImageBase64 {
		if item = strings.TrimSpace(item); item != "" {
			images = append(images, GeneratedImage{Base64: item, Format: "png"})
		}
	}
	for _, item := range resp.Data.ImageURLs {
		if item = strings.TrimSpace(item); item != "" {
			images = append(images, GeneratedImage{URL: item, Format: "png"})
		}
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("empty minimax image generation response")
	}
	if err := hydrateGeneratedImageBase64(ctx, images); err != nil {
		return nil, err
	}

	extra := map[string]any{
		"aspect_ratio":    aspectRatio,
		"response_format": responseFormat,
		"n":               n,
	}
	if params.Width > 0 && params.Height > 0 {
		extra["width"] = params.Width
		extra["height"] = params.Height
	}
	if resp.ID != "" {
		extra["id"] = resp.ID
	}
	if len(resp.Metadata) > 0 {
		extra["metadata"] = resp.Metadata
	}
	if len(params.SubjectReference) > 0 {
		extra["mode"] = "image_to_image"
	} else {
		extra["mode"] = "text_to_image"
	}
	return buildGenerationResult(c.cfg.TextToImage.Provider, model.Model, extra, images), nil
}

func hydrateGeneratedImageBase64(ctx context.Context, images []GeneratedImage) error {
	for i := range images {
		if images[i].Base64 != "" || images[i].URL == "" {
			continue
		}
		body, contentType, err := downloadGeneratedImage(ctx, images[i].URL)
		if err != nil {
			return err
		}
		images[i].Base64 = base64.StdEncoding.EncodeToString(body)
		if format := imageFormatFromContentType(contentType); format != "" {
			images[i].Format = format
		}
	}
	return nil
}

func downloadGeneratedImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(imageURL), nil)
	if err != nil {
		return nil, "", fmt.Errorf("build generated image download request: %w", err)
	}
	resp, err := imageHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download generated image: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read generated image: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download generated image status=%d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("download generated image returned empty body")
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func imageFormatFromContentType(contentType string) string {
	contentType = strings.TrimSpace(strings.ToLower(strings.Split(contentType, ";")[0]))
	switch contentType {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	default:
		return ""
	}
}

func buildGenerationResult(providerName, modelName string, extra map[string]any, images []GeneratedImage) map[string]any {
	result := map[string]any{
		"model":    modelName,
		"provider": providerName,
	}
	for k, v := range extra {
		if v != nil && v != "" {
			result[k] = v
		}
	}
	imageItems := make([]map[string]any, 0, len(images))
	for i, img := range images {
		item := map[string]any{
			"index":        i,
			"image_format": defaultString(img.Format, "png"),
			"file_name":    fmt.Sprintf("generated_image_%d.%s", i+1, defaultString(img.Format, "png")),
		}
		if img.Base64 != "" {
			item["image_base64"] = img.Base64
		}
		if img.URL != "" {
			item["image_url"] = img.URL
		}
		imageItems = append(imageItems, item)
	}
	result["images"] = imageItems

	first := images[0]
	fileName := fmt.Sprintf("generated_image_1.%s", defaultString(first.Format, "png"))
	result["file_name"] = fileName
	result["image_format"] = defaultString(first.Format, "png")
	if first.Base64 != "" {
		result["image_base64"] = first.Base64
	}
	if first.URL != "" {
		result["image_url"] = first.URL
	}
	result["app_message"] = map[string]any{
		"message_type": "image",
		"meta": map[string]any{
			"image_base64":  first.Base64,
			"image_url":     first.URL,
			"image_format":  defaultString(first.Format, "png"),
			"file_name":     fileName,
			"source_agent":  "image-agent",
			"source_model":  modelName,
			"source_vendor": providerName,
		},
	}
	return result
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

func (p *ImageProviderConfig) isMiniMax() bool {
	if p == nil {
		return false
	}
	kind := strings.TrimSpace(strings.ToLower(p.Kind))
	if kind == "minimax" || kind == "minimaxi" {
		return true
	}
	return strings.Contains(strings.ToLower(p.BaseURL), "minimax") && strings.Contains(p.ImageGenerationPath, "image_generation")
}
