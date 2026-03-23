package main

import (
	"encoding/json"
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
		sb.WriteString("\n【系统提示】你的人设尚未个性化。请在首次回复时友好地提醒用户，可以发送人设信息来定制你的形象。格式示例：\n名字:小悦 年龄:22 性别:女 性格:活泼开朗 称呼:主人\n当用户提供了人设信息后，你必须立即调用 set_persona 工具保存，不要只是口头确认。\n")
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

// normalizeColons 将全角冒号替换为半角，统一格式
func normalizeColons(s string) string {
	return strings.ReplaceAll(s, "：", ":")
}

// findKeywordColon 在 s 中查找 "keyword:" 或 "keyword：" 的位置，返回 (起始位置, 冒号后位置)
// 未找到返回 (-1, -1)
func findKeywordColon(s, keyword string) (int, int) {
	// 半角冒号
	idx := strings.Index(s, keyword+":")
	if idx >= 0 {
		return idx, idx + len(keyword) + 1
	}
	// 全角冒号
	idx = strings.Index(s, keyword+"：")
	if idx >= 0 {
		return idx, idx + len(keyword) + len("：")
	}
	return -1, -1
}

// UpdateFromUserInput 从用户输入解析 key:value 对更新字段
// 支持格式: "名字:小悦 年龄:22 性别:女 性格:活泼开朗 称呼:主人"
// 兼容全角冒号、换行分隔
// 返回 true 表示成功解析到至少 name 字段
func (p *PersonaProfile) UpdateFromUserInput(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}

	// 统一换行为空格（支持换行分隔的格式）
	input = strings.ReplaceAll(input, "\r\n", " ")
	input = strings.ReplaceAll(input, "\n", " ")

	// 收集所有 keyword:value 的位置
	type match struct {
		field    string // 标准字段名
		keyStart int    // keyword 起始位置
		valStart int    // value 起始位置（冒号后）
	}
	var matches []match

	for keyword, field := range personaKeyMap {
		keyStart, valStart := findKeywordColon(input, keyword)
		if keyStart >= 0 {
			// 检查是否已有同字段更靠前的匹配（去重：同 field 保留最早的）
			dup := false
			for _, m := range matches {
				if m.field == field {
					if keyStart < m.keyStart {
						m.keyStart = keyStart
						m.valStart = valStart
					}
					dup = true
					break
				}
			}
			if !dup {
				matches = append(matches, match{field: field, keyStart: keyStart, valStart: valStart})
			}
		}
	}

	if len(matches) == 0 {
		return false
	}

	// 按 keyStart 排序
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].keyStart < matches[i].keyStart {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// 提取每个字段的 value（从 valStart 到下一个 match 的 keyStart）
	parsed := make(map[string]string)
	for i, m := range matches {
		end := len(input)
		if i+1 < len(matches) {
			end = matches[i+1].keyStart
		}
		value := strings.TrimSpace(input[m.valStart:end])
		if value != "" {
			parsed[m.field] = value
		}
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

// setPersonaTool 虚拟工具定义（始终注入，由 LLM 判断是否调用）
var setPersonaTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "set_persona",
		Description: "设置或修改助手的人设信息。当用户要求设定、修改人设（名字、年龄、性别、性格、称呼等）时，调用此工具保存。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "助手名称，如 小悦"
				},
				"age": {
					"type": "string",
					"description": "年龄，如 22"
				},
				"gender": {
					"type": "string",
					"description": "性别，如 女"
				},
				"personality": {
					"type": "string",
					"description": "性格描述，如 活泼开朗、有点傲娇"
				},
				"owner_title": {
					"type": "string",
					"description": "对用户的称呼，如 主人"
				}
			},
			"required": ["name"]
		}`),
	},
}

// HandleSetPersona 处理 set_persona 工具调用，更新人设并保存
// 返回 (回复文本, 是否成功)
func (p *PersonaProfile) HandleSetPersona(argsJSON string) (string, bool) {
	var args struct {
		Name        string `json:"name"`
		Age         string `json:"age"`
		Gender      string `json:"gender"`
		Personality string `json:"personality"`
		OwnerTitle  string `json:"owner_title"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("参数解析失败: %v", err), false
	}
	if args.Name == "" {
		return "name 字段不能为空", false
	}

	p.Name = args.Name
	if args.Age != "" {
		p.Age = args.Age
	}
	if args.Gender != "" {
		p.Gender = args.Gender
	}
	if args.Personality != "" {
		p.Personality = args.Personality
	}
	if args.OwnerTitle != "" {
		p.OwnerTitle = args.OwnerTitle
	}

	if err := p.SaveToFile(); err != nil {
		log.Printf("[Persona] 保存失败: %v", err)
		return fmt.Sprintf("保存失败: %v", err), false
	}

	return fmt.Sprintf("人设设置成功: name=%s age=%s gender=%s personality=%s owner_title=%s",
		p.Name, p.Age, p.Gender, p.Personality, p.OwnerTitle), true
}
