package skill

import (
	"blog"
	"encoding/json"
	"fmt"
	"module"
	"time"
)

// SkillContent represents a single learning content item within a skill
// 技能内容项，包含学习的具体内容和时间信息
type SkillContent struct {
	ID          string    `json:"id"`          // 内容项唯一标识
	Title       string    `json:"title"`       // 内容标题
	Description string    `json:"description"` // 内容详细描述
	Status      string    `json:"status"`      // 状态：planned, in_progress, completed
	Priority    int       `json:"priority"`    // 优先级 (1-10)
	TimeSpent   int       `json:"time_spent"`  // 已花费时间（分钟）
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`  // 最后更新时间
	CompletedAt time.Time `json:"completed_at"` // 完成时间
	Resources   []string  `json:"resources"`   // 相关资源链接
	Notes       string    `json:"notes"`       // 学习笔记
}

// Skill represents a skill with multiple learning contents
// 技能定义，包含多个学习内容项
type Skill struct {
	ID          string         `json:"id"`          // 技能唯一标识
	Name        string         `json:"name"`        // 技能名称
	Description string         `json:"description"` // 技能描述
	Category    string         `json:"category"`    // 技能分类
	Level       string         `json:"level"`       // 当前水平：beginner, intermediate, advanced, expert
	TargetLevel string         `json:"target_level"` // 目标水平
	Contents    []SkillContent `json:"contents"`    // 学习内容列表
	Tags        []string       `json:"tags"`        // 技能标签
	CreatedAt   time.Time      `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time      `json:"updated_at"`  // 最后更新时间
	IsActive    bool           `json:"is_active"`   // 是否活跃
	Progress    float64        `json:"progress"`    // 总体进度 (0-100)
}

// SkillManager handles skill operations using the blog system
// 技能管理器，基于博客系统存储技能数据
type SkillManager struct {
	// 使用博客系统作为存储后端
}

// NewSkillManager creates a new SkillManager instance
func NewSkillManager() *SkillManager {
	return &SkillManager{}
}

// generateBlogTitle generates a blog title for a skill
func generateBlogTitle(skillID string) string {
	return fmt.Sprintf("skill-%s", skillID)
}

// AddSkill adds a new skill
func (sm *SkillManager) AddSkill(account string, skill *Skill) error {
	if skill.ID == "" {
		skill.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = time.Now()
	}
	skill.UpdatedAt = time.Now()

	return sm.saveSkillToBlog(account, skill)
}

// GetSkill retrieves a skill by ID
func (sm *SkillManager) GetSkill(account, skillID string) (*Skill, error) {
	title := generateBlogTitle(skillID)
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		return nil, fmt.Errorf("skill not found")
	}

	var skill Skill
	if err := json.Unmarshal([]byte(b.Content), &skill); err != nil {
		return nil, fmt.Errorf("failed to parse skill data: %w", err)
	}

	return &skill, nil
}

// UpdateSkill updates an existing skill
func (sm *SkillManager) UpdateSkill(account string, skill *Skill) error {
	skill.UpdatedAt = time.Now()
	return sm.saveSkillToBlog(account, skill)
}

// DeleteSkill removes a skill
func (sm *SkillManager) DeleteSkill(account, skillID string) error {
	title := generateBlogTitle(skillID)
	ret := blog.DeleteBlogWithAccount(account, title)
	if ret != 0 {
		return fmt.Errorf("failed to delete skill")
	}
	return nil
}

// GetAllSkills retrieves all skills
func (sm *SkillManager) GetAllSkills(account string) ([]*Skill, error) {
	var skills []*Skill

	for _, b := range blog.GetBlogsWithAccount(account) {
		if isSkillBlog(b.Title) {
			var skill Skill
			if err := json.Unmarshal([]byte(b.Content), &skill); err == nil {
				skills = append(skills, &skill)
			}
		}
	}

	return skills, nil
}

// AddSkillContent adds a new content item to a skill
func (sm *SkillManager) AddSkillContent(account, skillID string, content *SkillContent) error {
	skill, err := sm.GetSkill(account, skillID)
	if err != nil {
		return err
	}

	if content.ID == "" {
		content.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if content.CreatedAt.IsZero() {
		content.CreatedAt = time.Now()
	}
	content.UpdatedAt = time.Now()

	// 添加到技能内容列表的开头（最新在前）
	skill.Contents = append([]SkillContent{*content}, skill.Contents...)
	skill.UpdatedAt = time.Now()

	return sm.saveSkillToBlog(account, skill)
}

// UpdateSkillContent updates a content item in a skill
func (sm *SkillManager) UpdateSkillContent(account, skillID, contentID string, content *SkillContent) error {
	skill, err := sm.GetSkill(account, skillID)
	if err != nil {
		return err
	}

	for i := range skill.Contents {
		if skill.Contents[i].ID == contentID {
			content.UpdatedAt = time.Now()
			if content.Status == "completed" && content.CompletedAt.IsZero() {
				content.CompletedAt = time.Now()
			}
			skill.Contents[i] = *content
			skill.UpdatedAt = time.Now()
			return sm.saveSkillToBlog(account, skill)
		}
	}

	return fmt.Errorf("content not found")
}

// DeleteSkillContent removes a content item from a skill
func (sm *SkillManager) DeleteSkillContent(account, skillID, contentID string) error {
	skill, err := sm.GetSkill(account, skillID)
	if err != nil {
		return err
	}

	for i, content := range skill.Contents {
		if content.ID == contentID {
			// 从切片中删除元素
			skill.Contents = append(skill.Contents[:i], skill.Contents[i+1:]...)
			skill.UpdatedAt = time.Now()
			return sm.saveSkillToBlog(account, skill)
		}
	}

	return fmt.Errorf("content not found")
}

// GetSkillContent retrieves a specific content item
func (sm *SkillManager) GetSkillContent(account, skillID, contentID string) (*SkillContent, error) {
	skill, err := sm.GetSkill(account, skillID)
	if err != nil {
		return nil, err
	}

	for _, content := range skill.Contents {
		if content.ID == contentID {
			return &content, nil
		}
	}

	return nil, fmt.Errorf("content not found")
}

// CalculateSkillProgress calculates the overall progress of a skill
func (sm *SkillManager) CalculateSkillProgress(skill *Skill) float64 {
	if len(skill.Contents) == 0 {
		return 0
	}

	completed := 0
	for _, content := range skill.Contents {
		if content.Status == "completed" {
			completed++
		}
	}

	return float64(completed) / float64(len(skill.Contents)) * 100
}

// isSkillBlog checks if a blog title represents a skill
func isSkillBlog(title string) bool {
	return len(title) > 6 && title[:6] == "skill-"
}

// saveSkillToBlog saves a skill as a blog post
func (sm *SkillManager) saveSkillToBlog(account string, skill *Skill) error {
	title := generateBlogTitle(skill.ID)

	// 计算进度
	skill.Progress = sm.CalculateSkillProgress(skill)

	// Convert to JSON
	content, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert skill to JSON: %w", err)
	}

	// Find existing blog or create new one
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		// Create new blog
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "skill",
			AuthType: module.EAuthType_private,
		}
		blog.AddBlogWithAccount(account, ubd)
	} else {
		// Update existing blog
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "skill",
			AuthType: module.EAuthType_private,
		}
		blog.ModifyBlogWithAccount(account, ubd)
	}

	return nil
}

// GetSkillsByCategory retrieves skills filtered by category
func (sm *SkillManager) GetSkillsByCategory(account, category string) ([]*Skill, error) {
	allSkills, err := sm.GetAllSkills(account)
	if err != nil {
		return nil, err
	}

	var filteredSkills []*Skill
	for _, skill := range allSkills {
		if skill.Category == category {
			filteredSkills = append(filteredSkills, skill)
		}
	}

	return filteredSkills, nil
}

// GetSkillsByTag retrieves skills filtered by tag
func (sm *SkillManager) GetSkillsByTag(account, tag string) ([]*Skill, error) {
	allSkills, err := sm.GetAllSkills(account)
	if err != nil {
		return nil, err
	}

	var filteredSkills []*Skill
	for _, skill := range allSkills {
		for _, skillTag := range skill.Tags {
			if skillTag == tag {
				filteredSkills = append(filteredSkills, skill)
				break
			}
		}
	}

	return filteredSkills, nil
}

// GetActiveSkills retrieves only active skills
func (sm *SkillManager) GetActiveSkills(account string) ([]*Skill, error) {
	allSkills, err := sm.GetAllSkills(account)
	if err != nil {
		return nil, err
	}

	var activeSkills []*Skill
	for _, skill := range allSkills {
		if skill.IsActive {
			activeSkills = append(activeSkills, skill)
		}
	}

	return activeSkills, nil
}