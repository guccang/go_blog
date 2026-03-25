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

// ========================= 渐进式工具发现 =========================

// buildAgentDirectory 构建简要 Agent 目录（固定注入系统提示词）
func (b *Bridge) buildAgentDirectory() string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	if len(b.agentInfo) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用 Agent\n")
	sb.WriteString("需要使用某 agent 的工具时，先调用 get_agent_tools(agent_id) 获取该 agent 的完整工具列表和参数说明。\n\n")

	// llm-agent 自身信息
	sb.WriteString(fmt.Sprintf("- **%s** [%s]: LLM 编排中枢", b.cfg.AgentName, b.cfg.AgentID))
	if b.client.HostPlatform != "" {
		sb.WriteString(fmt.Sprintf(" (平台: %s)", b.client.HostPlatform))
	}
	sb.WriteString("\n")

	for id, info := range b.agentInfo {
		toolCount := len(b.agentTools[id])
		desc := info.Description
		if info.DetailDescription != "" {
			desc = truncateToFirstParagraph(info.DetailDescription, 200)
		}
		if desc == "" {
			desc = info.Name
		}

		// 简要一行：名称 [ID]: 描述 (N个工具) + 关键能力标签
		line := fmt.Sprintf("- **%s** [%s]: %s (%d个工具)", info.Name, id, desc, toolCount)

		// 附加关键能力标签（让 LLM 能快速匹配）
		var tags []string
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
	}
	return sb.String()
}

// getAgentToolDescriptions 获取指定 agent 的格式化工具列表（供 get_agent_tools 返回）
func (b *Bridge) getAgentToolDescriptions(agentID string) string {
	b.catalogMu.RLock()
	agentToolList := b.agentTools[agentID]
	info := b.agentInfo[agentID]
	b.catalogMu.RUnlock()

	if len(agentToolList) == 0 {
		return fmt.Sprintf("Agent %s 没有可用工具。", agentID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%s) 的工具列表\n\n", info.Name, agentID))

	// 详细 agent 信息
	if info.HostPlatform != "" {
		sb.WriteString(fmt.Sprintf("运行平台: %s", info.HostPlatform))
		if info.HostIP != "" {
			sb.WriteString(fmt.Sprintf(" | IP: %s", info.HostIP))
		}
		if info.Workspace != "" {
			sb.WriteString(fmt.Sprintf(" | 目录: %s", info.Workspace))
		}
		sb.WriteString("\n")
	}
	if len(info.DeployTargets) > 0 && len(info.TargetHosts) > 0 {
		sb.WriteString("部署目标:\n")
		for _, target := range info.DeployTargets {
			host := info.TargetHosts[target]
			sb.WriteString(fmt.Sprintf("  - %s → %s\n", target, host))
		}
	}
	if len(info.Models) > 0 {
		sb.WriteString(fmt.Sprintf("可用模型: %s\n", strings.Join(info.Models, ", ")))
	}
	if len(info.CodingTools) > 0 {
		sb.WriteString(fmt.Sprintf("编码工具: %s\n", strings.Join(info.CodingTools, ", ")))
	}
	sb.WriteString("\n")

	for _, t := range agentToolList {
		name := b.resolveToolName(t.Function.Name)
		sb.WriteString(fmt.Sprintf("### %s\n", name))
		sb.WriteString(fmt.Sprintf("%s\n", t.Function.Description))
		paramSummary := extractParamSummary(t.Function.Parameters)
		if paramSummary != "" {
			sb.WriteString(fmt.Sprintf("参数: %s\n", paramSummary))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// getBaseToolSet 返回基础工具集（ExecuteCode、Bash、文件操作等）
func (b *Bridge) getBaseToolSet() []LLMTool {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	var base []LLMTool
	for _, t := range b.llmTools {
		originalName := b.resolveToolNameLocked(t.Function.Name)
		if b.isBaseTool(originalName) {
			base = append(base, t)
		}
	}
	return base
}

// resolveToolNameLocked 在已持有 catalogMu 读锁时解析工具名
func (b *Bridge) resolveToolNameLocked(name string) string {
	if canonical, ok := b.toolNameMap[name]; ok {
		return canonical
	}
	return name
}

// getAgentToolsMap 获取 agentTools 快照
func (b *Bridge) getAgentToolsMap() map[string][]LLMTool {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	m := make(map[string][]LLMTool, len(b.agentTools))
	for k, v := range b.agentTools {
		cp := make([]LLMTool, len(v))
		copy(cp, v)
		m[k] = cp
	}
	return m
}

// resolveAgentByName 模糊匹配 agent（支持 ID、名称、部分匹配）
func (b *Bridge) resolveAgentByName(nameOrID string) string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	nameOrID = strings.ToLower(strings.TrimSpace(nameOrID))

	// 精确 ID 匹配
	if _, ok := b.agentInfo[nameOrID]; ok {
		return nameOrID
	}

	// 部分匹配 ID 或名称
	for id, info := range b.agentInfo {
		if strings.Contains(strings.ToLower(id), nameOrID) ||
			strings.Contains(strings.ToLower(info.Name), nameOrID) {
			return id
		}
	}
	return ""
}

// listAgentNames 列出所有 agent（用于错误提示）
func (b *Bridge) listAgentNames() string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	var lines []string
	for id, info := range b.agentInfo {
		lines = append(lines, fmt.Sprintf("- %s [%s]", info.Name, id))
	}
	return strings.Join(lines, "\n")
}

// injectVirtualTools 集中注入虚拟工具
func (b *Bridge) injectVirtualTools(tools []LLMTool, noTools bool) []LLMTool {
	if noTools {
		return tools
	}
	tools = append(tools, getAgentToolsTool, getSkillDetailTool, planAndExecuteTool)
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
