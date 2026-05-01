package main

import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ConfigWatcher 监控 settings 目录变化并自动触发配置重载。
// 通过轮询方式对比文件 hash，避免引入外部文件监控依赖。
type ConfigWatcher struct {
	settingsDir string
	configPath  string
	reloadFn    func() error
	interval    time.Duration
	debounce    time.Duration
	stopCh      chan struct{}
	lastHash    string
	mu          sync.Mutex
}

// NewConfigWatcher 创建配置监控器。
// settingsDir 为要监控的 settings 目录绝对路径。
// reloadFn 为检测到变化时的重载回调。
func NewConfigWatcher(settingsDir string, reloadFn func() error) *ConfigWatcher {
	return &ConfigWatcher{
		settingsDir: settingsDir,
		reloadFn:    reloadFn,
		interval:    30 * time.Second,
		debounce:    3 * time.Second,
		stopCh:      make(chan struct{}),
	}
}

// Start 启动后台监控循环，阻塞直到 Stop() 被调用。
// 应在单独的 goroutine 中调用。
func (w *ConfigWatcher) Start() {
	w.mu.Lock()
	w.lastHash = w.computeHash()
	w.mu.Unlock()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			newHash := w.computeHash()

			w.mu.Lock()
			changed := newHash != "" && newHash != w.lastHash
			w.mu.Unlock()

			if !changed {
				continue
			}

			log.Printf("[ConfigWatcher] settings changed detected, debouncing %v...", w.debounce)
			time.Sleep(w.debounce)

			// 消除抖动后重新计算 hash
			finalHash := w.computeHash()

			w.mu.Lock()
			if finalHash == w.lastHash || finalHash == "" {
				w.mu.Unlock()
				continue // 抖动期间无实质性变更
			}
			w.lastHash = finalHash
			w.mu.Unlock()

			log.Printf("[ConfigWatcher] reloading config...")
			if err := w.reloadFn(); err != nil {
				log.Printf("[ConfigWatcher] reload config failed: %v", err)
			} else {
				log.Printf("[ConfigWatcher] config reloaded successfully")
			}
		}
	}
}

// Stop 停止监控循环。
func (w *ConfigWatcher) Stop() {
	select {
	case <-w.stopCh:
		// already stopped
	default:
		close(w.stopCh)
	}
}

// computeHash 计算 settings 目录下所有 .json 文件的 MD5 哈希值。
// 目录不存在或读取失败时返回空字符串（触发安全处理）。
func (w *ConfigWatcher) computeHash() string {
	if w.settingsDir == "" {
		return ""
	}

	h := md5.New()
	count := 0

	_ = filepath.WalkDir(w.settingsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		h.Write([]byte(path))
		h.Write(data)
		count++
		return nil
	})

	if count == 0 {
		return ""
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}
