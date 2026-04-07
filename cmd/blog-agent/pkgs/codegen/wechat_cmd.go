package codegen

import (
	"fmt"
	log "mylog"
	"strings"
)

// AIRouteHandler AI 路由处理器（处理非 cg 命令的微信消息）
// 由外部注入，避免 codegen 直接依赖 llm
var AIRouteHandler func(wechatUser, account, message string) string

// HandleWechatCommand 处理企业微信指令（通过 AI 路由）
func HandleWechatCommand(wechatUser, message string) string {
	account := wechatUser
	if account == "" {
		account = "admin"
	}
	message = normalizeCodegenCommand(message)

	log.MessageF(log.ModuleAgent, "WeChat command from %s (account: %s): %s", wechatUser, account, message)

	// 拦截 cg 命令，直接处理，不经过 LLM
	if strings.HasPrefix(message, "cg ") || message == "cg" {
		return handleCodegenCommand(account, message)
	}

	// 非 cg 命令：交给 AI 路由处理
	if AIRouteHandler != nil {
		return AIRouteHandler(wechatUser, account, message)
	}

	return "⚠️ AI 路由未初始化"
}

func normalizeCodegenCommand(message string) string {
	message = strings.TrimSpace(message)
	if message == "/cg" {
		return "cg"
	}
	if strings.HasPrefix(message, "/cg ") {
		return "cg " + strings.TrimSpace(strings.TrimPrefix(message, "/cg "))
	}
	return message
}

// parseProjectAgent 从 "myapp@win" 解析出 (project, agentName)
func parseProjectAgent(s string) (project, agentName string) {
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

// resolveAgentID 根据 project 和 agentName 解析出目标 agentID
func resolveAgentID(project, agentName, toolFilter string) (string, error) {
	pool := GetAgentPool()
	if pool == nil {
		return "", fmt.Errorf("远程 agent 模式未启用")
	}

	if agentName != "" {
		agent := pool.FindAgentByName(agentName)
		if agent == nil {
			return "", fmt.Errorf("未找到在线 agent: %s", agentName)
		}
		return agent.ID, nil
	}

	remoteProjects := pool.ListRemoteProjects()
	var matched []RemoteProjectInfo
	for _, p := range remoteProjects {
		if p.Name != project {
			continue
		}
		if toolFilter != "" {
			hasTools := false
			for _, t := range p.Tools {
				if t == toolFilter {
					hasTools = true
					break
				}
			}
			if !hasTools {
				continue
			}
		}
		matched = append(matched, p)
	}
	if len(matched) == 1 {
		return matched[0].AgentID, nil
	}
	if len(matched) > 1 {
		var agents []string
		for _, m := range matched {
			agents = append(agents, m.Agent)
		}
		return "", fmt.Errorf("多个 agent 都有项目 %s，请用 %s@<agent> 指定\n可选: %s",
			project, project, strings.Join(agents, ", "))
	}

	return "", nil
}

// handleCodegenCommand 处理 cg 快捷命令
func handleCodegenCommand(userID, message string) string {
	args := strings.TrimPrefix(message, "cg")
	args = strings.TrimSpace(args)

	if args == "" {
		return getCodegenHelpText()
	}

	parts := strings.SplitN(args, " ", 2)
	subCmd := parts[0]
	var param string
	if len(parts) > 1 {
		param = strings.TrimSpace(parts[1])
	}

	switch subCmd {
	case "help", "h":
		return getCodegenHelpText()

	case "list", "ls":
		return handleCgList()

	case "create", "new":
		return handleCgCreate(param)

	case "start", "run":
		return handleCgStart(userID, param)

	case "deploy", "dp":
		return handleCgDeploy(userID, param)

	case "pipeline", "pip":
		return handleCgPipeline(userID, param)

	case "send", "msg":
		if param == "" {
			return "⚠️ 请提供消息内容\n用法: cg send <消息>"
		}
		sessionID, err := SendMessageForWeChat(userID, param)
		if err != nil {
			return fmt.Sprintf("❌ 发送失败: %v", err)
		}
		return fmt.Sprintf("📨 消息已发送到会话 %s", sessionID)

	case "status", "st":
		return GetStatusForWeChat(userID)

	case "stop":
		sessionID, err := StopSessionForWeChat(userID)
		if err != nil {
			return fmt.Sprintf("❌ 停止失败: %v", err)
		}
		return fmt.Sprintf("⏹ 编码会话 %s 已停止", sessionID)

	case "agents":
		return handleCgAgents()

	case "models":
		return handleCgModels()

	case "tools":
		return handleCgTools()

	default:
		return fmt.Sprintf("⚠️ 未知命令: cg %s\n\n%s", subCmd, getCodegenHelpText())
	}
}

func handleCgList() string {
	var remoteProjects []RemoteProjectInfo
	pool := GetAgentPool()
	if pool != nil {
		remoteProjects = pool.ListRemoteProjects()
	}

	if len(remoteProjects) == 0 {
		return "📂 暂无编码项目\n\n请确保远程 agent 已连接并上报项目\n使用 cg create <名称[@agent]> 创建项目"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📂 编码项目 (%d个)\n\n", len(remoteProjects)))
	for i, p := range remoteProjects {
		sb.WriteString(fmt.Sprintf("%d. %s@%s\n", i+1, p.Name, p.Agent))
	}
	return sb.String()
}

func handleCgCreate(param string) string {
	if param == "" {
		return "⚠️ 请指定项目名称\n用法: cg create <名称[@agent]>"
	}
	fields := strings.Fields(param)
	projectName, agentTarget := parseProjectAgent(fields[0])

	if agentTarget == "" {
		for _, p := range fields[1:] {
			if strings.HasPrefix(p, "@") {
				agentTarget = strings.TrimPrefix(p, "@")
			}
		}
	}

	pool := GetAgentPool()
	if pool == nil {
		return "❌ 远程 agent 模式未启用"
	}

	if agentTarget == "" {
		names := pool.GetAgentNames()
		if len(names) == 0 {
			return "❌ 无在线 agent，请先连接 agent 或用 cg create <名称>@<agent名> 指定"
		}
		agentTarget = names[0]
	}

	if err := pool.CreateRemoteProject(agentTarget, projectName); err != nil {
		return fmt.Sprintf("❌ 创建失败: %v", err)
	}
	return fmt.Sprintf("✅ 项目 **%s** 已在 agent **%s** 上创建", projectName, agentTarget)
}

func handleCgStart(userID, param string) string {
	if param == "" {
		return "⚠️ 请指定项目和需求\n用法: cg start <项目[@agent]> [#模型] [@工具] [!deploy] <编码需求>"
	}
	startParts := strings.SplitN(param, " ", 2)
	project, agentName := parseProjectAgent(startParts[0])
	rest := ""
	if len(startParts) > 1 {
		rest = strings.TrimSpace(startParts[1])
	}
	if rest == "" {
		return "⚠️ 请提供编码需求\n用法: cg start <项目[@agent]> [#模型] [@工具] [!deploy] <编码需求>"
	}
	model := ""
	tool := ""
	autoDeploy := false
	for strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, "@") || strings.HasPrefix(rest, "!") {
		optParts := strings.SplitN(rest, " ", 2)
		opt := optParts[0]
		if strings.HasPrefix(opt, "#") {
			model = strings.TrimPrefix(opt, "#")
		} else if strings.HasPrefix(opt, "@") {
			tool = NormalizeTool(strings.TrimPrefix(opt, "@"))
		} else if strings.EqualFold(opt, "!deploy") {
			autoDeploy = true
		}
		if len(optParts) > 1 {
			rest = strings.TrimSpace(optParts[1])
		} else {
			rest = ""
			break
		}
	}
	if rest == "" {
		return "⚠️ 请提供编码需求"
	}
	agentID, err := resolveAgentID(project, agentName, ToolClaudeCode)
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}
	sessionID, err := StartSessionForWeChat(userID, project, rest, model, tool, agentID, autoDeploy)
	if err != nil {
		return fmt.Sprintf("❌ 启动失败: %v", err)
	}
	info := fmt.Sprintf("🚀 编码会话已启动\n\n项目: %s", project)
	if agentName != "" {
		info += fmt.Sprintf("\nAgent: %s", agentName)
	}
	if model != "" {
		info += fmt.Sprintf("\n模型: %s", model)
	}
	if tool != "" && tool != "claudecode" {
		info += fmt.Sprintf("\n工具: %s", tool)
	}
	if autoDeploy {
		info += "\n部署: 编码完成后自动部署"
	}
	info += fmt.Sprintf("\n会话: %s\n\n进度将通过当前客户端推送", sessionID)
	return info
}

func handleCgDeploy(userID, param string) string {
	if param == "" {
		return "⚠️ 请指定项目名称\n用法: cg deploy <项目[@agent]> [#目标] [!pack]"
	}
	deployParts := strings.SplitN(param, " ", 2)
	project, agentName := parseProjectAgent(deployParts[0])
	rest := ""
	if len(deployParts) > 1 {
		rest = strings.TrimSpace(deployParts[1])
	}
	deployTarget := ""
	packOnly := false
	for rest != "" && (strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, "!")) {
		optParts := strings.SplitN(rest, " ", 2)
		opt := optParts[0]
		if strings.HasPrefix(opt, "#") {
			deployTarget = strings.TrimPrefix(opt, "#")
		} else if strings.EqualFold(opt, "!pack") {
			packOnly = true
		}
		if len(optParts) > 1 {
			rest = strings.TrimSpace(optParts[1])
		} else {
			rest = ""
		}
	}
	agentID, err := resolveAgentID(project, agentName, ToolDeploy)
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}
	sessionID, err := StartDeployForWeChat(userID, project, agentID, deployTarget, packOnly)
	if err != nil {
		return fmt.Sprintf("❌ 部署启动失败: %v", err)
	}
	info := fmt.Sprintf("🚀 部署已启动\n\n项目: %s", project)
	if agentName != "" {
		info += fmt.Sprintf("\nAgent: %s", agentName)
	}
	if deployTarget != "" {
		info += fmt.Sprintf("\n目标: %s", deployTarget)
	}
	if packOnly {
		info += "\n模式: 仅打包"
	}
	info += fmt.Sprintf("\n会话: %s\n\n进度将通过当前客户端推送", sessionID)
	return info
}

func handleCgPipeline(userID, param string) string {
	if param == "" || param == "list" || param == "ls" {
		pool := GetAgentPool()
		if pool == nil {
			return "❌ 远程 agent 模式未启用"
		}
		pipelines := pool.ListPipelines()
		if len(pipelines) == 0 {
			return "暂无可用 pipeline（deploy agent 未上报或未在线）"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📋 可用 Pipeline (%d个)\n\n", len(pipelines)))
		for _, p := range pipelines {
			sb.WriteString(fmt.Sprintf("  🔄 %s (agent: %s)\n", p.Name, p.Agent))
		}
		sb.WriteString("\n用法: cg pipeline <名称[@agent]>")
		return sb.String()
	}
	pipelineName, agentName := parseProjectAgent(strings.Fields(param)[0])
	agentID, err := resolveAgentID(pipelineName, agentName, ToolDeploy)
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}
	if agentID == "" {
		pool := GetAgentPool()
		if pool != nil {
			for _, p := range pool.ListRemoteProjects() {
				for _, t := range p.Tools {
					if t == ToolDeploy {
						agentID = p.AgentID
						break
					}
				}
				if agentID != "" {
					break
				}
			}
		}
	}
	if agentID == "" {
		return "❌ 未找到可用的 deploy agent"
	}
	sessionID, err := StartPipelineForWeChat(userID, pipelineName, agentID)
	if err != nil {
		return fmt.Sprintf("❌ Pipeline 启动失败: %v", err)
	}
	info := fmt.Sprintf("🔄 Pipeline 已启动\n\n编排: %s", pipelineName)
	if agentName != "" {
		info += fmt.Sprintf(" (agent: %s)", agentName)
	}
	info += fmt.Sprintf("\n会话: %s\n\n进度将通过当前客户端推送", sessionID)
	return info
}

func handleCgAgents() string {
	pool := GetAgentPool()
	if pool == nil {
		return "远程 agent 模式未启用"
	}
	agents := pool.GetAgents()
	if len(agents) == 0 {
		return "当前无在线 agent"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🖥 在线 Agent (%d个)\n\n", len(agents)))
	for i, a := range agents {
		name, _ := a["name"].(string)
		agentType, _ := a["agent_type"].(string)
		status, _ := a["status"].(string)
		if status == "" {
			status = "online"
		}
		typeLabel := ""
		if agentType != "" {
			typeLabel = fmt.Sprintf(" (%s)", agentType)
		}
		sb.WriteString(fmt.Sprintf("%d. **%s**%s [%s]\n",
			i+1, name, typeLabel, status))
	}
	return sb.String()
}

func handleCgModels() string {
	pool := GetAgentPool()
	if pool == nil {
		return "远程 agent 模式未启用"
	}
	models := pool.GetAllModels()
	if len(models) == 0 {
		return "当前无可用模型配置"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🤖 可用模型配置 (%d个)\n\n", len(models)))
	for i, m := range models {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, m))
	}
	sb.WriteString("\n用法: cg start <项目> #模型名 <需求>")
	return sb.String()
}

func handleCgTools() string {
	pool := GetAgentPool()
	if pool == nil {
		return "远程 agent 模式未启用"
	}
	tools := pool.GetAllTools()
	if len(tools) == 0 {
		return "当前无可用编码工具"
	}
	toolLabels := map[string]string{
		"claudecode": "Claude Code (默认)",
		"opencode":   "OpenCode",
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔧 可用编码工具 (%d个)\n\n", len(tools)))
	for i, t := range tools {
		label := toolLabels[t]
		if label == "" {
			label = t
		}
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, label))
	}
	sb.WriteString("\n用法: cg start <项目> @oc <需求>")
	sb.WriteString("\n别名: @oc/@opencode=OpenCode, @cc/@claude=ClaudeCode")
	return sb.String()
}

func getCodegenHelpText() string {
	return "💻 CodeGen 编码助手命令\n\n" +
		"cg list — 列出所有项目\n" +
		"cg create <名称[@agent]> — 创建项目\n" +
		"cg start <项目[@agent]> <需求> — 启动编码\n" +
		"cg start <项目[@agent]> #<模型> <需求> — 指定模型\n" +
		"cg start <项目[@agent]> @oc <需求> — 用OpenCode\n" +
		"cg start <项目[@agent]> !deploy <需求> — 编码后自动部署\n" +
		"cg deploy <项目[@agent]> — 仅部署（不编码）\n" +
		"cg pipeline list — 列出可用编排\n" +
		"cg pipeline <编排名[@agent]> — 执行部署编排\n" +
		"cg send <消息> — 追加指令\n" +
		"cg status — 查看进度\n" +
		"cg stop — 停止编码\n" +
		"cg models — 查看可用模型配置\n" +
		"cg tools — 查看可用编码工具\n" +
		"cg agents — 查看在线agent\n\n" +
		"@agent 语法: 多agent同名项目时用 项目@agent 指定目标\n" +
		"工具别名: @oc/@opencode=OpenCode, @cc/@claude=ClaudeCode\n" +
		"示例: cg start myapp@win #sonnet !deploy 写个HTTP服务"
}
