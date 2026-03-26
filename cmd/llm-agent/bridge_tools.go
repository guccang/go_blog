package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"uap"
)

// ========================= 工具名称转换 =========================

// sanitizeToolName 将工具名转为 LLM 兼容格式（. → _）
func sanitizeToolName(name string) string {
	result := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			result[i] = '_'
		} else {
			result[i] = name[i]
		}
	}
	return string(result)
}

// unsanitizeToolName 将 LLM 函数名还原为原始工具名（_ → .）
// 只替换第一个 _（命名空间分隔符），其余保留
func unsanitizeToolName(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == '_' {
			return name[:i] + "." + name[i+1:]
		}
	}
	return name
}

// ========================= 工具查询 =========================

// getToolAgent 查找工具所属的 agent
func (b *Bridge) getToolAgent(toolName string) (string, bool) {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	agentID, ok := b.toolCatalog[toolName]
	return agentID, ok
}

// getSiblingTools 获取与指定工具同 agent 的所有兄弟工具
// 用于工具业务失败时扩展可选工具集，让 LLM 自行决策是修复参数重试还是切换替代工具
func (b *Bridge) getSiblingTools(toolName string) []LLMTool {
	agentID, ok := b.getToolAgent(toolName)
	if !ok {
		return nil
	}
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	return b.agentTools[agentID]
}

// getLLMTools 获取 LLM 工具列表
func (b *Bridge) getLLMTools() []LLMTool {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	return b.llmTools
}

// filterToolsBySelection 根据用户选择过滤工具列表
// selectedTools 为空时返回全部工具
func (b *Bridge) filterToolsBySelection(selectedTools []string) []LLMTool {
	allTools := b.getLLMTools()
	if len(selectedTools) == 0 {
		return allTools
	}

	// 构建 O(1) 查找表，同时支持 sanitized 名称（下划线）和原始名称（点号）
	selectedMap := make(map[string]bool, len(selectedTools)*2)
	for _, name := range selectedTools {
		selectedMap[name] = true
		selectedMap[sanitizeToolName(name)] = true
	}

	var filtered []LLMTool
	for _, tool := range allTools {
		if selectedMap[tool.Function.Name] {
			filtered = append(filtered, tool)
		}
	}

	if len(filtered) == 0 {
		log.Printf("[Bridge] no tools matched selection %v, not using tools", selectedTools)
		return nil
	}

	log.Printf("[Bridge] filtered %d tools from %d by user selection", len(filtered), len(allTools))
	return filtered
}

// ========================= 全量工具目录 =========================

// buildBriefToolCatalog 构建简要的 Agent + 工具目录（仅工具名+短描述，不含参数）
// LLM 需要参数详情时通过 get_tool_detail 按需获取
func (b *Bridge) buildBriefToolCatalog() string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	if len(b.agentInfo) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用 Agent 与工具\n")
	sb.WriteString("以下是所有在线 Agent 及其工具。需要了解工具参数时，调用 get_tool_detail(tool_name) 获取完整参数定义。\n\n")

	// llm-agent 自身信息
	sb.WriteString(fmt.Sprintf("### %s [%s]: LLM 编排中枢", b.cfg.AgentName, b.cfg.AgentID))
	if b.client.HostPlatform != "" {
		sb.WriteString(fmt.Sprintf(" | 平台: %s", b.client.HostPlatform))
	}
	sb.WriteString("\n\n")

	for id, info := range b.agentInfo {
		agentToolList := b.agentTools[id]

		// Agent 标题行：名称 [ID]: 描述
		desc := info.Description
		if info.DetailDescription != "" {
			desc = truncateToFirstParagraph(info.DetailDescription, 200)
		}
		if desc == "" {
			desc = info.Name
		}

		line := fmt.Sprintf("### %s [%s]: %s", info.Name, id, desc)

		// 附加关键能力标签
		var tags []string
		if info.HostPlatform != "" {
			tags = append(tags, "平台: "+info.HostPlatform)
		}
		if len(info.DeployTargets) > 0 {
			tags = append(tags, "部署: "+strings.Join(info.DeployTargets, ","))
		}
		if len(info.SSHHosts) > 0 {
			tags = append(tags, "SSH: "+strings.Join(info.SSHHosts, ","))
		}
		if len(info.Models) > 0 {
			tags = append(tags, "模型: "+strings.Join(info.Models, ","))
		}
		if len(info.CodingTools) > 0 {
			tags = append(tags, "编码: "+strings.Join(info.CodingTools, ","))
		}
		if info.PythonVersion != "" {
			tags = append(tags, "Python: "+info.PythonVersion)
		}
		if len(info.LogSources) > 0 {
			var sources []string
			for k := range info.LogSources {
				sources = append(sources, k)
			}
			tags = append(tags, "日志: "+strings.Join(sources, ","))
		}
		if len(info.Pipelines) > 0 {
			tags = append(tags, "流水线: "+strings.Join(info.Pipelines, ","))
		}
		if len(tags) > 0 {
			line += " | " + strings.Join(tags, " | ")
		}
		sb.WriteString(line + "\n")

		// 每个工具一行：仅名称+短描述（不含参数摘要）
		for _, t := range agentToolList {
			name := b.resolveToolNameLocked(t.Function.Name)
			toolDesc := strings.TrimSpace(t.Function.Description)
			if toolDesc == "" {
				toolDesc = "无描述"
			}
			if len([]rune(toolDesc)) > 50 {
				toolDesc = string([]rune(toolDesc)[:50]) + "..."
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", name, toolDesc))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildFullToolCatalog 构建完整的 Agent + 工具目录（含参数摘要，用于需要详细信息的场景）
func (b *Bridge) buildFullToolCatalog() string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	if len(b.agentInfo) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用 Agent 与工具\n")
	sb.WriteString("以下是所有在线 Agent 及其工具。直接调用工具即可，无需额外发现步骤。\n\n")

	// llm-agent 自身信息
	sb.WriteString(fmt.Sprintf("### %s [%s]: LLM 编排中枢", b.cfg.AgentName, b.cfg.AgentID))
	if b.client.HostPlatform != "" {
		sb.WriteString(fmt.Sprintf(" | 平台: %s", b.client.HostPlatform))
	}
	sb.WriteString("\n\n")

	for id, info := range b.agentInfo {
		agentToolList := b.agentTools[id]

		// Agent 标题行：名称 [ID]: 描述
		desc := info.Description
		if info.DetailDescription != "" {
			desc = truncateToFirstParagraph(info.DetailDescription, 200)
		}
		if desc == "" {
			desc = info.Name
		}

		line := fmt.Sprintf("### %s [%s]: %s", info.Name, id, desc)

		// 附加关键能力标签
		var tags []string
		if info.HostPlatform != "" {
			tags = append(tags, "平台: "+info.HostPlatform)
		}
		if len(info.DeployTargets) > 0 {
			tags = append(tags, "部署: "+strings.Join(info.DeployTargets, ","))
		}
		if len(info.SSHHosts) > 0 {
			tags = append(tags, "SSH: "+strings.Join(info.SSHHosts, ","))
		}
		if len(info.Models) > 0 {
			tags = append(tags, "模型: "+strings.Join(info.Models, ","))
		}
		if len(info.CodingTools) > 0 {
			tags = append(tags, "编码: "+strings.Join(info.CodingTools, ","))
		}
		if info.PythonVersion != "" {
			tags = append(tags, "Python: "+info.PythonVersion)
		}
		if len(info.LogSources) > 0 {
			var sources []string
			for k := range info.LogSources {
				sources = append(sources, k)
			}
			tags = append(tags, "日志: "+strings.Join(sources, ","))
		}
		if len(info.Pipelines) > 0 {
			tags = append(tags, "流水线: "+strings.Join(info.Pipelines, ","))
		}
		if len(tags) > 0 {
			line += " | " + strings.Join(tags, " | ")
		}
		sb.WriteString(line + "\n")

		// 每个工具一行：紧凑格式
		for _, t := range agentToolList {
			name := b.resolveToolNameLocked(t.Function.Name)
			toolDesc := strings.TrimSpace(t.Function.Description)
			if toolDesc == "" {
				toolDesc = "无描述"
			}
			// 截断过长描述
			if len([]rune(toolDesc)) > 80 {
				toolDesc = string([]rune(toolDesc)[:80]) + "..."
			}
			paramSummary := extractParamSummary(t.Function.Parameters)
			if paramSummary != "" {
				sb.WriteString(fmt.Sprintf("  - %s: %s | 参数: %s\n", name, toolDesc, paramSummary))
			} else {
				sb.WriteString(fmt.Sprintf("  - %s: %s\n", name, toolDesc))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// resolveToolNameLocked 在已持有 catalogMu 读锁时解析工具名
func (b *Bridge) resolveToolNameLocked(name string) string {
	if canonical, ok := b.toolNameMap[name]; ok {
		return canonical
	}
	return name
}

// getAgentDetailTool 虚拟工具定义（按需获取 Agent 详细信息）
var getAgentDetailTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "get_agent_detail",
		Description: "获取指定 Agent 的详细信息，包括完整描述、工具列表、平台信息等。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"agent_id": {"type": "string", "description": "Agent ID（如 deploy-agent、cron-agent）"}
			},
			"required": ["agent_id"]
		}`),
	},
}

// getToolDetailTool 虚拟工具定义（按需获取工具参数详情）
var getToolDetailTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "get_tool_detail",
		Description: "获取工具的完整参数定义（JSON Schema）。在调用不熟悉的工具前使用。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"tool_name": {"type": "string", "description": "工具名称（如 DeployProject、ExecuteCode）"},
				"agent_id":  {"type": "string", "description": "Agent ID，查看该 Agent 所有工具参数（与 tool_name 二选一）"}
			}
		}`),
	},
}

// handleGetToolDetail 处理 get_tool_detail 请求（共享逻辑，tool_handler 和 processor 都调用）
func (b *Bridge) handleGetToolDetail(toolName, agentID string) *ToolCallResult {
	if toolName == "" && agentID == "" {
		return &ToolCallResult{Result: "请提供 tool_name 或 agent_id 参数", AgentID: "builtin"}
	}

	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	if toolName != "" {
		// 单个工具查询
		sanitized := sanitizeToolName(toolName)
		for _, tool := range b.llmTools {
			if tool.Function.Name == sanitized || tool.Function.Name == toolName {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", toolName, tool.Function.Description))
				sb.WriteString("参数 Schema:\n")
				sb.WriteString(string(tool.Function.Parameters))
				log.Printf("[Bridge] get_tool_detail: tool=%s", toolName)
				return &ToolCallResult{Result: sb.String(), AgentID: "builtin"}
			}
		}
		return &ToolCallResult{Result: fmt.Sprintf("工具 '%s' 不存在", toolName), AgentID: "builtin"}
	}

	// Agent 级查询
	agentToolList, ok := b.agentTools[agentID]
	if !ok {
		return &ToolCallResult{Result: fmt.Sprintf("Agent '%s' 不存在或无工具", agentID), AgentID: "builtin"}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Agent %s 的工具参数\n\n", agentID))
	for _, t := range agentToolList {
		name := b.resolveToolNameLocked(t.Function.Name)
		sb.WriteString(fmt.Sprintf("### %s\n%s\n参数: %s\n\n", name, t.Function.Description, string(t.Function.Parameters)))
	}
	log.Printf("[Bridge] get_tool_detail: agent=%s tools=%d", agentID, len(agentToolList))
	return &ToolCallResult{Result: sb.String(), AgentID: "builtin"}
}

// handleGetAgentDetail 处理 get_agent_detail 请求
func (b *Bridge) handleGetAgentDetail(agentID string) *ToolCallResult {
	if agentID == "" {
		return &ToolCallResult{Result: "请提供 agent_id 参数", AgentID: "builtin"}
	}

	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	info, ok := b.agentInfo[agentID]
	if !ok {
		// 列出可用 agent
		var ids []string
		for id := range b.agentInfo {
			ids = append(ids, id)
		}
		return &ToolCallResult{
			Result:  fmt.Sprintf("Agent '%s' 不存在。可用 Agent: %s", agentID, strings.Join(ids, ", ")),
			AgentID: "builtin",
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Agent: %s (%s)\n", info.Name, info.ID))
	if info.Description != "" {
		sb.WriteString(fmt.Sprintf("简介: %s\n", info.Description))
	}
	if info.DetailDescription != "" {
		sb.WriteString(fmt.Sprintf("\n### 详细说明\n%s\n", info.DetailDescription))
	}
	if info.HostPlatform != "" {
		sb.WriteString(fmt.Sprintf("运行平台: %s\n", info.HostPlatform))
	}
	if info.HostIP != "" {
		sb.WriteString(fmt.Sprintf("主机IP: %s\n", info.HostIP))
	}
	if info.Workspace != "" {
		sb.WriteString(fmt.Sprintf("工作目录: %s\n", info.Workspace))
	}
	if len(info.CodingTools) > 0 {
		sb.WriteString(fmt.Sprintf("编码工具: %s\n", strings.Join(info.CodingTools, ", ")))
	}
	if len(info.Models) > 0 {
		sb.WriteString(fmt.Sprintf("模型配置: %s\n", strings.Join(info.Models, ", ")))
	}
	if len(info.SSHHosts) > 0 {
		sb.WriteString(fmt.Sprintf("SSH主机: %s\n", strings.Join(info.SSHHosts, ", ")))
	}
	if len(info.DeployTargets) > 0 {
		sb.WriteString(fmt.Sprintf("部署目标: %s\n", strings.Join(info.DeployTargets, ", ")))
	}
	if len(info.TargetHosts) > 0 {
		sb.WriteString("部署目标→SSH地址:\n")
		for target, host := range info.TargetHosts {
			sb.WriteString(fmt.Sprintf("  - %s → %s\n", target, host))
		}
	}
	if len(info.Pipelines) > 0 {
		sb.WriteString(fmt.Sprintf("Pipeline: %s\n", strings.Join(info.Pipelines, ", ")))
	}
	if info.PythonVersion != "" {
		sb.WriteString(fmt.Sprintf("Python: %s\n", info.PythonVersion))
	}
	if len(info.LogSources) > 0 {
		sb.WriteString("日志源:\n")
		for name, desc := range info.LogSources {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", name, desc))
		}
	}
	if len(info.SupportedSoftware) > 0 {
		sb.WriteString(fmt.Sprintf("支持软件: %s\n", strings.Join(info.SupportedSoftware, ", ")))
	}

	// 工具列表
	if tools, ok := b.agentTools[agentID]; ok && len(tools) > 0 {
		sb.WriteString(fmt.Sprintf("\n### 工具列表 (%d 个)\n", len(tools)))
		for _, t := range tools {
			name := b.resolveToolNameLocked(t.Function.Name)
			desc := t.Function.Description
			if len([]rune(desc)) > 80 {
				desc = string([]rune(desc)[:80]) + "..."
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", name, desc))
		}
	}

	log.Printf("[Bridge] get_agent_detail: agent=%s", agentID)
	return &ToolCallResult{Result: sb.String(), AgentID: "builtin"}
}

// injectVirtualTools 集中注入虚拟工具
func (b *Bridge) injectVirtualTools(tools []LLMTool, noTools bool) []LLMTool {
	if noTools {
		return tools
	}
	tools = append(tools, getSkillDetailTool, getToolDetailTool, getAgentDetailTool, planAndExecuteTool)
	if b.skillMgr != nil && len(b.skillMgr.GetAllSkills()) > 0 {
		tools = append(tools, executeSkillTool)
	}
	if b.persona != nil {
		tools = append(tools, setPersonaTool)
	}
	if b.memoryMgr != nil {
		tools = append(tools, setRuleTool)
	}
	// 条件注入模型切换工具
	if b.hasMultipleModels() {
		tools = append(tools, listProvidersTool, getCurrentModelTool, switchProviderTool, switchModelTool)
	}
	return tools
}

// ========================= 跨 Agent 工具调用 =========================

// longRunningTools 需要长超时的工具（编码、部署等耗时操作）
var longRunningTools = map[string]bool{
	"CodegenStartSession": true,
	"CodegenSendMessage":  true,
	"AcpStartSession":     true,
	"AcpSendMessage":      true,
	"AcpAnalyzeProject":   true,
	"DeployProject":       true,
	"DeployAdhoc":         true,
	"DeployPipeline":      true,
	"ExecuteCode":         true,
}

// isLongRunningTool 判断是否为长时间运行的工具
func isLongRunningTool(toolName string) bool {
	return longRunningTools[toolName]
}

// ToolCallResult 工具调用结果（含路由信息）
type ToolCallResult struct {
	Result  string // 工具返回内容
	AgentID string // 目标 agent ID（发送方）
	FromID  string // 结果来源 agent ID（响应方）
}

// CallTool 统一工具调用入口（无 context）
func (b *Bridge) CallTool(toolName string, args json.RawMessage) (*ToolCallResult, error) {
	return b.DispatchTool(context.Background(), toolName, args, nil)
}

// CallToolCtx context 感知的工具调用，支持级联取消
func (b *Bridge) CallToolCtx(ctx context.Context, toolName string, args json.RawMessage) (*ToolCallResult, error) {
	return b.DispatchTool(ctx, toolName, args, nil)
}

// CallToolCtxWithProgress context 感知的工具调用，支持进度回调转发
func (b *Bridge) CallToolCtxWithProgress(ctx context.Context, toolName string, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
	return b.DispatchTool(ctx, toolName, args, sink)
}

// callRemoteAgent 发送 tool_call 到远程 agent 并等待 MsgToolResult
// 从原 callToolCtxWithSink 提取，纯 UAP 消息收发
func (b *Bridge) callRemoteAgent(ctx context.Context, toolName, agentID string, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("cancelled before tool call %s: %v", toolName, err)
		}
	} else {
		ctx = context.Background()
	}

	msgID := uap.NewMsgID()
	ch := make(chan *toolResultWithFrom, 1)

	b.pendMu.Lock()
	b.pending[msgID] = ch
	b.pendMu.Unlock()

	// 注册进度回调 sink（deploy-agent 的 tool_progress 会通过 msgID 关联）
	if sink != nil {
		b.toolProgressMu.Lock()
		b.toolProgressSinks[msgID] = sink
		b.toolProgressMu.Unlock()
	}

	defer func() {
		b.pendMu.Lock()
		delete(b.pending, msgID)
		b.pendMu.Unlock()
		if sink != nil {
			b.toolProgressMu.Lock()
			delete(b.toolProgressSinks, msgID)
			b.toolProgressMu.Unlock()
		}
	}()

	log.Printf("[Bridge] tool_call → agent=%s tool=%s msgID=%s", agentID, toolName, msgID)

	err := b.client.Send(&uap.Message{
		Type: uap.MsgToolCall,
		ID:   msgID,
		From: b.cfg.AgentID,
		To:   agentID,
		Payload: mustMarshal(uap.ToolCallPayload{
			ToolName:  toolName,
			Arguments: args,
		}),
		Ts: time.Now().UnixMilli(),
	})
	if err != nil {
		return nil, fmt.Errorf("send tool_call: %v", err)
	}

	// 等待结果（长时间工具使用更长超时）
	timeout := time.Duration(b.cfg.ToolCallTimeoutSec) * time.Second
	if isLongRunningTool(toolName) {
		longTimeout := time.Duration(b.cfg.LongToolTimeoutSec) * time.Second
		if longTimeout <= 0 {
			longTimeout = 600 * time.Second
		}
		timeout = longTimeout
	}
	select {
	case result := <-ch:
		if !result.Success {
			return &ToolCallResult{Result: result.Result, AgentID: agentID, FromID: result.FromID},
				fmt.Errorf("tool error: %s", result.Error)
		}
		log.Printf("[Bridge] tool_result ← from=%s tool=%s msgID=%s", result.FromID, toolName, msgID)
		return &ToolCallResult{
			Result:  result.Result,
			AgentID: agentID,
			FromID:  result.FromID,
		}, nil
	case <-time.After(timeout):
		return &ToolCallResult{AgentID: agentID},
			fmt.Errorf("tool_call %s timeout after %v", toolName, timeout)
	case <-ctx.Done():
		return nil, fmt.Errorf("tool_call %s cancelled: %v", toolName, ctx.Err())
	}
}
