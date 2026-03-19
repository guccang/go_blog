package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	maxFileChars int            // 所有日期文件总字符数（超过触发 LLM 压缩）
	maxEntries   int            // 最大条目数
	expiryDays   int            // 记忆过期天数（0=不过期）

	// LLM 压缩回调（由 bridge 注入，避免循环依赖）
	llmCompactFunc func(entries []MemoryEntry) ([]MemoryEntry, error)

	// toolName → skillName 映射回调（由 bridge 注入）
	skillNameResolver func(toolName string) string
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
		maxFileChars: 50000, // 默认 50K 字符触发压缩
		maxEntries:   200,   // 默认最多 200 条
		expiryDays:   30,    // 默认 30 天过期
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

// SetSkillNameResolver 注入 toolName → skillName 映射回调
func (m *MemoryManager) SetSkillNameResolver(fn func(toolName string) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skillNameResolver = fn
}

// LoadAutoSkillSummary 读取指定技能的汇总文件内容
func (m *MemoryManager) LoadAutoSkillSummary(skillName string) string {
	filePath := m.autoSkillSummaryFilePath(skillName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	// 去掉标题行（如 # coding 技能经验汇总）
	if strings.HasPrefix(content, "#") {
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = strings.TrimSpace(content[idx+1:])
		}
	}
	return content
}

// ========================= 路径工具函数 =========================

// memoryFilePathForDate 返回指定日期的记忆文件路径: memory_2026_03_19.md
func (m *MemoryManager) memoryFilePathForDate(date string) string {
	// "2026-03-19" → "memory_2026_03_19.md"
	safe := strings.ReplaceAll(date, "-", "_")
	return filepath.Join(m.memoryDir, fmt.Sprintf("memory_%s.md", safe))
}

// todayFilePath 返回今天的记忆文件路径
func (m *MemoryManager) todayFilePath() string {
	return m.memoryFilePathForDate(time.Now().Format("2006-01-02"))
}

// autoSkillDatedFilePath 返回按技能+日期的原始经验文件路径
// → memory_auto_skill_coding_2026_03_19.md
func (m *MemoryManager) autoSkillDatedFilePath(skillName, date string) string {
	safe := strings.ReplaceAll(date, "-", "_")
	return filepath.Join(m.memoryDir, fmt.Sprintf("memory_auto_skill_%s_%s.md", skillName, safe))
}

// autoSkillSummaryFilePath 返回技能汇总文件路径（无日期后缀）
// → memory_auto_skill_coding.md
func (m *MemoryManager) autoSkillSummaryFilePath(skillName string) string {
	return filepath.Join(m.memoryDir, fmt.Sprintf("memory_auto_skill_%s.md", skillName))
}

// listAutoSkillDatedFiles 扫描带日期的 auto_skill 原始文件（排除 .done）
func (m *MemoryManager) listAutoSkillDatedFiles() ([]string, error) {
	pattern := filepath.Join(m.memoryDir, "memory_auto_skill_*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	// 过滤：只保留带日期后缀的文件，排除汇总文件和 .done 文件
	var dated []string
	for _, f := range matches {
		base := filepath.Base(f)
		if strings.HasSuffix(base, ".done") {
			continue
		}
		skillName, date := parseAutoSkillFilename(base)
		if skillName != "" && date != "" {
			dated = append(dated, f)
		}
	}
	sort.Strings(dated)
	return dated, nil
}

// parseAutoSkillFilename 从文件名提取 skillName 和 date
// memory_auto_skill_coding_2026_03_19.md → ("coding", "2026-03-19")
// memory_auto_skill_coding.md → ("coding", "")  汇总文件无日期
func parseAutoSkillFilename(filename string) (skillName string, date string) {
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, ".done")
	base = strings.TrimSuffix(base, ".md")
	base = strings.TrimPrefix(base, "memory_auto_skill_")
	if base == "" {
		return "", ""
	}

	// 尝试从末尾匹配日期模式 _YYYY_MM_DD（10 字符 + 2 下划线 = 最后 10 个字符）
	// 格式: {skillname}_2026_03_19
	if len(base) > 11 {
		tail := base[len(base)-10:] // "2026_03_19"
		// 验证日期格式: 4位_2位_2位
		if len(tail) == 10 && tail[4] == '_' && tail[7] == '_' {
			dateCandidate := strings.ReplaceAll(tail, "_", "-")
			// 简单验证是否像日期
			if dateCandidate[0] >= '0' && dateCandidate[0] <= '9' {
				name := base[:len(base)-11] // 去掉 _2026_03_19
				return name, dateCandidate
			}
		}
	}

	// 无日期后缀 → 汇总文件
	return base, ""
}

// listMemoryFiles 扫描普通记忆文件 memory_YYYY_MM_DD.md，排除 auto_skill 文件
func (m *MemoryManager) listMemoryFiles() ([]string, error) {
	pattern := filepath.Join(m.memoryDir, "memory_*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	// 排除 auto_skill 文件
	var normal []string
	for _, f := range matches {
		base := filepath.Base(f)
		if strings.HasPrefix(base, "memory_auto_skill_") {
			continue
		}
		normal = append(normal, f)
	}
	sort.Strings(normal)
	return normal, nil
}

// dateFromFilename 从普通记忆文件名提取日期: memory_2026_03_19.md → 2026-03-19
func dateFromFilename(filename string) string {
	base := filepath.Base(filename)
	// 跳过 auto_skill 文件
	if strings.HasPrefix(base, "memory_auto_skill_") {
		return ""
	}
	base = strings.TrimPrefix(base, "memory_")        // 2026_03_19.md
	base = strings.TrimSuffix(base, ".md")            // 2026_03_19
	return strings.ReplaceAll(base, "_", "-")          // 2026-03-19
}

// ========================= 解析 =========================

// parseDateMemoryFile 解析单日记忆文件内容
// 格式: # 2026-03-19 头 + ### [category] source: content 条目
func parseDateMemoryFile(content, date string) []MemoryEntry {
	var entries []MemoryEntry
	lines := strings.Split(content, "\n")

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

		// 跳过文件标题行: # 2026-03-19
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
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
				Date:     date,
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

// ========================= 核心操作 =========================

// Load 启动时从所有日期文件解析已有记忆
func (m *MemoryManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	files, err := m.listMemoryFiles()
	if err != nil {
		return fmt.Errorf("list memory files: %v", err)
	}

	if len(files) == 0 {
		log.Printf("[Memory] 无记忆文件，从空记忆开始")
		return nil
	}

	var allEntries []MemoryEntry
	for _, f := range files {
		date := dateFromFilename(f)
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("[Memory] 读取 %s 失败: %v，跳过", filepath.Base(f), err)
			continue
		}
		entries := parseDateMemoryFile(string(data), date)
		allEntries = append(allEntries, entries...)
	}

	m.entries = allEntries

	// 启动时清理过期条目
	m.removeExpiredLocked()

	log.Printf("[Memory] 加载 %d 条记忆（来自 %d 个日期文件）", len(m.entries), len(files))
	return nil
}

// AddEntry 追加记忆并写入文件，超限时自动触发压缩
func (m *MemoryManager) AddEntry(entry MemoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.Date == "" {
		entry.Date = time.Now().Format("2006-01-02")
	}

	// auto_skill 条目只写文件，不加入内存 entries（走独立文件体系）
	if entry.Category == "auto_skill" {
		if err := m.appendToFile(entry); err != nil {
			log.Printf("[Memory] 写入 auto_skill 文件失败: %v", err)
		}
		return
	}

	m.entries = append(m.entries, entry)

	// 追加写入对应日期文件
	if err := m.appendToFile(entry); err != nil {
		log.Printf("[Memory] 写入 %s 失败: %v", filepath.Base(m.memoryFilePathForDate(entry.Date)), err)
	}

	// 检查是否需要压缩
	m.checkAndCompactLocked()
}

// appendToFile 追加单条记忆到对应日期文件
func (m *MemoryManager) appendToFile(entry MemoryEntry) error {
	if err := os.MkdirAll(m.memoryDir, 0755); err != nil {
		return fmt.Errorf("create memory dir: %v", err)
	}

	// auto_skill 分流：写入 memory_auto_skill_{skillname}_{date}.md
	if entry.Category == "auto_skill" {
		skillName := extractSkillNameFromContent(entry.Content, m.skillNameResolver)
		if skillName != "" {
			return m.appendAutoSkillEntry(skillName, entry)
		}
		// 找不到 skillName 时回退到普通记忆文件
		log.Printf("[Memory] auto_skill 条目未能提取 skillName，回退到普通文件")
	}

	filePath := m.memoryFilePathForDate(entry.Date)

	// 检查文件是否存在，不存在则先写日期头
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		header := fmt.Sprintf("# %s\n\n", entry.Date)
		if err := os.WriteFile(filePath, []byte(header), 0644); err != nil {
			return fmt.Errorf("write date header: %v", err)
		}
	}

	// 追加条目
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	line := fmt.Sprintf("### [%s] %s: %s\n\n", entry.Category, entry.Source, entry.Content)
	_, err = f.WriteString(line)
	return err
}

// appendAutoSkillEntry 追加 auto_skill 条目到技能+日期文件
func (m *MemoryManager) appendAutoSkillEntry(skillName string, entry MemoryEntry) error {
	filePath := m.autoSkillDatedFilePath(skillName, entry.Date)

	// 检查文件是否存在，不存在则先写标题
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		header := fmt.Sprintf("# %s 技能经验 %s\n\n", skillName, entry.Date)
		if err := os.WriteFile(filePath, []byte(header), 0644); err != nil {
			return fmt.Errorf("write auto_skill header: %v", err)
		}
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	line := fmt.Sprintf("### [%s] %s: %s\n\n", entry.Category, entry.Source, entry.Content)
	_, err = f.WriteString(line)
	return err
}

// extractSkillNameFromContent 从 auto_skill 条目内容中提取工具名，再映射到 skillName
// Content 格式: "工具 ExecuteCode 使用指南（自动生成）:\n..."
func extractSkillNameFromContent(content string, resolver func(string) string) string {
	// 提取 "工具 XXX " 中的工具名
	const prefix = "工具 "
	idx := strings.Index(content, prefix)
	if idx < 0 {
		return ""
	}
	rest := content[idx+len(prefix):]
	spaceIdx := strings.IndexAny(rest, " \t\n")
	if spaceIdx < 0 {
		return ""
	}
	toolName := rest[:spaceIdx]

	// 通过 resolver 映射到 skillName
	if resolver != nil {
		if skillName := resolver(toolName); skillName != "" {
			return skillName
		}
	}

	// 兜底：用 toolName 本身（小写）
	return strings.ToLower(toolName)
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

// removeExpiredLocked 清理过期条目并删除完全过期的日期文件（需持有写锁）
func (m *MemoryManager) removeExpiredLocked() {
	if m.expiryDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -m.expiryDays).Format("2006-01-02")

	// 统计每个日期的条目数，用于判断哪些日期文件可以整个删除
	dateCounts := make(map[string]int)
	for _, entry := range m.entries {
		dateCounts[entry.Date]++
	}

	var kept []MemoryEntry
	removed := 0
	expiredDates := make(map[string]bool)

	for _, entry := range m.entries {
		if entry.Date >= cutoff {
			kept = append(kept, entry)
		} else {
			removed++
			expiredDates[entry.Date] = true
		}
	}

	if removed > 0 {
		m.entries = kept

		// 删除完全过期的日期文件
		for date := range expiredDates {
			filePath := m.memoryFilePathForDate(date)
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				log.Printf("[Memory] 删除过期文件 %s 失败: %v", filepath.Base(filePath), err)
			} else if err == nil {
				log.Printf("[Memory] 删除过期文件: %s", filepath.Base(filePath))
			}
		}

		log.Printf("[Memory] 清理 %d 条过期记忆（超过 %d 天）", removed, m.expiryDays)
	}

	// 同时清理过期的 auto_skill 日期文件
	m.removeExpiredAutoSkillFiles(cutoff)
}

// removeExpiredAutoSkillFiles 清理过期的 auto_skill 日期文件及其 .done 文件
func (m *MemoryManager) removeExpiredAutoSkillFiles(cutoff string) {
	// 扫描所有 auto_skill 日期文件（包括 .done）
	pattern := filepath.Join(m.memoryDir, "memory_auto_skill_*.md*")
	matches, _ := filepath.Glob(pattern)

	for _, f := range matches {
		base := filepath.Base(f)
		cleanBase := strings.TrimSuffix(base, ".done")
		_, date := parseAutoSkillFilename(cleanBase)
		if date == "" {
			continue // 汇总文件，不清理
		}
		if date < cutoff {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				log.Printf("[Memory] 删除过期 auto_skill 文件 %s 失败: %v", base, err)
			} else if err == nil {
				log.Printf("[Memory] 删除过期 auto_skill 文件: %s", base)
			}
		}
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
	if err := m.rewriteFiles(); err != nil {
		log.Printf("[Memory] 压缩重写失败: %v", err)
	}
}

// simpleCompactLocked 简单压缩：优先保留 solution/pattern/preference，截断旧 error（需持有写锁）
func (m *MemoryManager) simpleCompactLocked() {
	targetCount := m.maxEntries * 2 / 3

	// 分类：重要条目（solution/pattern/preference）和普通条目（error）
	// auto_skill 不再混在普通记忆中，跳过
	var important, normal []MemoryEntry
	for _, entry := range m.entries {
		switch entry.Category {
		case "auto_skill":
			continue // auto_skill 走独立文件体系，不参与普通压缩
		case "solution", "pattern", "preference":
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
		if err := m.rewriteFiles(); err != nil {
			log.Printf("[Memory] 过期清理重写失败: %v", err)
		}
	}
}

// rewriteFiles 按日期分组重写所有普通记忆文件（需持有写锁）
// auto_skill 条目走独立文件体系，不参与重写
func (m *MemoryManager) rewriteFiles() error {
	if err := os.MkdirAll(m.memoryDir, 0755); err != nil {
		return err
	}

	// 按 date 分组（排除 auto_skill）
	dateEntries := make(map[string][]MemoryEntry)
	for _, entry := range m.entries {
		if entry.Category == "auto_skill" {
			continue
		}
		dateEntries[entry.Date] = append(dateEntries[entry.Date], entry)
	}

	// 写入每个日期文件
	for date, entries := range dateEntries {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", date))
		for _, entry := range entries {
			sb.WriteString(fmt.Sprintf("### [%s] %s: %s\n\n", entry.Category, entry.Source, entry.Content))
		}
		filePath := m.memoryFilePathForDate(date)
		if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("write %s: %v", filepath.Base(filePath), err)
		}
	}

	// 删除不再有条目的旧日期文件
	existingFiles, err := m.listMemoryFiles()
	if err != nil {
		return fmt.Errorf("list memory files for cleanup: %v", err)
	}
	for _, f := range existingFiles {
		date := dateFromFilename(f)
		if _, exists := dateEntries[date]; !exists {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				log.Printf("[Memory] 删除空日期文件 %s 失败: %v", filepath.Base(f), err)
			} else if err == nil {
				log.Printf("[Memory] 删除空日期文件: %s", filepath.Base(f))
			}
		}
	}

	return nil
}
