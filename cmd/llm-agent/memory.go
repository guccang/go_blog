package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MemoryEntry 单条记忆
type MemoryEntry struct {
	Date     string `json:"date"`     // "2026-03-19"
	Category string `json:"category"` // "error" | "solution" | "pattern" | "preference"
	Source   string `json:"source"`   // "tool_call" | "user" | "auto_skill"
	Content  string `json:"content"`
}

// MemoryManager 记忆管理器
type MemoryManager struct {
	mu           sync.RWMutex
	memoryDir    string // workspace/memory/
	entries      []MemoryEntry
	errorTracker map[string]int // errorKey → 累计次数（用于 skill 迭代触发）
	maxChars     int            // 注入 prompt 的最大字符数
	maxFileChars int            // MEMORY.md 文件最大字符数（超过触发 LLM 压缩）
	maxEntries   int            // 最大条目数
	expiryDays   int            // 记忆过期天数（0=不过期）

	// LLM 压缩回调（由 bridge 注入，避免循环依赖）
	llmCompactFunc func(entries []MemoryEntry) ([]MemoryEntry, error)
}

// NewMemoryManager 创建记忆管理器
func NewMemoryManager(memoryDir string, maxChars int) *MemoryManager {
	if maxChars <= 0 {
		maxChars = 8000
	}
	return &MemoryManager{
		memoryDir:    memoryDir,
		errorTracker: make(map[string]int),
		maxChars:     maxChars,
		maxFileChars: 50000,  // 默认 50K 字符触发压缩
		maxEntries:   200,    // 默认最多 200 条
		expiryDays:   30,     // 默认 30 天过期
	}
}

// SetLimits 设置大小限制和过期时间
func (m *MemoryManager) SetLimits(maxFileChars, maxEntries, expiryDays int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if maxFileChars > 0 {
		m.maxFileChars = maxFileChars
	}
	if maxEntries > 0 {
		m.maxEntries = maxEntries
	}
	if expiryDays > 0 {
		m.expiryDays = expiryDays
	}
}

// SetLLMCompactFunc 注入 LLM 压缩回调
func (m *MemoryManager) SetLLMCompactFunc(fn func(entries []MemoryEntry) ([]MemoryEntry, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llmCompactFunc = fn
}

// memoryFilePath 返回 MEMORY.md 的完整路径
func (m *MemoryManager) memoryFilePath() string {
	return filepath.Join(m.memoryDir, "MEMORY.md")
}

// Load 启动时从 MEMORY.md 解析已有记忆
func (m *MemoryManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.memoryFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Memory] MEMORY.md 不存在，从空记忆开始")
			return nil
		}
		return fmt.Errorf("read MEMORY.md: %v", err)
	}

	m.entries = parseMemoryMD(string(data))

	// 启动时清理过期条目
	m.removeExpiredLocked()

	log.Printf("[Memory] 加载 %d 条记忆", len(m.entries))
	return nil
}

// parseMemoryMD 解析 MEMORY.md 格式
func parseMemoryMD(content string) []MemoryEntry {
	var entries []MemoryEntry
	lines := strings.Split(content, "\n")

	var currentDate string
	var currentEntry *MemoryEntry
	var contentBuf strings.Builder

	flushEntry := func() {
		if currentEntry != nil {
			currentEntry.Content = strings.TrimSpace(contentBuf.String())
			if currentEntry.Content != "" {
				entries = append(entries, *currentEntry)
			}
			currentEntry = nil
			contentBuf.Reset()
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 日期标题: ## 2026-03-19
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			currentDate = strings.TrimPrefix(trimmed, "## ")
			continue
		}

		// 条目标题: ### [category] source: 描述
		if strings.HasPrefix(trimmed, "### [") {
			flushEntry()

			rest := strings.TrimPrefix(trimmed, "### [")
			closeBracket := strings.Index(rest, "]")
			if closeBracket < 0 {
				continue
			}
			category := rest[:closeBracket]
			afterBracket := strings.TrimSpace(rest[closeBracket+1:])

			source := "unknown"
			desc := afterBracket
			if colonIdx := strings.Index(afterBracket, ":"); colonIdx > 0 {
				source = strings.TrimSpace(afterBracket[:colonIdx])
				desc = strings.TrimSpace(afterBracket[colonIdx+1:])
			}

			currentEntry = &MemoryEntry{
				Date:     currentDate,
				Category: category,
				Source:   source,
			}
			contentBuf.WriteString(desc)
			contentBuf.WriteString("\n")
			continue
		}

		// 普通内容行
		if currentEntry != nil && trimmed != "" {
			contentBuf.WriteString(line)
			contentBuf.WriteString("\n")
		}
	}

	flushEntry()
	return entries
}

// AddEntry 追加记忆并写入文件，超限时自动触发压缩
func (m *MemoryManager) AddEntry(entry MemoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.Date == "" {
		entry.Date = time.Now().Format("2006-01-02")
	}

	m.entries = append(m.entries, entry)

	// 追加写入 MEMORY.md
	if err := m.appendToFile(entry); err != nil {
		log.Printf("[Memory] 写入 MEMORY.md 失败: %v", err)
	}

	// 检查是否需要压缩
	m.checkAndCompactLocked()
}

// appendToFile 追加单条记忆到文件
func (m *MemoryManager) appendToFile(entry MemoryEntry) error {
	if err := os.MkdirAll(m.memoryDir, 0755); err != nil {
		return fmt.Errorf("create memory dir: %v", err)
	}

	existing, _ := os.ReadFile(m.memoryFilePath())
	content := string(existing)

	dateHeader := fmt.Sprintf("## %s", entry.Date)
	needDateHeader := !strings.Contains(content, dateHeader)

	var sb strings.Builder
	if needDateHeader {
		if len(content) > 0 && !strings.HasSuffix(content, "\n\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(dateHeader)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("### [%s] %s: %s\n\n", entry.Category, entry.Source, entry.Content))

	f, err := os.OpenFile(m.memoryFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(sb.String())
	return err
}

// BuildPromptBlock 构建注入 system prompt 的记忆文本
func (m *MemoryManager) BuildPromptBlock() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 长期记忆（历史经验）\n")
	sb.WriteString("以下是从历史任务中积累的经验教训，请在执行任务时参考：\n\n")

	// 从最新的条目开始，直到达到字符限制
	totalChars := 0
	var selected []MemoryEntry
	for i := len(m.entries) - 1; i >= 0; i-- {
		entry := m.entries[i]
		entryLen := len(entry.Content) + len(entry.Category) + len(entry.Date) + 20
		if totalChars+entryLen > m.maxChars {
			break
		}
		selected = append([]MemoryEntry{entry}, selected...)
		totalChars += entryLen
	}

	currentDate := ""
	for _, entry := range selected {
		if entry.Date != currentDate {
			currentDate = entry.Date
			sb.WriteString(fmt.Sprintf("### %s\n", entry.Date))
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", entry.Category, entry.Content))
	}
	sb.WriteString("\n")

	return sb.String()
}

// TrackError 累计错误次数，返回当前计数
func (m *MemoryManager) TrackError(errorKey string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorTracker[errorKey]++
	return m.errorTracker[errorKey]
}

// GetErrorCount 获取某个错误键的累计次数
func (m *MemoryManager) GetErrorCount(errorKey string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errorTracker[errorKey]
}

// ========================= 大小限制 + 过期 + LLM 压缩 =========================

// removeExpiredLocked 清理过期条目（需持有写锁）
func (m *MemoryManager) removeExpiredLocked() {
	if m.expiryDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -m.expiryDays).Format("2006-01-02")
	var kept []MemoryEntry
	removed := 0
	for _, entry := range m.entries {
		// 日期格式 "2006-01-02"，字符串比较即可
		if entry.Date >= cutoff {
			kept = append(kept, entry)
		} else {
			removed++
		}
	}

	if removed > 0 {
		m.entries = kept
		log.Printf("[Memory] 清理 %d 条过期记忆（超过 %d 天）", removed, m.expiryDays)
	}
}

// checkAndCompactLocked 检查是否超限，超限则触发压缩（需持有写锁）
func (m *MemoryManager) checkAndCompactLocked() {
	// 先清理过期
	m.removeExpiredLocked()

	// 计算当前文件字符数
	totalChars := 0
	for _, entry := range m.entries {
		totalChars += len(entry.Content) + len(entry.Category) + len(entry.Date) + 30
	}

	needCompact := len(m.entries) > m.maxEntries || totalChars > m.maxFileChars
	if !needCompact {
		return
	}

	log.Printf("[Memory] 触发压缩: entries=%d/%d chars=%d/%d",
		len(m.entries), m.maxEntries, totalChars, m.maxFileChars)

	// 尝试 LLM 智能压缩
	if m.llmCompactFunc != nil {
		compacted, err := m.llmCompactFunc(m.entries)
		if err != nil {
			log.Printf("[Memory] LLM 压缩失败: %v，回退到简单压缩", err)
			m.simpleCompactLocked()
		} else {
			m.entries = compacted
			log.Printf("[Memory] LLM 压缩完成: %d 条记忆", len(m.entries))
		}
	} else {
		m.simpleCompactLocked()
	}

	// 重写文件
	if err := m.rewriteFile(); err != nil {
		log.Printf("[Memory] 压缩重写 MEMORY.md 失败: %v", err)
	}
}

// simpleCompactLocked 简单压缩：优先保留 solution/pattern/preference，截断旧 error（需持有写锁）
func (m *MemoryManager) simpleCompactLocked() {
	targetCount := m.maxEntries * 2 / 3

	// 分类：重要条目（solution/pattern/preference/auto_skill）和普通条目（error）
	var important, normal []MemoryEntry
	for _, entry := range m.entries {
		switch entry.Category {
		case "solution", "pattern", "preference", "auto_skill":
			important = append(important, entry)
		default:
			normal = append(normal, entry)
		}
	}

	// 重要条目全部保留，普通条目只保留最近的
	var result []MemoryEntry
	result = append(result, important...)

	remaining := targetCount - len(important)
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 0 && len(normal) > remaining {
		normal = normal[len(normal)-remaining:]
	}
	result = append(result, normal...)

	m.entries = result
	log.Printf("[Memory] 简单压缩: 保留 %d 条（重要 %d + 普通 %d）",
		len(result), len(important), len(result)-len(important))
}

// Compact 外部触发压缩（公开方法）
func (m *MemoryManager) Compact(maxEntries int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if maxEntries > 0 {
		m.maxEntries = maxEntries
	}
	m.checkAndCompactLocked()
}

// CleanupExpired 外部触发过期清理
func (m *MemoryManager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	before := len(m.entries)
	m.removeExpiredLocked()
	if len(m.entries) != before {
		if err := m.rewriteFile(); err != nil {
			log.Printf("[Memory] 过期清理重写失败: %v", err)
		}
	}
}

// rewriteFile 重写整个 MEMORY.md（需持有写锁）
func (m *MemoryManager) rewriteFile() error {
	if err := os.MkdirAll(m.memoryDir, 0755); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# LLM Agent Memory\n\n")

	currentDate := ""
	for _, entry := range m.entries {
		if entry.Date != currentDate {
			currentDate = entry.Date
			sb.WriteString(fmt.Sprintf("## %s\n\n", entry.Date))
		}
		sb.WriteString(fmt.Sprintf("### [%s] %s: %s\n\n", entry.Category, entry.Source, entry.Content))
	}

	return os.WriteFile(m.memoryFilePath(), []byte(sb.String()), 0644)
}
