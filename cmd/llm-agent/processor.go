package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ========================= TaskContext =========================

// TaskContext 统一任务输入
type TaskContext struct {
	Ctx                context.Context // 可取消的 context（nil 表示不可取消）
	TaskID             string
	Account            string
	Query              string // 用户问题（用于 plan_and_execute）
	Source             string // "web" | "wechat" | "llm_request"
	PreferAudioReply   bool
	Messages           []Message // 预构建消息（nil 则自动构建）
	SelectedTools      []string
	NoTools            bool
	Sink               EventSink
	PersistedAssistant string
	Trace              *RequestTrace // 请求追踪（可选）
}

func isMCPToolListQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}

	hasMCP := strings.Contains(q, "mcp")
	hasTool := strings.Contains(q, "tool") ||
		strings.Contains(q, "tools") ||
		strings.Contains(q, "工具")
	if !hasMCP || !hasTool {
		return false
	}

	intents := []string{
		"有哪些", "列出", "查看", "显示", "全部", "所有", "清单", "目录列表", "盘点", "列表", "都有", "哪些",
		"all", "list", "show", "catalog", "inventory", "enumerate",
	}
	for _, intent := range intents {
		if strings.Contains(q, intent) {
			return true
		}
	}
	return false
}

func (b *Bridge) buildMCPToolListReply(query string, tools []LLMTool) (string, bool) {
	if !isMCPToolListQuery(query) {
		return "", false
	}

	if len(tools) == 0 {
		return "当前没有可用的 MCP 工具。", true
	}

	// 按 agent 分组
	b.catalogMu.RLock()
	agentInfoCopy := make(map[string]AgentInfo, len(b.agentInfo))
	for k, v := range b.agentInfo {
		agentInfoCopy[k] = v
	}
	toolCatalogCopy := make(map[string]string, len(b.toolCatalog))
	for k, v := range b.toolCatalog {
		toolCatalogCopy[k] = v
	}
	b.catalogMu.RUnlock()

	// 构建 tool → agent 映射（使用 sanitized name）
	type groupedTool struct {
		Name string
		Desc string
	}
	agentGroups := make(map[string][]groupedTool)
	ungrouped := make([]groupedTool, 0)

	sorted := make([]LLMTool, len(tools))
	copy(sorted, tools)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Function.Name < sorted[j].Function.Name
	})

	for _, tool := range sorted {
		originalName := b.resolveToolName(tool.Function.Name)
		desc := strings.TrimSpace(tool.Function.Description)
		if desc == "" {
			desc = "无描述"
		}
		gt := groupedTool{Name: originalName, Desc: desc}

		agentID, ok := toolCatalogCopy[originalName]
		if !ok {
			agentID, ok = toolCatalogCopy[tool.Function.Name]
		}
		if ok {
			agentGroups[agentID] = append(agentGroups[agentID], gt)
		} else {
			ungrouped = append(ungrouped, gt)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前共有 %d 个 MCP 工具：\n\n", len(sorted)))

	idx := 1
	for agentID, groupTools := range agentGroups {
		info, hasInfo := agentInfoCopy[agentID]
		if hasInfo && info.Description != "" {
			sb.WriteString(fmt.Sprintf("📦 %s (%s)\n", info.Name, info.Description))
		} else if hasInfo {
			sb.WriteString(fmt.Sprintf("📦 %s\n", info.Name))
		} else {
			sb.WriteString(fmt.Sprintf("📦 %s\n", agentID))
		}
		for _, gt := range groupTools {
			sb.WriteString(fmt.Sprintf("  %d. %s - %s\n", idx, gt.Name, gt.Desc))
			idx++
		}
		sb.WriteString("\n")
	}
	if len(ungrouped) > 0 {
		sb.WriteString("📦 其他\n")
		for _, gt := range ungrouped {
			sb.WriteString(fmt.Sprintf("  %d. %s - %s\n", idx, gt.Name, gt.Desc))
			idx++
		}
	}
	return strings.TrimSpace(sb.String()), true
}

// ========================= 虚拟工具定义 =========================

// executeSkillTool 虚拟工具定义（技能子任务执行）
var executeSkillTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "execute_skill",
		Description: "执行一个技能。这是处理用户任务的首选方式——技能封装了完整的工具集和执行策略，在独立子任务中运行并返回结果。匹配到可用技能时必须优先使用。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"skill_name": {
					"type": "string",
					"description": "技能名称（来自可用技能列表）"
				},
				"query": {
					"type": "string",
					"description": "具体任务描述"
				}
			},
			"required": ["skill_name", "query"]
		}`),
	},
}

// getSkillDetailTool 虚拟工具定义：获取技能详细文档
var getSkillDetailTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "get_skill_detail",
		Description: "获取指定技能的详细文档，包括工具列表、执行策略和历史经验。在决定是否使用某个技能前，可以先查看其详细说明。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"skill_name": {
					"type": "string",
					"description": "技能名称（来自系统提示词中的 Skill 目录）"
				}
			},
			"required": ["skill_name"]
		}`),
	},
}

// ========================= 统一处理函数 =========================

// executeToolCall 执行单个工具调用（供内部使用）
func (b *Bridge) executeToolCall(ctx context.Context, toolName, arguments string, sink EventSink) (*ToolCallResult, error) {
	originalName := b.resolveToolName(toolName)

	b.catalogMu.RLock()
	handler, hasHandler := b.toolHandlers[originalName]
	b.catalogMu.RUnlock()

	if !hasHandler {
		return nil, fmt.Errorf("工具 %s 未找到", originalName)
	}

	return handler(ctx, json.RawMessage(arguments), sink)
}
