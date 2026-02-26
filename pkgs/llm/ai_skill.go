package llm

import (
	"control"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"strings"
	"time"
)

// ============================================================================
// AI Skills 可插拔体系
// 用户通过创建博客（ai_skill-xxx）来扩展 AI 行为，无需改代码
// ============================================================================

const aiSkillPrefix = "ai_skill-"

// AISkill 可插拔 AI 技能卡
type AISkill struct {
	Name        string   `json:"name"`                 // 技能名称
	Description string   `json:"description"`          // 技能简述
	Triggers    []string `json:"triggers"`             // 触发关键词
	Instruction string   `json:"instruction"`          // 技能指令（prompt 模板）
	Examples    string   `json:"examples,omitempty"`   // few-shot 示例
	IsActive    bool     `json:"is_active"`            // 是否启用
	Priority    int      `json:"priority,omitempty"`   // 优先级 (1-10, 默认5)
	CreatedAt   string   `json:"created_at,omitempty"` // 创建时间
	UpdatedAt   string   `json:"updated_at,omitempty"` // 更新时间
}

// AISkillTemplate AI 技能卡模板（供 AI 创建技能时参考）
var AISkillTemplate = AISkill{
	Name:        "技能名称",
	Description: "一句话描述这个技能做什么",
	Triggers:    []string{"触发词1", "触发词2"},
	Instruction: "当用户触发此技能时，你应该：\n1. 第一步\n2. 第二步\n3. 第三步",
	Examples:    "用户: 示例问题\nAI: 示例回答",
	IsActive:    true,
	Priority:    5,
}

// LoadActiveSkills 从博客加载所有启用的 AI 技能
func LoadActiveSkills(account string) []AISkill {
	var skills []AISkill

	blogs := control.GetBlogs(account)
	for title, b := range blogs {
		if !strings.HasPrefix(title, aiSkillPrefix) {
			continue
		}

		var skill AISkill
		if err := json.Unmarshal([]byte(b.Content), &skill); err != nil {
			log.WarnF(log.ModuleLLM, "Failed to parse AI skill %s: %v", title, err)
			continue
		}

		if skill.IsActive {
			skills = append(skills, skill)
		}
	}

	log.DebugF(log.ModuleLLM, "Loaded %d active AI skills for account %s", len(skills), account)
	return skills
}

// MatchSkills 根据用户查询匹配相关技能
func MatchSkills(query string, skills []AISkill) []AISkill {
	queryLower := strings.ToLower(query)
	var matched []AISkill

	for _, skill := range skills {
		for _, trigger := range skill.Triggers {
			if strings.Contains(queryLower, strings.ToLower(trigger)) {
				matched = append(matched, skill)
				break
			}
		}
	}

	return matched
}

// BuildSkillsPrompt 将技能列表构建为 System Prompt 片段
func BuildSkillsPrompt(skills []AISkill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("🔌 已安装的 AI 技能:\n")

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("- 【%s】%s (触发: %s)\n",
			skill.Name, skill.Description, strings.Join(skill.Triggers, "/")))
		if skill.Instruction != "" {
			sb.WriteString(fmt.Sprintf("  指令: %s\n", truncateForSession(skill.Instruction, 200)))
		}
	}

	sb.WriteString("\n当用户的问题匹配到上述技能的触发词时，请按照对应技能的指令执行。")
	return sb.String()
}

// BuildMatchedSkillPrompt 为匹配到的特定技能构建详细 prompt
func BuildMatchedSkillPrompt(skill AISkill) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n[激活技能: %s]\n", skill.Name))
	sb.WriteString(skill.Instruction)
	if skill.Examples != "" {
		sb.WriteString(fmt.Sprintf("\n\n参考示例:\n%s", skill.Examples))
	}
	return sb.String()
}

// ============================================================================
// AI 技能管理（供 MCP 工具调用）
// ============================================================================

// ListAllAISkills 列出所有 AI 技能（含停用的）
func ListAllAISkills(account string) string {
	var skills []AISkill

	blogs := control.GetBlogs(account)
	for title, b := range blogs {
		if !strings.HasPrefix(title, aiSkillPrefix) {
			continue
		}

		var skill AISkill
		if err := json.Unmarshal([]byte(b.Content), &skill); err != nil {
			continue
		}
		skills = append(skills, skill)
	}

	result := map[string]interface{}{
		"total":  len(skills),
		"skills": skills,
	}
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes)
}

// CreateAISkill 创建新的 AI 技能
func CreateAISkill(account, name, description string, triggers []string, instruction, examples string) string {
	skill := AISkill{
		Name:        name,
		Description: description,
		Triggers:    triggers,
		Instruction: instruction,
		Examples:    examples,
		IsActive:    true,
		Priority:    5,
		CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}

	// 生成博客标题
	skillID := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	blogTitle := fmt.Sprintf("%s%s", aiSkillPrefix, skillID)

	// 序列化为 JSON
	jsonBytes, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "序列化失败: %v"}`, err)
	}

	// 检查是否已存在
	existingBlog := control.GetBlog(account, blogTitle)
	if existingBlog != nil {
		blogData := &module.UploadedBlogData{
			Title:    blogTitle,
			Content:  string(jsonBytes),
			Tags:     existingBlog.Tags,
			AuthType: existingBlog.AuthType,
		}
		control.ModifyBlog(account, blogData)
	} else {
		blogData := &module.UploadedBlogData{
			Title:    blogTitle,
			Content:  string(jsonBytes),
			Tags:     "AI技能|自动生成",
			AuthType: module.EAuthType_private,
		}
		control.AddBlog(account, blogData)
	}

	log.InfoF(log.ModuleLLM, "AI Skill created: %s (%s)", name, blogTitle)
	return fmt.Sprintf(`{"success": true, "blog_title": "%s", "skill": %s}`, blogTitle, string(jsonBytes))
}

// ToggleAISkill 启用/禁用技能
func ToggleAISkill(account, skillName string, active bool) string {
	// 查找技能
	blogs := control.GetBlogs(account)
	for title, b := range blogs {
		if !strings.HasPrefix(title, aiSkillPrefix) {
			continue
		}

		var skill AISkill
		if err := json.Unmarshal([]byte(b.Content), &skill); err != nil {
			continue
		}

		if strings.EqualFold(skill.Name, skillName) {
			skill.IsActive = active
			skill.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

			jsonBytes, _ := json.MarshalIndent(skill, "", "  ")
			blogData := &module.UploadedBlogData{
				Title:    title,
				Content:  string(jsonBytes),
				Tags:     b.Tags,
				AuthType: b.AuthType,
			}
			control.ModifyBlog(account, blogData)

			status := "启用"
			if !active {
				status = "停用"
			}
			return fmt.Sprintf(`{"success": true, "skill": "%s", "status": "%s"}`, skillName, status)
		}
	}

	return fmt.Sprintf(`{"error": "未找到技能: %s"}`, skillName)
}

// GetSkillTemplate 获取技能模板 JSON
func GetSkillTemplate() string {
	jsonBytes, _ := json.MarshalIndent(AISkillTemplate, "", "  ")
	return string(jsonBytes)
}
