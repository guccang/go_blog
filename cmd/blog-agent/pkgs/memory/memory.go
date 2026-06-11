// Package memory 实现私人管家的用户记忆库。
// 借鉴 MiMo Code：用可审阅的 Markdown 文本 + 关键词检索，而非向量库。
// 存储复用 blog 体系（Redis + blogs_txt 文件，按 account 隔离，私有权限）。
package memory

import (
	"blog"
	"encoding/json"
	"fmt"
	"module"
	"sort"
	"strings"
	"time"
)

const (
	titlePrefix  = "记忆_"
	tagMemory    = "记忆"
	FileMemory   = "MEMORY"     // 长期事实：喜好、习惯、忌讳、重要的人、纪念日
	FileCheck    = "checkpoint" // 当前关注点：本周重点、进行中的事、管家下一步
	FileGoals    = "goals"      // 目标树视图（G1/G1.1，judge 判据）
	journalKey   = "journal"    // journal 文件名形如 journal_2026-06-11
	maxFileBytes = 256 * 1024
)

// 记忆文件白名单（journal 单独处理）
var coreFiles = []string{FileMemory, FileCheck, FileGoals}

func memoryTitle(file string) string {
	return titlePrefix + file
}

func journalFile(date string) string {
	return journalKey + "_" + date
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func nowStamp() string {
	return time.Now().Format("2006-01-02 15:04")
}

// ValidFile 校验文件名是否为合法记忆文件（核心文件或 journal_YYYY-MM-DD）。
func ValidFile(file string) bool {
	for _, name := range coreFiles {
		if file == name {
			return true
		}
	}
	if strings.HasPrefix(file, journalKey+"_") {
		suffix := strings.TrimPrefix(file, journalKey+"_")
		_, err := time.Parse("2006-01-02", suffix)
		return err == nil
	}
	return false
}

// ReadFile 读取记忆文件内容；不存在返回空字符串。
func ReadFile(account, file string) (string, error) {
	if !ValidFile(file) {
		return "", fmt.Errorf("非法记忆文件名: %s（允许 MEMORY/checkpoint/goals/journal_YYYY-MM-DD）", file)
	}
	b := blog.GetBlogWithAccount(account, memoryTitle(file))
	if b == nil {
		return "", nil
	}
	return b.Content, nil
}

// WriteFile 整体写入记忆文件（不存在则创建）。
func WriteFile(account, file, content string) error {
	if !ValidFile(file) {
		return fmt.Errorf("非法记忆文件名: %s", file)
	}
	if len(content) > maxFileBytes {
		return fmt.Errorf("记忆文件过大（%d 字节，上限 %d）", len(content), maxFileBytes)
	}
	udb := &module.UploadedBlogData{
		Title:    memoryTitle(file),
		Content:  content,
		AuthType: module.EAuthType_private,
		Tags:     tagMemory,
		Encrypt:  0,
		Account:  account,
	}
	if blog.GetBlogWithAccount(account, udb.Title) != nil {
		if ret := blog.ModifyBlogWithAccount(account, udb); ret != 0 {
			return fmt.Errorf("更新记忆文件失败: %s", file)
		}
		return nil
	}
	if ret := blog.AddBlogWithAccount(account, udb); ret != 0 {
		return fmt.Errorf("创建记忆文件失败: %s", file)
	}
	return nil
}

// Append 在记忆文件末尾追加一条带时间戳的条目。
func Append(account, file, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("记忆内容为空")
	}
	existing, err := ReadFile(account, file)
	if err != nil {
		return err
	}
	entry := fmt.Sprintf("- [%s] %s", nowStamp(), content)
	merged := strings.TrimRight(existing, "\n")
	if merged == "" {
		merged = entry
	} else {
		merged = merged + "\n" + entry
	}
	return WriteFile(account, file, merged+"\n")
}

// AppendJournal 写入当日 journal。
func AppendJournal(account, content string) (string, error) {
	file := journalFile(today())
	if err := Append(account, file, content); err != nil {
		return "", err
	}
	return file, nil
}

// ListFiles 返回该账号全部记忆文件名（核心文件 + 现存 journal，按名称倒序，journal 新日期在前）。
func ListFiles(account string) []string {
	all := blog.GetBlogsWithAccount(account)
	var files []string
	for title := range all {
		if !strings.HasPrefix(title, titlePrefix) {
			continue
		}
		name := strings.TrimPrefix(title, titlePrefix)
		if ValidFile(name) {
			files = append(files, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

// SearchHit 一条检索命中。
type SearchHit struct {
	File  string `json:"file"`
	Line  string `json:"line"`
	Score int    `json:"score"`
}

// Search 关键词检索：按空白分词，命中词数计分，逐行匹配。
// 检索范围：核心文件 + 最近 recentJournalDays 天的 journal。
func Search(account, query string, limit, recentJournalDays int) []SearchHit {
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	if recentJournalDays <= 0 {
		recentJournalDays = 14
	}

	files := append([]string{}, coreFiles...)
	cutoff := time.Now().AddDate(0, 0, -recentJournalDays).Format("2006-01-02")
	for _, name := range ListFiles(account) {
		if strings.HasPrefix(name, journalKey+"_") &&
			strings.TrimPrefix(name, journalKey+"_") >= cutoff {
			files = append(files, name)
		}
	}

	var hits []SearchHit
	for _, file := range files {
		content, err := ReadFile(account, file)
		if err != nil || content == "" {
			continue
		}
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			score := scoreLine(trimmed, terms)
			if score > 0 {
				hits = append(hits, SearchHit{File: file, Line: trimmed, Score: score})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func tokenize(query string) []string {
	var terms []string
	for _, term := range strings.Fields(strings.ToLower(query)) {
		term = strings.TrimSpace(term)
		if term != "" {
			terms = append(terms, term)
		}
	}
	return terms
}

func scoreLine(line string, terms []string) int {
	lower := strings.ToLower(line)
	score := 0
	for _, term := range terms {
		if strings.Contains(lower, term) {
			score++
		}
	}
	return score
}

// SearchJSON 返回检索结果的 JSON 字符串（供 MCP 工具直接使用）。
func SearchJSON(account, query string, limit, recentJournalDays int) string {
	hits := Search(account, query, limit, recentJournalDays)
	if hits == nil {
		hits = []SearchHit{}
	}
	data, err := json.Marshal(hits)
	if err != nil {
		return "[]"
	}
	return string(data)
}
