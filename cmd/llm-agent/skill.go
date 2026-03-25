package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// SkillEntry 单个 skill 的元数据和内容
type SkillEntry struct {
	Name        string   // YAML name
	Description string   // YAML description（展示在 skill 目录中）
	Summary     string   // YAML summary（用法要点，展示在目录中）
	Tools       []string // YAML tools（关联的工具名列表）
	Agents      []string // YAML agents（所需 agent 前缀列表，如 blog-agent, exec_code）
	Keywords    []string // YAML keywords（用于静态匹配的关键词列表）
	Content     string   // Markdown body（frontmatter 之后的正文）
	FilePath    string   // 文件路径（调试用）
}

// AgentOnlineChecker agent 在线检查函数类型
// 传入 agent 前缀，返回是否有匹配的在线 agent
type AgentOnlineChecker func(prefix string) bool

// SkillManager skill 加载与匹配管理器
type SkillManager struct {
	skills             []SkillEntry
	workspaceDir       string
	memoryDir          string             // workspace/memory/，用于加载 auto_skill 汇总
	agentOnlineChecker AgentOnlineChecker // 检查 agent 是否在线
}

// NewSkillManager 创建 skill 管理器
func NewSkillManager(workspaceDir string) *SkillManager {
	return &SkillManager{
		workspaceDir: workspaceDir,
	}
}

// Load 扫描 workspace/skills/*/SKILL.md，解析 YAML frontmatter 并加载
func (sm *SkillManager) Load() error {
	skillsDir := filepath.Join(sm.workspaceDir, "skills")

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[SkillManager] skills 目录不存在: %s", skillsDir)
			return nil
		}
		return fmt.Errorf("read skills dir: %v", err)
	}

	var loaded []SkillEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			log.Printf("[SkillManager] 跳过 %s: %v", skillFile, err)
			continue
		}

		skill, err := parseSkillFile(string(data), skillFile)
		if err != nil {
			log.Printf("[SkillManager] 解析失败 %s: %v", skillFile, err)
			continue
		}

		loaded = append(loaded, *skill)
	}

	sm.skills = loaded

	// 打印加载摘要
	var names []string
	for _, s := range sm.skills {
		names = append(names, s.Name)
	}
	log.Printf("[SkillManager] loaded %d skills: %s", len(sm.skills), strings.Join(names, ", "))

	return nil
}

// parseSkillFile 解析 SKILL.md：提取 YAML frontmatter（--- 之间）和正文
func parseSkillFile(content, filePath string) (*SkillEntry, error) {
	content = strings.TrimSpace(content)

	// 检查是否以 --- 开头
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing frontmatter start (---)")
	}

	// 找到第二个 ---
	rest := content[3:]
	rest = strings.TrimLeft(rest, "\r\n")
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return nil, fmt.Errorf("missing frontmatter end (---)")
	}

	frontmatter := rest[:endIdx]
	body := strings.TrimSpace(rest[endIdx+4:]) // 跳过 \n---

	// 逐行解析 YAML（简单 key: value 格式，不依赖外部库）
	skill := &SkillEntry{
		FilePath: filePath,
		Content:  body,
	}

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = value
		case "summary":
			skill.Summary = value
		case "tools":
			// 逗号分隔的工具名列表
			for _, t := range strings.Split(value, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					skill.Tools = append(skill.Tools, t)
				}
			}
		case "keywords":
			// 逗号分隔的关键词列表（用于静态匹配）
			for _, k := range strings.Split(value, ",") {
				k = strings.TrimSpace(k)
				if k != "" {
					skill.Keywords = append(skill.Keywords, k)
				}
			}
		case "agents":
			// 逗号分隔的所需 agent 前缀列表
			for _, a := range strings.Split(value, ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					skill.Agents = append(skill.Agents, a)
				}
			}
		}
	}

	if skill.Name == "" {
		return nil, fmt.Errorf("missing 'name' in frontmatter")
	}

	return skill, nil
}

// GetAllSkills 返回所有已加载的 skill
func (sm *SkillManager) GetAllSkills() []SkillEntry {
	return sm.skills
}

// GetAvailableSkills 返回所需 agent 均在线的 skill（过滤掉不可用的）
func (sm *SkillManager) GetAvailableSkills() []SkillEntry {
	var available []SkillEntry
	for _, skill := range sm.skills {
		if sm.isSkillAvailable(&skill) {
			available = append(available, skill)
		}
	}
	return available
}

// GetSkillOwnedTools 收集所有 skill 声明的工具名集合
func (sm *SkillManager) GetSkillOwnedTools() map[string]bool {
	owned := make(map[string]bool)
	for _, skill := range sm.skills {
		for _, t := range skill.Tools {
			owned[t] = true
		}
	}
	return owned
}

// MatchByTools 仅通过工具名匹配 skill（用于子任务）
func (sm *SkillManager) MatchByTools(toolHints []string) []SkillEntry {
	if len(sm.skills) == 0 || len(toolHints) == 0 {
		return nil
	}

	hintSet := make(map[string]bool, len(toolHints)*2)
	for _, t := range toolHints {
		hintSet[t] = true
		// 兼容命名空间格式：deploy_DeployProject → DeployProject
		if dot := strings.LastIndex(t, "."); dot >= 0 {
			hintSet[t[dot+1:]] = true
		}
		if us := strings.Index(t, "_"); us >= 0 {
			hintSet[t[us+1:]] = true
		}
	}

	var matched []SkillEntry
	for _, skill := range sm.skills {
		for _, t := range skill.Tools {
			if hintSet[t] {
				matched = append(matched, skill)
				break
			}
		}
	}

	return matched
}

// BuildCatalog 构建 skill 目录文本（Level 1，含 summary 用法要点）
func (sm *SkillManager) BuildCatalog() string {
	if len(sm.skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用技能\n")
	for _, skill := range sm.skills {
		if skill.Summary != "" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s — %s\n", skill.Name, skill.Description, skill.Summary))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Name, skill.Description))
		}
	}
	return sb.String()
}

// BuildCatalogWithToolHint 构建 skill 目录，提示 LLM 通过 execute_skill 工具使用
// agent 不在线的技能标注为不可用
func (sm *SkillManager) BuildCatalogWithToolHint() string {
	if len(sm.skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用技能\n")
	sb.WriteString("当用户请求匹配以下技能的适用场景时，调用 execute_skill 工具执行。\n")
	sb.WriteString("使用前可调用 get_skill_detail(skill_name) 查看详细文档和参数说明。\n")
	sb.WriteString("技能内部会获取完整的专业工具集，不要绕过技能直接 call_tool。\n\n")
	for _, skill := range sm.skills {
		offline := sm.offlineAgents(&skill)
		if len(offline) > 0 {
			// 标注不可用
			sb.WriteString(fmt.Sprintf("- ~~**%s**~~: %s [不可用: agent %s offline]\n",
				skill.Name, skill.Description, strings.Join(offline, ", ")))
			continue
		}
		if skill.Summary != "" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s — %s\n", skill.Name, skill.Description, skill.Summary))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Name, skill.Description))
		}
		if len(skill.Keywords) > 0 {
			sb.WriteString(fmt.Sprintf("  适用: %s\n", strings.Join(skill.Keywords, ", ")))
		}
	}
	return sb.String()
}

// GetSkill 按名称查找 skill
func (sm *SkillManager) GetSkill(name string) *SkillEntry {
	for i := range sm.skills {
		if sm.skills[i].Name == name {
			return &sm.skills[i]
		}
	}
	return nil
}

// SetMemoryDir 设置记忆目录路径（用于加载 auto_skill 汇总文件）
func (sm *SkillManager) SetMemoryDir(dir string) {
	sm.memoryDir = dir
}

// SetAgentOnlineChecker 注入 agent 在线检查函数
func (sm *SkillManager) SetAgentOnlineChecker(checker AgentOnlineChecker) {
	sm.agentOnlineChecker = checker
}

// isSkillAvailable 检查技能所需的所有 agent 是否在线
// 无 agents 声明的技能始终可用
func (sm *SkillManager) isSkillAvailable(skill *SkillEntry) bool {
	if len(skill.Agents) == 0 || sm.agentOnlineChecker == nil {
		return true
	}
	for _, prefix := range skill.Agents {
		if !sm.agentOnlineChecker(prefix) {
			return false
		}
	}
	return true
}

// offlineAgents 返回技能所需但不在线的 agent 列表
func (sm *SkillManager) offlineAgents(skill *SkillEntry) []string {
	if len(skill.Agents) == 0 || sm.agentOnlineChecker == nil {
		return nil
	}
	var offline []string
	for _, prefix := range skill.Agents {
		if !sm.agentOnlineChecker(prefix) {
			offline = append(offline, prefix)
		}
	}
	return offline
}

// BuildSkillBlock 构建匹配到的 skill 正文（Level 2，按需注入）
func (sm *SkillManager) BuildSkillBlock(matched []SkillEntry) string {
	if len(matched) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 技能指引\n")
	for _, skill := range matched {
		sb.WriteString(fmt.Sprintf("\n### %s\n", skill.Name))
		sb.WriteString(skill.Content)
		// 加载对应的 auto_skill 汇总
		if summary := sm.loadAutoSkillSummary(skill.Name); summary != "" {
			sb.WriteString("\n\n#### 历史经验补充\n")
			sb.WriteString(summary)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// loadAutoSkillSummary 读取指定技能的 auto_skill 汇总文件
func (sm *SkillManager) loadAutoSkillSummary(skillName string) string {
	if sm.memoryDir == "" {
		return ""
	}
	filePath := filepath.Join(sm.memoryDir, fmt.Sprintf("memory_auto_skill_%s.md", skillName))
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
