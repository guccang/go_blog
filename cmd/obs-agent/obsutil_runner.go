package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type proxyUploadRequest struct {
	LocalPath    string
	ObjectKey    string
	ContentType  string
	OriginalName string
}

type proxyUploader interface {
	Upload(ctx context.Context, req proxyUploadRequest) error
}

type obsutilUploader struct {
	cfg *Config
}

func newProxyUploader(cfg *Config) proxyUploader {
	if cfg == nil {
		return nil
	}
	return &obsutilUploader{cfg: cfg}
}

func (u *obsutilUploader) Upload(ctx context.Context, req proxyUploadRequest) error {
	if u == nil || u.cfg == nil {
		return fmt.Errorf("obsutil uploader is not configured")
	}
	binaryPath, err := resolveObsutilBinary(u.cfg)
	if err != nil {
		return err
	}
	bucket := strings.TrimSpace(u.cfg.OBS.Bucket)
	if bucket == "" {
		return fmt.Errorf("obs bucket is required")
	}
	objectKey := normalizeOBSObjectKey(u.cfg.OBS.KeyPrefix, req.ObjectKey)
	if objectKey == "" {
		return fmt.Errorf("object key is required")
	}

	args := []string{
		"cp",
		req.LocalPath,
		fmt.Sprintf("obs://%s/%s", bucket, objectKey),
		"-f",
		"-e=" + strings.TrimSpace(u.cfg.OBS.Endpoint),
		"-i=" + strings.TrimSpace(u.cfg.OBS.AccessKey),
		"-k=" + strings.TrimSpace(u.cfg.OBS.SecretKey),
	}
	if strings.TrimSpace(req.ContentType) != "" {
		args = append(args, "-meta=Content-Type:"+req.ContentType)
	}

	timeout := time.Duration(u.cfg.ObsutilTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("obsutil upload timeout after %s: %s", timeout, trimCommandOutput(output))
		}
		return fmt.Errorf("obsutil upload failed: %s", trimCommandOutput(output))
	}
	return nil
}

func resolveObsutilBinary(cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	candidates := make([]string, 0, 4)
	if p := strings.TrimSpace(cfg.ObsutilPath); p != "" {
		candidates = append(candidates, p)
	}
	platformDir := runtimePlatformDir()
	binaryName := runtimeBinaryName()
	if exe, err := os.Executable(); err == nil {
		baseDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(baseDir, "obsutil", platformDir, binaryName))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "obsutil", platformDir, binaryName))
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("obsutil binary not found, expected one of: %s", strings.Join(candidates, ", "))
}

func runtimePlatformDir() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

func runtimeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "obsutil.exe"
	}
	return "obsutil"
}

func normalizeOBSObjectKey(prefix, key string) string {
	key = strings.Trim(strings.TrimSpace(key), "/")
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	switch {
	case prefix == "":
		return key
	case key == "":
		return prefix
	default:
		return prefix + "/" + key
	}
}

func trimCommandOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "no command output"
	}
	const limit = 2048
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}
