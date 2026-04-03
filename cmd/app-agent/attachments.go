package main

import (
	"encoding/base64"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
)

func attachmentRootDir(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "app-attachments"
	}
	return filepath.Clean(root)
}

func buildAttachmentFileID(rootDir, filePath string) (string, error) {
	rootDir = attachmentRootDir(rootDir)
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve attachment root: %w", err)
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve attachment file: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absFile)
	if err != nil {
		return "", fmt.Errorf("resolve attachment relative path: %w", err)
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return "", fmt.Errorf("attachment path escaped root")
	}
	return base64.RawURLEncoding.EncodeToString([]byte(rel)), nil
}

func resolveAttachmentPath(rootDir, fileID string) (string, error) {
	rootDir = attachmentRootDir(rootDir)
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(fileID))
	if err != nil {
		return "", fmt.Errorf("invalid file_id")
	}
	rel := filepath.Clean(filepath.FromSlash(string(decoded)))
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid attachment path")
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve attachment root: %w", err)
	}
	fullPath := filepath.Join(absRoot, rel)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve attachment path: %w", err)
	}
	rootPrefix := absRoot + string(filepath.Separator)
	if absPath != absRoot && !strings.HasPrefix(absPath, rootPrefix) {
		return "", fmt.Errorf("attachment path escaped root")
	}
	return absPath, nil
}

func attachmentMimeType(messageType, fileName, format string) string {
	fileName = strings.TrimSpace(fileName)
	if ext := strings.TrimSpace(filepath.Ext(fileName)); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
	}
	format = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(format)), ".")
	switch strings.TrimSpace(strings.ToLower(messageType)) {
	case "image":
		switch format {
		case "png":
			return "image/png"
		case "gif":
			return "image/gif"
		case "webp":
			return "image/webp"
		case "bmp":
			return "image/bmp"
		case "svg":
			return "image/svg+xml"
		default:
			return "image/jpeg"
		}
	case "audio":
		switch format {
		case "wav":
			return "audio/wav"
		case "mp3":
			return "audio/mpeg"
		case "ogg":
			return "audio/ogg"
		default:
			return "audio/mp4"
		}
	case "zip", "archive":
		return "application/zip"
	default:
		if ct := mime.TypeByExtension("." + format); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}

func cloneMeta(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}
