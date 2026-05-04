package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

var videoHTTPClient = &http.Client{}

type VideoClient struct {
	cfg *Config
}

func NewVideoClient(cfg *Config) *VideoClient {
	return &VideoClient{cfg: cfg}
}

type GenerateVideoParams struct {
	Prompt           string
	FirstFrameImage  string
	ModelAlias       string
	Duration         int
	Resolution       string
	PromptOptimizer  *bool
	FastPretreatment *bool
	AIGCWatermark    *bool
}

func (c *VideoClient) Generate(ctx context.Context, params GenerateVideoParams) (map[string]any, error) {
	provider, model, providerName, modelAlias, err := c.resolveVideo(params.ModelAlias)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if strings.EqualFold(model.Model, "MiniMax-Hailuo-2.3-Fast") && strings.TrimSpace(params.FirstFrameImage) == "" {
		return nil, fmt.Errorf("MiniMax-Hailuo-2.3-Fast requires first_frame_image")
	}

	start := time.Now()
	taskID, err := c.createTask(ctx, provider, model, params)
	if err != nil {
		return nil, err
	}
	log.Printf("[VideoClient] task created provider=%s model=%s task_id=%s duration=%v", providerName, model.Model, taskID, time.Since(start))

	status, err := c.pollTask(ctx, provider, taskID)
	if err != nil {
		return nil, err
	}
	file, videoBytes, err := c.retrieveAndDownload(ctx, provider, status.FileID)
	if err != nil {
		return nil, err
	}

	fileName := strings.TrimSpace(file.Filename)
	if fileName == "" {
		fileName = "hailuo_" + taskID + ".mp4"
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if format == "" {
		format = "mp4"
	}

	meta := map[string]any{
		"video_base64":     base64.StdEncoding.EncodeToString(videoBytes),
		"video_format":     format,
		"file_name":        fileName,
		"file_format":      format,
		"file_size":        len(videoBytes),
		"mime_type":        "video/mp4",
		"provider":         providerName,
		"model":            model.Model,
		"model_alias":      modelAlias,
		"task_id":          taskID,
		"minimax_file_id":  status.FileID,
		"download_url":     file.DownloadURL,
		"duration_seconds": effectiveDuration(model, params.Duration),
		"resolution":       effectiveResolution(model, params.Resolution),
		"video_width":      status.VideoWidth,
		"video_height":     status.VideoHeight,
	}

	log.Printf("[VideoClient] generation done provider=%s model=%s task_id=%s bytes=%d duration=%v", providerName, model.Model, taskID, len(videoBytes), time.Since(start))
	return map[string]any{
		"video_base64": meta["video_base64"],
		"video_format": format,
		"file_name":    fileName,
		"file_size":    len(videoBytes),
		"mime_type":    "video/mp4",
		"provider":     providerName,
		"model":        model.Model,
		"task_id":      taskID,
		"file_id":      status.FileID,
		"video_width":  status.VideoWidth,
		"video_height": status.VideoHeight,
		"app_message": map[string]any{
			"message_type": "video",
			"content":      strings.TrimSpace(params.Prompt),
			"meta":         meta,
		},
	}, nil
}

func (c *VideoClient) resolveVideo(modelAlias string) (*VideoProviderConfig, *VideoModelConfig, string, string, error) {
	providerName := strings.TrimSpace(c.cfg.VideoGeneration.Provider)
	alias := strings.TrimSpace(modelAlias)
	if alias == "" {
		alias = strings.TrimSpace(c.cfg.VideoGeneration.Model)
	}
	provider, ok := c.cfg.Providers[providerName]
	if !ok {
		return nil, nil, "", "", fmt.Errorf("video_generation provider not found: %s", providerName)
	}
	model, ok := provider.Models[alias]
	if !ok {
		return nil, nil, "", "", fmt.Errorf("video model not found: %s/%s", providerName, alias)
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return nil, nil, "", "", fmt.Errorf("video provider api_key is required: %s", providerName)
	}
	return &provider, &model, providerName, alias, nil
}

func (c *VideoClient) createTask(ctx context.Context, provider *VideoProviderConfig, model *VideoModelConfig, params GenerateVideoParams) (string, error) {
	body := map[string]any{
		"model":      model.Model,
		"prompt":     strings.TrimSpace(params.Prompt),
		"duration":   effectiveDuration(model, params.Duration),
		"resolution": effectiveResolution(model, params.Resolution),
	}
	if image := strings.TrimSpace(params.FirstFrameImage); image != "" {
		body["first_frame_image"] = image
	}
	setBool(body, "prompt_optimizer", firstBool(params.PromptOptimizer, model.PromptOptimizer))
	setBool(body, "fast_pretreatment", firstBool(params.FastPretreatment, model.FastPretreatment))
	setBool(body, "aigc_watermark", firstBool(params.AIGCWatermark, model.AIGCWatermark))

	bodyJSON, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodPost, joinURL(provider.BaseURL, provider.VideoGenerationPath), bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("Content-Type", "application/json")

	var result struct {
		TaskID   string   `json:"task_id"`
		BaseResp baseResp `json:"base_resp"`
	}
	if err := doJSON(req, &result); err != nil {
		return "", err
	}
	if result.BaseResp.StatusCode != 0 {
		return "", fmt.Errorf("minimax status_code=%d: %s", result.BaseResp.StatusCode, strings.TrimSpace(result.BaseResp.StatusMsg))
	}
	if strings.TrimSpace(result.TaskID) == "" {
		return "", fmt.Errorf("minimax response missing task_id")
	}
	return strings.TrimSpace(result.TaskID), nil
}

type queryVideoStatus struct {
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	FileID      string   `json:"file_id"`
	VideoWidth  int      `json:"video_width"`
	VideoHeight int      `json:"video_height"`
	BaseResp    baseResp `json:"base_resp"`
}

func (c *VideoClient) pollTask(ctx context.Context, provider *VideoProviderConfig, taskID string) (*queryVideoStatus, error) {
	attempts := c.cfg.MaxPollAttempts
	interval := time.Duration(c.cfg.PollIntervalSec) * time.Second
	for i := 0; i < attempts; i++ {
		status, err := c.queryTask(ctx, provider, taskID)
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(strings.TrimSpace(status.Status)) {
		case "success":
			if strings.TrimSpace(status.FileID) == "" {
				return nil, fmt.Errorf("video task succeeded but file_id is empty")
			}
			return status, nil
		case "fail", "failed":
			return nil, fmt.Errorf("video task failed: task_id=%s", taskID)
		}
		log.Printf("[VideoClient] task polling task_id=%s status=%s attempt=%d/%d", taskID, status.Status, i+1, attempts)
		if !sleepContext(ctx, interval) {
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("video task timeout after %d attempts: %s", attempts, taskID)
}

func (c *VideoClient) queryTask(ctx context.Context, provider *VideoProviderConfig, taskID string) (*queryVideoStatus, error) {
	endpoint := joinURL(provider.BaseURL, provider.QueryPath)
	values := url.Values{}
	values.Set("task_id", taskID)
	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	var result queryVideoStatus
	if err := doJSON(req, &result); err != nil {
		return nil, err
	}
	if result.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("minimax status_code=%d: %s", result.BaseResp.StatusCode, strings.TrimSpace(result.BaseResp.StatusMsg))
	}
	return &result, nil
}

type retrievedFile struct {
	FileID      string `json:"file_id"`
	Bytes       int    `json:"bytes"`
	CreatedAt   int64  `json:"created_at"`
	Filename    string `json:"filename"`
	Purpose     string `json:"purpose"`
	DownloadURL string `json:"download_url"`
}

func (c *VideoClient) retrieveAndDownload(ctx context.Context, provider *VideoProviderConfig, fileID string) (*retrievedFile, []byte, error) {
	endpoint := joinURL(provider.BaseURL, provider.FileRetrievePath)
	values := url.Values{}
	values.Set("file_id", fileID)
	req, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	var result struct {
		File     retrievedFile `json:"file"`
		BaseResp baseResp      `json:"base_resp"`
	}
	if err := doJSON(req, &result); err != nil {
		return nil, nil, err
	}
	if result.BaseResp.StatusCode != 0 {
		return nil, nil, fmt.Errorf("minimax status_code=%d: %s", result.BaseResp.StatusCode, strings.TrimSpace(result.BaseResp.StatusMsg))
	}
	if strings.TrimSpace(result.File.DownloadURL) == "" {
		return nil, nil, fmt.Errorf("minimax file response missing download_url")
	}

	downloadReq, err := http.NewRequestWithContext(c.withTimeout(ctx), http.MethodGet, result.File.DownloadURL, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := videoHTTPClient.Do(downloadReq)
	if err != nil {
		return nil, nil, fmt.Errorf("download video: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 32768))
		return nil, nil, fmt.Errorf("download video status=%d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	limit := int64(c.cfg.MaxDownloadBytes)
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read video body: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, nil, fmt.Errorf("video exceeds max_download_bytes=%d", c.cfg.MaxDownloadBytes)
	}
	return &result.File, data, nil
}

type baseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

func (c *VideoClient) withTimeout(ctx context.Context) context.Context {
	if _, ok := ctx.Deadline(); ok {
		return ctx
	}
	newCtx, _ := context.WithTimeout(ctx, time.Duration(c.cfg.RequestTimeoutSec)*time.Second)
	return newCtx
}

func doJSON(req *http.Request, out any) error {
	resp, err := videoHTTPClient.Do(req)
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

func effectiveDuration(model *VideoModelConfig, requested int) int {
	if requested > 0 {
		return requested
	}
	if model.Duration > 0 {
		return model.Duration
	}
	return 6
}

func effectiveResolution(model *VideoModelConfig, requested string) string {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	if strings.TrimSpace(model.Resolution) != "" {
		return strings.TrimSpace(model.Resolution)
	}
	return "768P"
}

func firstBool(primary, fallback *bool) *bool {
	if primary != nil {
		return primary
	}
	return fallback
}

func setBool(body map[string]any, key string, value *bool) {
	if value != nil {
		body[key] = *value
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
