package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// PersonaProfile 结构化人设配置（YAML frontmatter + Markdown body）
type PersonaProfile struct {
	Name        string // 助手名称
	Age         string // 年龄
	Gender      string // 性别
	Personality string // 性格描述
	OwnerTitle  string // 对主人的称呼
	Body        string // Markdown 正文（规则等）
	FilePath    string // PERSONA.md 文件路径
}

// ParsePersonaFile 解析 PERSONA.md（YAML frontmatter + body）
// 无 frontmatter 时视为纯 body（向后兼容）
func ParsePersonaFile(content string) *PersonaProfile {
	p := &PersonaProfile{}

	content = strings.TrimSpace(content)
	if content == "" {
		return p
	}

	// 检测 YAML frontmatter（以 --- 开头）
	if !strings.HasPrefix(content, "---") {
		// 无 frontmatter，整体作为 body（向后兼容）
		p.Body = content
		return p
	}

	// 查找结束的 ---
	rest := content[3:] // 跳过开头的 ---
	rest = strings.TrimLeft(rest, "\r\n")
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		// 没有结束标记，整体作为 body
		p.Body = content
		return p
	}

	// 解析 frontmatter 区域（简单 key: value 解析，不引入 YAML 库）
	frontmatter := rest[:endIdx]
	body := strings.TrimSpace(rest[endIdx+4:]) // 跳过 \n---

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
		val := strings.TrimSpace(line[colonIdx+1:])
		// 去除引号
		val = strings.Trim(val, `"'`)

		switch key {
		case "name":
			p.Name = val
		case "age":
			p.Age = val
		case "gender":
			p.Gender = val
		case "personality":
			p.Personality = val
		case "owner_title":
			p.OwnerTitle = val
		}
	}

	p.Body = body
	return p
}

// IsConfigured 检查人设是否已设置（name 非空即视为已设置）
func (p *PersonaProfile) IsConfigured() bool {
	return p.Name != ""
}

// BuildSystemPrompt 生成系统 prompt 开头
// 已设置 → "你是「小悦」，一个22岁的女生..." + body
// 未设置 → "你是 Go Blog 智能助手" + body + 设置提醒
func (p *PersonaProfile) BuildSystemPrompt() string {
	var sb strings.Builder

	if p.IsConfigured() {
		// 已设置人设：生成个性化开头
		sb.WriteString(fmt.Sprintf("你是「%s」", p.Name))
		if p.Age != "" || p.Gender != "" {
			sb.WriteString("，一个")
			if p.Age != "" {
				sb.WriteString(p.Age + "岁的")
			}
			if p.Gender != "" {
				sb.WriteString(p.Gender + "生")
			}
		}
		if p.Personality != "" {
			sb.WriteString("，性格" + p.Personality)
		}
		sb.WriteString("。\n")
		if p.OwnerTitle != "" {
			sb.WriteString(fmt.Sprintf("你称呼用户为「%s」。\n", p.OwnerTitle))
		}
	} else {
		// 未设置：默认身份
		sb.WriteString("你是 Go Blog 智能助手，帮助用户完成各种任务。\n")
	}

	// 追加 body（规则等）
	if p.Body != "" {
		sb.WriteString(p.Body)
		sb.WriteString("\n")
	}

	// 未设置时追加提醒
	if !p.IsConfigured() {
		sb.WriteString("\n【系统提示】你的人设尚未个性化。请在首次回复时友好地提醒用户，可以发送人设信息来定制你的形象。格式示例：\n名字:小悦 年龄:22 性别:女 性格:活泼开朗 称呼:主人\n")
	}

	return sb.String()
}

// personaKeyMap 中文/英文关键词 → 字段名映射
var personaKeyMap = map[string]string{
	"名字": "name", "名称": "name", "name": "name",
	"年龄": "age", "age": "age",
	"性别": "gender", "gender": "gender",
	"性格": "personality", "personality": "personality",
	"称呼": "owner_title", "owner_title": "owner_title",
}

// UpdateFromUserInput 从用户输入解析 key:value 对更新字段
// 支持格式: "名字:小悦 年龄:22 性别:女 性格:活泼开朗 称呼:主人"
// 返回 true 表示成功解析到至少 name 字段
func (p *PersonaProfile) UpdateFromUserInput(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}

	// 解析 key:value 对（空格分隔，但 value 可能包含中文无空格）
	parsed := make(map[string]string)
	// 先按已知关键词切分
	remaining := input
	for {
		// 找到最早出现的关键词
		bestIdx := -1
		bestKey := ""
		bestField := ""
		for keyword, field := range personaKeyMap {
			idx := strings.Index(remaining, keyword+":")
			if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
				bestIdx = idx
				bestKey = keyword
				bestField = field
			}
		}
		if bestIdx < 0 {
			break
		}

		// 提取 value：从 key: 之后到下一个关键词之前
		valueStart := bestIdx + len(bestKey) + 1 // 跳过 "key:"
		rest := remaining[valueStart:]

		// 找下一个关键词的位置
		nextIdx := len(rest)
		for keyword := range personaKeyMap {
			idx := strings.Index(rest, keyword+":")
			if idx > 0 && idx < nextIdx {
				nextIdx = idx
			}
		}

		value := strings.TrimSpace(rest[:nextIdx])
		if value != "" {
			parsed[bestField] = value
		}

		remaining = rest[nextIdx:]
	}

	// 必须至少有 name
	if parsed["name"] == "" {
		return false
	}

	// 更新字段
	p.Name = parsed["name"]
	if v, ok := parsed["age"]; ok {
		p.Age = v
	}
	if v, ok := parsed["gender"]; ok {
		p.Gender = v
	}
	if v, ok := parsed["personality"]; ok {
		p.Personality = v
	}
	if v, ok := parsed["owner_title"]; ok {
		p.OwnerTitle = v
	}

	return true
}

// SaveToFile 将当前人设写回 PERSONA.md（frontmatter + body）
func (p *PersonaProfile) SaveToFile() error {
	if p.FilePath == "" {
		return fmt.Errorf("persona file path not set")
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: \"%s\"\n", p.Name))
	sb.WriteString(fmt.Sprintf("age: \"%s\"\n", p.Age))
	sb.WriteString(fmt.Sprintf("gender: \"%s\"\n", p.Gender))
	sb.WriteString(fmt.Sprintf("personality: \"%s\"\n", p.Personality))
	sb.WriteString(fmt.Sprintf("owner_title: \"%s\"\n", p.OwnerTitle))
	sb.WriteString("---\n\n")
	if p.Body != "" {
		sb.WriteString(p.Body)
		sb.WriteString("\n")
	}

	if err := os.WriteFile(p.FilePath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write persona file: %v", err)
	}

	log.Printf("[Persona] 已保存人设到 %s (name=%s)", p.FilePath, p.Name)
	return nil
}
