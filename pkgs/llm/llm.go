// Package llm provides LLM (Large Language Model) integration functionality.
//
// This package is organized into the following modules:
//   - config.go: Configuration management (LLMConfig, InitConfig)
//   - message.go: Message types and sanitization (Message, SanitizeMessages)
//   - diary.go: Diary/blog saving functionality (SaveLLMResponseToDiary)
//   - stream.go: Stream processing and tool detection (ProcessStreamingResponseWithToolDetection)
//   - client.go: LLM API client (SendStreamingLLMRequest)
//   - tool_handler.go: Tool execution loop (ToolExecutor, ProcessQueryStreaming)
//   - http_handler.go: HTTP request handling (ProcessRequest)
//   - ai_skill.go: Pluggable AI skills system
package llm

import (
	"fmt"
	"mcp"
	log "mylog"
	"strings"
)

// Info prints module version information
func Info() {
	log.Debug(log.ModuleLLM, "info llm v1.0")
}

// Init initializes the LLM module
func Init() error {
	InitConfig()
	registerSkillTools()
	return nil
}

// registerSkillTools 注册 AI 技能管理的 MCP 工具
func registerSkillTools() {
	// ListAISkills - 列出所有 AI 技能
	mcp.RegisterCallBack("ListAISkills", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			return `{"error": "缺少 account 参数"}`
		}
		result := ListAllAISkills(account)
		template := GetSkillTemplate()
		return fmt.Sprintf(`{"skills": %s, "template": %s, "instruction": "展示所有已安装的 AI 技能。如果用户想创建新技能，使用 CreateAISkill 工具，参考 template 中的格式。"}`, result, template)
	})
	mcp.RegisterCallBackPrompt("ListAISkills", "展示所有 AI 技能列表，包含模板供参考")

	// CreateAISkill - 创建新 AI 技能
	mcp.RegisterCallBack("CreateAISkill", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		name, _ := args["name"].(string)
		description, _ := args["description"].(string)
		instruction, _ := args["instruction"].(string)
		examples, _ := args["examples"].(string)

		if account == "" || name == "" || instruction == "" {
			return `{"error": "缺少必要参数: account, name, instruction"}`
		}

		// 解析 triggers
		var triggers []string
		if triggersRaw, ok := args["triggers"]; ok {
			switch v := triggersRaw.(type) {
			case string:
				triggers = strings.Split(v, ",")
				for i := range triggers {
					triggers[i] = strings.TrimSpace(triggers[i])
				}
			case []interface{}:
				for _, t := range v {
					if s, ok := t.(string); ok {
						triggers = append(triggers, s)
					}
				}
			}
		}
		if len(triggers) == 0 {
			triggers = []string{name}
		}

		return CreateAISkill(account, name, description, triggers, instruction, examples)
	})
	mcp.RegisterCallBackPrompt("CreateAISkill", "创建新的 AI 技能卡，技能会自动保存并在后续对话中生效")

	// ToggleAISkill - 启用/禁用技能
	mcp.RegisterCallBack("ToggleAISkill", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		skillName, _ := args["name"].(string)
		if account == "" || skillName == "" {
			return `{"error": "缺少参数: account, name"}`
		}

		active := true
		if v, ok := args["active"]; ok {
			switch a := v.(type) {
			case bool:
				active = a
			case string:
				active = strings.ToLower(a) != "false"
			}
		}

		return ToggleAISkill(account, skillName, active)
	})
	mcp.RegisterCallBackPrompt("ToggleAISkill", "启用或停用指定的 AI 技能")

	// GetSkillTemplate - 获取技能模板
	mcp.RegisterCallBack("GetSkillTemplate", func(args map[string]interface{}) string {
		template := GetSkillTemplate()
		return fmt.Sprintf(`{"template": %s, "instruction": "这是 AI 技能卡的模板格式。创建新技能时，请参考此模板填写各字段。使用 CreateAISkill 工具创建。"}`, template)
	})
	mcp.RegisterCallBackPrompt("GetSkillTemplate", "获取 AI 技能卡的创建模板")

	log.InfoF(log.ModuleLLM, "AI Skill MCP tools registered (ListAISkills, CreateAISkill, ToggleAISkill, GetSkillTemplate)")
}
