package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultAPKListLimit = 50
	maxAPKListLimit     = 200
)

type apkPackageInfo struct {
	FileID       string `json:"file_id"`
	FileName     string `json:"file_name"`
	Owner        string `json:"owner,omitempty"`
	RelativePath string `json:"relative_path"`
	FileSize     int64  `json:"file_size"`
	UpdatedAt    int64  `json:"updated_at"`
	Version      string `json:"version,omitempty"`
	FilePath     string `json:"-"`
}

func (b *Bridge) listStoredAPKPackages(owner, query string, limit int) ([]apkPackageInfo, error) {
	items, err := b.scanStoredAPKPackages(owner, query)
	if err != nil {
		return nil, err
	}
	limit = normalizeAPKListLimit(limit)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (b *Bridge) pushStoredAPKPackage(toUser, groupID, fileID, fileName, owner, content string) (map[string]any, error) {
	toUser = strings.TrimSpace(toUser)
	groupID = normalizeGroupID(groupID)
	fileID = strings.TrimSpace(fileID)
	fileName = strings.TrimSpace(fileName)
	owner = strings.TrimSpace(owner)
	content = strings.TrimSpace(content)

	if (toUser == "") == (groupID == "") {
		return nil, fmt.Errorf("exactly one of to_user or group_id is required")
	}
	if fileID == "" && fileName == "" {
		return nil, fmt.Errorf("file_id or file_name is required")
	}

	pkg, err := b.findStoredAPKPackage(fileID, fileName, owner)
	if err != nil {
		return nil, err
	}
	attachment := b.buildStoredAPKAttachment(pkg)
	version := extractApkVersion(pkg.FileName)

	result := map[string]any{
		"source_file_id":   pkg.FileID,
		"source_file_name": pkg.FileName,
		"source_owner":     pkg.Owner,
		"version":          version,
	}
	if attachment.StorageProvider != "" {
		result["storage_provider"] = attachment.StorageProvider
	}
	if attachment.ObjectKey != "" {
		result["object_key"] = attachment.ObjectKey
	}

	if toUser != "" {
		lastVersion, _ := b.lastApkVersionForTarget(toUser)
		if !b.shouldPushApk(toUser, version) {
			return nil, fmt.Errorf("apk version %s is not newer than last sent version %s for user %s", version, lastVersion, toUser)
		}
		if err := b.sendExistingAttachmentMessage(toUser, content, "file", nil, attachment); err != nil {
			return nil, err
		}
		b.recordApkVersion(toUser, version)
		result["to_user"] = toUser
		return result, nil
	}

	targetKey := "group:" + groupID
	lastVersion, _ := b.lastApkVersionForTarget(targetKey)
	if !b.shouldPushApk(targetKey, version) {
		return nil, fmt.Errorf("apk version %s is not newer than last sent version %s for group %s", version, lastVersion, groupID)
	}
	robotAccount, ok := b.groups.RobotAccount(groupID)
	if !ok {
		return nil, fmt.Errorf("group robot account not found")
	}
	recipients, err := b.groups.HumanMembers(groupID)
	if err != nil {
		return nil, err
	}
	if err := b.broadcastGroupMessageWithAttachment(groupID, robotAccount, content, "file", nil, attachment); err != nil {
		return nil, err
	}
	b.recordApkVersion(targetKey, version)
	result["group_id"] = groupID
	result["recipient_count"] = len(recipients)
	result["recipients"] = recipients
	return result, nil
}

func (b *Bridge) scanStoredAPKPackages(owner, query string) ([]apkPackageInfo, error) {
	root := attachmentRootDir(b.cfg.AttachmentStoreDir)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve attachment root: %w", err)
	}
	if _, err := os.Stat(absRoot); err != nil {
		if os.IsNotExist(err) {
			return []apkPackageInfo{}, nil
		}
		return nil, fmt.Errorf("stat attachment root: %w", err)
	}

	owner = strings.TrimSpace(owner)
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]apkPackageInfo, 0)
	walkErr := filepath.Walk(absRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(info.Name()), ".apk") {
			return nil
		}
		item, err := buildAPKPackageInfo(absRoot, path, info)
		if err != nil {
			log.Printf("[Bridge] skip apk package path=%s err=%v", path, err)
			return nil
		}
		if owner != "" && item.Owner != owner {
			return nil
		}
		if query != "" && !matchAPKPackageQuery(item, query) {
			return nil
		}
		items = append(items, item)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("scan attachment root: %w", walkErr)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt != items[j].UpdatedAt {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		if items[i].FileName != items[j].FileName {
			return items[i].FileName < items[j].FileName
		}
		return items[i].FileID < items[j].FileID
	})
	return items, nil
}

func (b *Bridge) findStoredAPKPackage(fileID, fileName, owner string) (*apkPackageInfo, error) {
	if fileID != "" {
		return b.findStoredAPKPackageByFileID(fileID)
	}

	items, err := b.scanStoredAPKPackages(owner, "")
	if err != nil {
		return nil, err
	}
	matches := make([]apkPackageInfo, 0)
	for _, item := range items {
		if strings.EqualFold(item.FileName, fileName) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("apk package %q not found", fileName)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("multiple apk packages matched file_name %q; use file_id instead", fileName)
	}
}

func (b *Bridge) findStoredAPKPackageByFileID(fileID string) (*apkPackageInfo, error) {
	path, err := resolveAttachmentPath(b.cfg.AttachmentStoreDir, fileID)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat stored apk package: %w", err)
	}
	if info.IsDir() || !strings.EqualFold(filepath.Ext(info.Name()), ".apk") {
		return nil, fmt.Errorf("file_id %s does not point to an apk package", fileID)
	}
	root := attachmentRootDir(b.cfg.AttachmentStoreDir)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve attachment root: %w", err)
	}
	item, err := buildAPKPackageInfo(absRoot, path, info)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (b *Bridge) buildStoredAPKAttachment(pkg *apkPackageInfo) *AppAttachment {
	attachment := &AppAttachment{
		MessageType: "file",
		FileID:      pkg.FileID,
		FileName:    pkg.FileName,
		FilePath:    pkg.FilePath,
		FileSize:    int(pkg.FileSize),
		Format:      "apk",
		MIMEType:    "application/vnd.android.package-archive",
	}
	b.applyAttachmentStorageFromFile(pkg.Owner, attachment)
	return attachment
}

func (b *Bridge) lastApkVersionForTarget(target string) (string, bool) {
	b.apkVersionMu.RLock()
	defer b.apkVersionMu.RUnlock()
	version, ok := b.lastApkVersions[target]
	return version, ok
}

func buildAPKPackageInfo(absRoot, path string, info os.FileInfo) (apkPackageInfo, error) {
	fileID, err := buildAttachmentFileID(absRoot, path)
	if err != nil {
		return apkPackageInfo{}, err
	}
	rel, err := filepath.Rel(absRoot, path)
	if err != nil {
		return apkPackageInfo{}, fmt.Errorf("resolve attachment relative path: %w", err)
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	owner := ""
	parts := strings.Split(rel, "/")
	if len(parts) > 1 {
		owner = parts[0]
	}
	return apkPackageInfo{
		FileID:       fileID,
		FileName:     info.Name(),
		Owner:        owner,
		RelativePath: rel,
		FileSize:     info.Size(),
		UpdatedAt:    info.ModTime().UnixMilli(),
		Version:      extractApkVersion(info.Name()),
		FilePath:     path,
	}, nil
}

func matchAPKPackageQuery(item apkPackageInfo, query string) bool {
	if query == "" {
		return true
	}
	for _, candidate := range []string{
		item.FileName,
		item.Owner,
		item.RelativePath,
		item.Version,
	} {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func normalizeAPKListLimit(limit int) int {
	if limit <= 0 {
		return defaultAPKListLimit
	}
	if limit > maxAPKListLimit {
		return maxAPKListLimit
	}
	return limit
}
