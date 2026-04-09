package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
)

// ========================= 工具发现 =========================

// DiscoverTools 从 gateway 获取所有在线 agent 的工具定义
func (b *Bridge) DiscoverTools() error {
	url := fmt.Sprintf("%s/api/gateway/tools", b.cfg.GatewayHTTP)

	resp, err := gatewayHTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %v", err)
	}

	var result struct {
		Success bool              `json:"success"`
		Tools   []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("gateway returned success=false")
	}

	catalog := make(map[string]string)

	// 用于去重，记录已添加的工具以及判断是否需要覆盖（优先保留有参数的）
	type toolEntry struct {
		AgentID   string
		Tool      LLMTool
		HasParams bool
	}
	dedupMap := make(map[string]toolEntry)

	for _, raw := range result.Tools {
		var tool struct {
			AgentID     string          `json:"agent_id"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		}
		if err := json.Unmarshal(raw, &tool); err != nil {
			log.Printf("[Bridge] skip invalid tool: %v", err)
			continue
		}

		// 跳过自身的工具（如果有）
		if tool.AgentID == b.cfg.AgentID {
			continue
		}

		// 构建 LLM 函数名
		llmFuncName := sanitizeToolName(tool.Name)

		params := tool.Parameters
		hasParams := len(params) > 0 && string(params) != `{"type":"object","properties":{}}`
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		newTool := LLMTool{
			Type: "function",
			Function: LLMFunction{
				Name:        llmFuncName,
				Description: tool.Description,
				Parameters:  params,
			},
		}

		// 去重逻辑：如果已经存在同名工具，优先保留有参数的那个
		existing, exists := dedupMap[llmFuncName]
		if !exists || (!existing.HasParams && hasParams) {
			dedupMap[llmFuncName] = toolEntry{
				AgentID:   tool.AgentID,
				Tool:      newTool,
				HasParams: hasParams,
			}
			catalog[tool.Name] = tool.AgentID // 更新 catalog 路由到正确的 Agent
		}
	}

	var llmTools []LLMTool
	var toolNames []string
	agentToolsMap := make(map[string][]LLMTool)
	for name, entry := range dedupMap {
		llmTools = append(llmTools, entry.Tool)
		toolNames = append(toolNames, name)
		agentToolsMap[entry.AgentID] = append(agentToolsMap[entry.AgentID], entry.Tool)
	}

	b.catalogMu.Lock()
	// 比较工具集合是否真正变化（而非仅数量波动）
	prevNames := make(map[string]struct{}, len(b.llmTools))
	for _, t := range b.llmTools {
		prevNames[t.Function.Name] = struct{}{}
	}
	b.toolCatalog = catalog
	b.llmTools = llmTools
	b.agentTools = agentToolsMap

	// 注册远程工具到统一注册表
	for toolName, agentID := range catalog {
		b.registerRemoteToolLocked(toolName, agentID)
	}

	b.catalogMu.Unlock()

	// 仅当工具集合真正变动时才打印日志
	toolsChanged := len(toolNames) != len(prevNames)
	var added, removed []string
	if !toolsChanged {
		for _, name := range toolNames {
			if _, ok := prevNames[name]; !ok {
				toolsChanged = true
				added = append(added, name)
			}
		}
	} else {
		// 计算新增和移除的工具
		newNames := make(map[string]struct{}, len(toolNames))
		for _, name := range toolNames {
			newNames[name] = struct{}{}
			if _, ok := prevNames[name]; !ok {
				added = append(added, name)
			}
		}
		for name := range prevNames {
			if _, ok := newNames[name]; !ok {
				removed = append(removed, name)
			}
		}
	}
	if toolsChanged {
		if len(added) > 0 || len(removed) > 0 {
			log.Printf("[Bridge] tools changed: %d→%d (+%d -%d) added=%v removed=%v",
				len(prevNames), len(llmTools), len(added), len(removed), added, removed)
		} else {
			log.Printf("[Bridge] discovered %d unique tools from %d entries (was %d)",
				len(llmTools), len(result.Tools), len(prevNames))
		}
	}

	// 应用工具权限策略
	b.applyToolPolicy()

	// 合并内置工具（Bash）到 llmTools（用于 LLM function calling）
	// 注意：handler 已在 registerBuiltinTools 中注册到统一注册表
	if b.bashManager != nil {
		b.catalogMu.Lock()
		for _, tool := range b.bashManager.ToolDefs() {
			exists := false
			for _, t := range b.llmTools {
				if t.Function.Name == tool.Function.Name {
					exists = true
					break
				}
			}
			if !exists {
				b.llmTools = append(b.llmTools, tool)
			}
		}
		b.catalogMu.Unlock()
	}

	return nil
}

// applyToolPolicy 根据配置的 allow/deny 列表过滤工具
func (b *Bridge) applyToolPolicy() {
	if b.cfg.ToolPolicy == nil {
		return
	}
	policy := b.cfg.ToolPolicy
	if len(policy.Allow) == 0 && len(policy.Deny) == 0 {
		return
	}

	denySet := make(map[string]bool, len(policy.Deny))
	for _, name := range policy.Deny {
		denySet[name] = true
		denySet[sanitizeToolName(name)] = true
	}
	allowSet := make(map[string]bool, len(policy.Allow))
	for _, name := range policy.Allow {
		allowSet[name] = true
		allowSet[sanitizeToolName(name)] = true
	}

	b.catalogMu.Lock()
	defer b.catalogMu.Unlock()

	var filtered []LLMTool
	var removed []string
	for _, tool := range b.llmTools {
		name := tool.Function.Name
		originalName := name
		if cn, ok := b.toolNameMap[name]; ok {
			originalName = cn
		}

		// deny 优先
		if denySet[name] || denySet[originalName] {
			removed = append(removed, originalName)
			delete(b.toolCatalog, originalName)
			continue
		}
		// allow 非空时，只保留白名单中的
		if len(allowSet) > 0 && !allowSet[name] && !allowSet[originalName] {
			removed = append(removed, originalName)
			delete(b.toolCatalog, originalName)
			continue
		}
		filtered = append(filtered, tool)
	}
	b.llmTools = filtered

	// 同步清理 agentTools
	for agentID, tools := range b.agentTools {
		var agentFiltered []LLMTool
		for _, tool := range tools {
			name := tool.Function.Name
			originalName := name
			if cn, ok := b.toolNameMap[name]; ok {
				originalName = cn
			}
			if denySet[name] || denySet[originalName] {
				continue
			}
			if len(allowSet) > 0 && !allowSet[name] && !allowSet[originalName] {
				continue
			}
			agentFiltered = append(agentFiltered, tool)
		}
		b.agentTools[agentID] = agentFiltered
	}

	if len(removed) > 0 {
		log.Printf("[Bridge] tool policy applied: removed %d tools: %v", len(removed), removed)
	}
}

// DiscoverAgents 从 gateway 获取所有在线 agent 的元数据（含 meta 扩展字段）
func (b *Bridge) DiscoverAgents() error {
	url := fmt.Sprintf("%s/api/gateway/agents", b.cfg.GatewayHTTP)

	resp, err := gatewayHTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %v", err)
	}

	var result struct {
		Success bool `json:"success"`
		Agents  []struct {
			AgentID      string         `json:"agent_id"`
			AgentType    string         `json:"agent_type"`
			Name         string         `json:"name"`
			Description  string         `json:"description"`
			HostPlatform string         `json:"host_platform"`
			HostIP       string         `json:"host_ip"`
			Workspace    string         `json:"workspace"`
			Tools        []string       `json:"tools"`
			Meta         map[string]any `json:"meta"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("gateway returned success=false")
	}

	infoMap := make(map[string]AgentInfo, len(result.Agents))
	for _, a := range result.Agents {
		if a.AgentID == b.cfg.AgentID {
			continue // 跳过自身
		}
		info := AgentInfo{
			ID:           a.AgentID,
			Name:         a.Name,
			Description:  a.Description,
			ToolNames:    a.Tools,
			HostPlatform: a.HostPlatform,
			HostIP:       a.HostIP,
			Workspace:    a.Workspace,
		}
		// 从 meta 提取动态能力信息
		if a.Meta != nil {
			if desc, ok := a.Meta["agent_description"].(string); ok {
				info.DetailDescription = desc
			}
			info.Models = parseStringSlice(a.Meta["models"])
			info.ClaudeCodeModels = parseStringSlice(a.Meta["claudecode_models"])
			info.OpenCodeModels = parseStringSlice(a.Meta["opencode_models"])
			info.CodingTools = parseStringSlice(a.Meta["coding_tools"])
			// 兼容旧 agent：base 字段为空时从 meta 回退
			if info.HostPlatform == "" {
				if hp, ok := a.Meta["host_platform"].(string); ok {
					info.HostPlatform = hp
				}
			}
			info.SSHHosts = parseStringSlice(a.Meta["ssh_hosts"])
			info.DeployTargets = parseStringSlice(a.Meta["deploy_targets"])
			info.TargetHosts = parseStringMap(a.Meta["target_hosts"])
			info.Pipelines = parseStringSlice(a.Meta["pipelines"])
			if pv, ok := a.Meta["python_version"].(string); ok {
				info.PythonVersion = pv
			}
			if met, ok := a.Meta["max_exec_time"].(float64); ok {
				info.MaxExecTime = int(met)
			}
			info.LogSources = parseStringMap(a.Meta["log_sources"])
			info.SupportedSoftware = parseStringSlice(a.Meta["supported_software"])
			if hs, ok := a.Meta["host_stats"].(map[string]interface{}); ok {
				info.HostStats = make(map[string]any, len(hs))
				for k, v := range hs {
					info.HostStats[k] = v
				}
			}
		}
		infoMap[a.AgentID] = info
	}

	b.catalogMu.Lock()
	prevAgentCount := len(b.agentInfo)
	b.agentInfo = infoMap
	b.catalogMu.Unlock()

	if len(infoMap) != prevAgentCount {
		log.Printf("[Bridge] discovered %d agents (was %d)", len(infoMap), prevAgentCount)
		for id, info := range infoMap {
			log.Printf("[Bridge]   agent: %s (%s) tools=%v models=%v coding_tools=%v",
				info.Name, id, info.ToolNames, info.Models, info.CodingTools)
		}
	}
	return nil
}

// parseStringSlice 从 any (interface{}) 解析 []string，兼容 JSON 反序列化的 []interface{}
func parseStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseStringMap 从 any (interface{}) 解析 map[string]string，兼容 JSON 反序列化的 map[string]interface{}
func parseStringMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok {
			result[k] = s
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// truncateToFirstParagraph 截取首段文本（到第一个空行或 maxLen 为止）
func truncateToFirstParagraph(text string, maxLen int) string {
	// 按空行分段
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && sb.Len() > 0 {
			break // 遇到空行，首段结束
		}
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(trimmed)
		if sb.Len() >= maxLen {
			break
		}
	}
	result := sb.String()
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}
	return result
}

// getAgentDescriptionBlock 构建 agent 描述文本用于注入系统提示（含可用模型和工具信息）
func (b *Bridge) getAgentDescriptionBlock() string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	if len(b.agentInfo) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用 Agent 能力\n")

	// 注入 llm-agent 自身信息
	sb.WriteString(fmt.Sprintf("- **%s** (%s): LLM 运行时中枢\n", b.cfg.AgentName, b.cfg.AgentID))
	if b.client.HostPlatform != "" {
		sb.WriteString(fmt.Sprintf("  - 运行平台: %s\n", b.client.HostPlatform))
	}
	if b.client.HostIP != "" {
		sb.WriteString(fmt.Sprintf("  - 主机IP: %s\n", b.client.HostIP))
	}
	if b.client.Workspace != "" {
		sb.WriteString(fmt.Sprintf("  - 工作目录: %s\n", b.client.Workspace))
	}

	for _, info := range b.agentInfo {
		// 标题行：有 description 就显示，没有就只显示名称
		if info.Description != "" {
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", info.Name, info.ID, info.Description))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", info.Name, info.ID))
		}
		// 注入基础信息
		if info.HostPlatform != "" {
			sb.WriteString(fmt.Sprintf("  - 运行平台: %s\n", info.HostPlatform))
		}
		if info.HostIP != "" {
			sb.WriteString(fmt.Sprintf("  - 主机IP: %s\n", info.HostIP))
		}
		if info.Workspace != "" {
			sb.WriteString(fmt.Sprintf("  - 工作目录: %s\n", info.Workspace))
		}
		// 注入可用模型和编码工具信息，让 LLM 知道合法参数值
		if len(info.CodingTools) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用编码工具(tool参数): %s\n", strings.Join(info.CodingTools, ", ")))
		}
		if len(info.Models) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用模型配置(model参数): %s\n", strings.Join(info.Models, ", ")))
		}
		if len(info.SSHHosts) > 0 {
			sb.WriteString(fmt.Sprintf("  - SSH主机: %s\n", strings.Join(info.SSHHosts, ", ")))
		}
		if len(info.DeployTargets) > 0 {
			sb.WriteString(fmt.Sprintf("  - 部署目标(deploy_target参数): %s\n", strings.Join(info.DeployTargets, ", ")))
		}
		if len(info.TargetHosts) > 0 {
			sb.WriteString("  - 部署目标对应SSH地址(ssh_host参数):\n")
			for target, host := range info.TargetHosts {
				sb.WriteString(fmt.Sprintf("    - %s → %s\n", target, host))
			}
		}
		if len(info.Pipelines) > 0 {
			sb.WriteString(fmt.Sprintf("  - Pipeline: %s\n", strings.Join(info.Pipelines, ", ")))
		}
		if info.PythonVersion != "" {
			sb.WriteString(fmt.Sprintf("  - Python版本: %s", info.PythonVersion))
			if info.MaxExecTime > 0 {
				sb.WriteString(fmt.Sprintf(", 执行超时: %ds", info.MaxExecTime))
			}
			sb.WriteString("\n")
		}
		if len(info.LogSources) > 0 {
			sb.WriteString("  - 可查日志源(source参数):\n")
			for name, desc := range info.LogSources {
				sb.WriteString(fmt.Sprintf("    - %s: %s\n", name, desc))
			}
		}
		if len(info.SupportedSoftware) > 0 {
			sb.WriteString(fmt.Sprintf("  - 支持检测/安装的软件(software参数): %s\n", strings.Join(info.SupportedSoftware, ", ")))
		}
		if len(info.HostStats) > 0 {
			var parts []string
			if v, ok := info.HostStats["cpu_cores"]; ok {
				parts = append(parts, fmt.Sprintf("CPU %v核", v))
			}
			if v, ok := info.HostStats["mem_total_gb"]; ok {
				parts = append(parts, fmt.Sprintf("内存 %sGB", v))
			}
			if total, ok := info.HostStats["disk_total_gb"]; ok {
				if free, ok2 := info.HostStats["disk_free_gb"]; ok2 {
					parts = append(parts, fmt.Sprintf("磁盘 %sGB/可用 %sGB", total, free))
				} else {
					parts = append(parts, fmt.Sprintf("磁盘 %sGB", total))
				}
			}
			if len(parts) > 0 {
				sb.WriteString(fmt.Sprintf("  - 主机资源: %s\n", strings.Join(parts, ", ")))
			}
		}
	}
	return sb.String()
}

// executeCodeAgentType execute-code-agent 的类型标识（元工具，始终保留不参与路由筛选）
const executeCodeAgentType = "execute_code"

// isExecuteCodeAgent 判断是否为 execute-code-agent（元工具）
func isExecuteCodeAgent(info AgentInfo) bool {
	// 通过工具名判断（更可靠，不依赖 agent_id 命名）
	for _, name := range info.ToolNames {
		if name == "ExecuteCode" {
			return true
		}
	}
	return false
}

// isFileToolName 判断工具名是否为文件操作工具（始终保留）
func isFileToolName(name string) bool {
	return strings.HasSuffix(name, "ReadFile") ||
		strings.HasSuffix(name, "WriteFile") ||
		strings.HasSuffix(name, "ExecBash")
}

// isFileToolAgent 判断 agent 是否提供文件操作工具
func isFileToolAgent(info AgentInfo) bool {
	for _, name := range info.ToolNames {
		if isFileToolName(name) {
			return true
		}
	}
	return false
}

// getToolsForAgents 从 agentTools 收集指定 agent 的工具
func (b *Bridge) getToolsForAgents(agentIDs []string) []LLMTool {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	idSet := make(map[string]bool, len(agentIDs))
	for _, id := range agentIDs {
		idSet[id] = true
	}

	var tools []LLMTool
	seen := make(map[string]bool)
	for agentID, agentToolList := range b.agentTools {
		if !idSet[agentID] {
			continue
		}
		for _, tool := range agentToolList {
			if !seen[tool.Function.Name] {
				tools = append(tools, tool)
				seen[tool.Function.Name] = true
			}
		}
	}
	return tools
}
