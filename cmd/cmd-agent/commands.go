package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type commandRequest struct {
	SourceAgentID string
	Channel       string
	UserID        string
	Content       string
}

func (a *CMDAGent) dispatchCommand(req commandRequest) error {
	args := strings.TrimSpace(strings.TrimPrefix(req.Content, "cg"))
	if args == "" {
		return a.sendClientNotify(req.route(), getCodegenHelpText())
	}

	parts := strings.SplitN(args, " ", 2)
	subCmd := parts[0]
	param := ""
	if len(parts) > 1 {
		param = strings.TrimSpace(parts[1])
	}

	switch subCmd {
	case "help", "h":
		return a.sendClientNotify(req.route(), getCodegenHelpText())
	case "agents":
		return a.handleCgAgents(req)
	case "list", "ls":
		return a.handleCgList(req)
	case "models":
		return a.handleCgModels(req)
	case "tools":
		return a.handleCgTools(req)
	case "create", "new":
		return a.handleCgCreate(req, param)
	case "start", "run":
		return a.handleCgStart(req, param)
	case "send", "msg":
		return a.handleCgSend(req, param)
	case "status", "st":
		return a.handleCgStatus(req)
	case "stop":
		return a.handleCgStop(req)
	case "deploy", "dp":
		return a.handleCgDeploy(req, param)
	case "pipeline", "pip":
		return a.handleCgPipeline(req, param)
	default:
		return a.sendClientNotify(req.route(), fmt.Sprintf("⚠️ 未知命令: cg %s\n\n%s", subCmd, getCodegenHelpText()))
	}
}

func (r commandRequest) route() sessionRoute {
	return sessionRoute{
		SourceAgentID: r.SourceAgentID,
		Channel:       r.Channel,
		UserID:        r.UserID,
	}
}

func (a *CMDAGent) handleCgAgents(req commandRequest) error {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return a.sendClientNotify(req.route(), "当前无在线 agent")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🖥 在线 Agent (%d个)\n\n", len(agents)))
	for i, agent := range agents {
		status := "online"
		typeLabel := ""
		if strings.TrimSpace(agent.AgentType) != "" {
			typeLabel = fmt.Sprintf(" (%s)", agent.AgentType)
		}
		sb.WriteString(fmt.Sprintf("%d. **%s**%s [%s]\n", i+1, agent.Name, typeLabel, status))
	}
	return a.sendClientNotify(req.route(), strings.TrimSpace(sb.String()))
}

func (a *CMDAGent) handleCgList(req commandRequest) error {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return err
	}

	type item struct {
		project string
		agent   string
	}
	var items []item
	onlineCodingAgents := 0
	for _, agent := range agents {
		if !supportsCodingAgent(agent) {
			continue
		}
		onlineCodingAgents++
		projects := a.availableProjectsForAgent(agent)
		for _, project := range projects {
			items = append(items, item{project: project, agent: agent.Name})
		}
	}
	if len(items) == 0 {
		if onlineCodingAgents > 0 {
			return a.sendClientNotify(req.route(), "📂 当前 acp-agent 已在线，但还没有编码项目\n\n使用 cg create <名称[@agent]> 创建项目")
		}
		return a.sendClientNotify(req.route(), "📂 暂无编码项目\n\n请确保 acp-agent 已连接并上报项目\n使用 cg create <名称[@agent]> 创建项目")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📂 编码项目 (%d个)\n\n", len(items)))
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. %s@%s\n", i+1, item.project, item.agent))
	}
	return a.sendClientNotify(req.route(), strings.TrimSpace(sb.String()))
}

func (a *CMDAGent) availableProjectsForAgent(agent gatewayAgentSnapshot) []string {
	projects := projectNamesFromMeta(agent.Meta)
	if len(projects) == 0 {
		projects = a.fetchProjectsFromAgent(agent)
	}
	return uniqueSorted(projects)
}

func (a *CMDAGent) fetchProjectsFromAgent(agent gatewayAgentSnapshot) []string {
	toolName := ""
	switch {
	case hasTool(agent, "AcpListProjects"):
		toolName = "AcpListProjects"
	case hasTool(agent, "CodegenListProjects"):
		toolName = "CodegenListProjects"
	default:
		return nil
	}

	requestID := "cmd_list_probe_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(agent.AgentID, requestID, toolName, map[string]any{})
	if err != nil {
		return nil
	}
	result := <-resultCh
	if !result.Success {
		return nil
	}

	var payload struct {
		Projects []struct {
			Name string `json:"name"`
		} `json:"projects"`
	}
	if err := json.Unmarshal([]byte(result.Result), &payload); err != nil {
		return nil
	}
	var names []string
	for _, item := range payload.Projects {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		names = append(names, item.Name)
	}
	return uniqueSorted(names)
}

func (a *CMDAGent) handleCgModels(req commandRequest) error {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return err
	}
	var models []string
	for _, agent := range agents {
		if !supportsCodingAgent(agent) {
			continue
		}
		models = append(models, modelNamesFromMeta(agent.Meta)...)
	}
	models = uniqueSorted(models)
	if len(models) == 0 {
		return a.sendClientNotify(req.route(), "当前无可用模型配置")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🤖 可用模型配置 (%d个)\n\n", len(models)))
	for i, model := range models {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, model))
	}
	sb.WriteString("\n用法: cg start <项目> #模型名 <需求>")
	return a.sendClientNotify(req.route(), strings.TrimSpace(sb.String()))
}

func (a *CMDAGent) handleCgTools(req commandRequest) error {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return err
	}
	var tools []string
	for _, agent := range agents {
		if !supportsCodingAgent(agent) {
			continue
		}
		tools = append(tools, codingToolsFromMeta(agent.Meta)...)
		if hasTool(agent, "AcpStartSession") {
			tools = append(tools, "acp")
		}
	}
	tools = uniqueSorted(tools)
	if len(tools) == 0 {
		return a.sendClientNotify(req.route(), "当前无可用编码工具")
	}

	labels := map[string]string{
		"claudecode": "Claude Code (默认)",
		"opencode":   "OpenCode",
		"acp":        "ACP / Claude Agent",
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔧 可用编码工具 (%d个)\n\n", len(tools)))
	for i, tool := range tools {
		label := labels[tool]
		if label == "" {
			label = tool
		}
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, label))
	}
	sb.WriteString("\n用法: cg start <项目> @oc <需求>")
	sb.WriteString("\n别名: @oc/@opencode=OpenCode, @cc/@claude=ClaudeCode")
	return a.sendClientNotify(req.route(), strings.TrimSpace(sb.String()))
}

func (a *CMDAGent) handleCgCreate(req commandRequest, param string) error {
	if strings.TrimSpace(param) == "" {
		return a.sendClientNotify(req.route(), "⚠️ 请指定项目名称\n用法: cg create <名称[@agent]>")
	}

	fields := strings.Fields(param)
	projectName, agentName := parseProjectAgent(fields[0])
	if agentName == "" {
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "@") {
				agentName = strings.TrimPrefix(field, "@")
				break
			}
		}
	}

	agent, err := a.resolveCodegenCreateAgent(projectName, agentName)
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}

	requestID := "cmd_create_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(agent.AgentID, requestID, createProjectToolName(agent), map[string]any{"name": projectName})
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 创建失败: "+result.Error)
	}
	return a.sendClientNotify(req.route(), fmt.Sprintf("✅ 项目 **%s** 已在 agent **%s** 上创建", projectName, agent.Name))
}

func (a *CMDAGent) handleCgStart(req commandRequest, param string) error {
	if strings.TrimSpace(param) == "" {
		return a.sendClientNotify(req.route(), "⚠️ 请指定项目和需求\n用法: cg start <项目[@agent]> [#模型] [@工具] [!deploy] <编码需求>")
	}

	startParts := strings.SplitN(param, " ", 2)
	project, agentName := parseProjectAgent(startParts[0])
	rest := ""
	if len(startParts) > 1 {
		rest = strings.TrimSpace(startParts[1])
	}
	if rest == "" {
		return a.sendClientNotify(req.route(), "⚠️ 请提供编码需求\n用法: cg start <项目[@agent]> [#模型] [@工具] [!deploy] <编码需求>")
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
			tool = normalizeTool(strings.TrimPrefix(opt, "@"))
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
		return a.sendClientNotify(req.route(), "⚠️ 请提供编码需求")
	}

	agent, err := a.resolveCodingAgent(project, agentName, false)
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}

	requestID := "cmd_start_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	route := sessionRoute{
		SourceAgentID: req.SourceAgentID,
		Channel:       req.Channel,
		UserID:        req.UserID,
		TargetAgentID: agent.AgentID,
		Project:       project,
		AutoDeploy:    autoDeploy,
	}
	a.setPendingRoute(requestID, route)

	if err := a.sendClientNotify(route, buildStartInfo(project, agentNameOrDefault(agentName, agent.Name), model, tool, autoDeploy, requestID)); err != nil {
		return err
	}

	args, toolName := buildCodingStartCall(agent, a.cfg.AgentID, project, rest, model, tool)
	route.Kind = codingBackendKind(toolName)
	resultCh, err := a.callTool(agent.AgentID, requestID, toolName, args)
	if err != nil {
		return err
	}
	go a.awaitCodingStartResult(route, requestID, toolName, resultCh)
	return nil
}

func (a *CMDAGent) awaitCodingStartResult(route sessionRoute, requestID, toolName string, resultCh <-chan toolCallResult) {
	result := <-resultCh
	if !result.Success {
		_ = a.sendClientNotify(route, "❌ 启动失败: "+result.Error)
		_ = a.sendTaskComplete(route, requestID, "error", result.Error)
		return
	}

	var data codegenToolResult
	if err := json.Unmarshal([]byte(result.Result), &data); err != nil {
		_ = a.sendClientNotify(route, "❌ 编码结果解析失败: "+err.Error())
		return
	}
	if data.SessionID != "" {
		a.associateSessionRoute(data.SessionID, route)
		a.rememberUserCodegenSession(route.UserID, userCodegenSession{
			SessionID:     data.SessionID,
			TargetAgentID: route.TargetAgentID,
			Project:       route.Project,
			Backend:       codingBackendKind(toolName),
		})
	}
	_ = a.sendTaskComplete(route, data.SessionID, "done", "")
	if route.AutoDeploy {
		go a.startAutoDeploy(route, data)
	}
}

func (a *CMDAGent) handleCgSend(req commandRequest, param string) error {
	if strings.TrimSpace(param) == "" {
		return a.sendClientNotify(req.route(), "⚠️ 请提供消息内容\n用法: cg send <消息>")
	}
	last, ok := a.getUserCodegenSession(req.UserID)
	if !ok {
		return a.sendClientNotify(req.route(), "❌ 没有活跃的编码会话，请先启动一个会话")
	}
	if last.Backend == "acp" {
		return a.sendClientNotify(req.route(), "❌ acp-agent 已移除 AcpSendMessage，当前不支持 cg send")
	}

	requestID := "cmd_send_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	route := sessionRoute{
		SourceAgentID: req.SourceAgentID,
		Channel:       req.Channel,
		UserID:        req.UserID,
		TargetAgentID: last.TargetAgentID,
		Project:       last.Project,
		Kind:          last.Backend,
	}
	a.setPendingRoute(requestID, route)

	if err := a.sendClientNotify(route, fmt.Sprintf("📨 消息已发送到会话 %s", last.SessionID)); err != nil {
		return err
	}

	sendArgs := map[string]any{
		"prompt":     param,
		"session_id": last.SessionID,
	}
	if last.Backend == "acp" {
		sendArgs["caller_agent_id"] = a.cfg.AgentID
		sendArgs["keep_session"] = true
	}
	resultCh, err := a.callTool(last.TargetAgentID, requestID, sendToolName(last.Backend), sendArgs)
	if err != nil {
		return err
	}
	go func() {
		result := <-resultCh
		if !result.Success {
			_ = a.sendClientNotify(route, "❌ 发送失败: "+result.Error)
			_ = a.sendTaskComplete(route, last.SessionID, "error", result.Error)
			return
		}
		var data codegenToolResult
		if err := json.Unmarshal([]byte(result.Result), &data); err != nil {
			return
		}
		if data.SessionID != "" {
			a.associateSessionRoute(data.SessionID, route)
			a.rememberUserCodegenSession(route.UserID, userCodegenSession{
				SessionID:     data.SessionID,
				TargetAgentID: route.TargetAgentID,
				Project:       route.Project,
				Backend:       last.Backend,
			})
		}
		_ = a.sendTaskComplete(route, firstNonEmpty(data.SessionID, last.SessionID), "done", "")
	}()
	return nil
}

func (a *CMDAGent) handleCgStatus(req commandRequest) error {
	last, ok := a.getUserCodegenSession(req.UserID)
	if !ok {
		return a.sendClientNotify(req.route(), "当前没有活跃的编码会话")
	}
	if last.Backend == "acp" {
		return a.sendClientNotify(req.route(), "❌ acp-agent 已移除 AcpGetStatus，当前不支持 cg status")
	}

	requestID := "cmd_status_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(last.TargetAgentID, requestID, statusToolName(last.Backend), map[string]any{
		"session_id": last.SessionID,
	})
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 查询失败: "+result.Error)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(result.Result), &data); err != nil {
		return a.sendClientNotify(req.route(), result.Result)
	}
	status, _ := data["status"].(string)
	project, _ := data["project"].(string)
	summary, _ := data["summary"].(string)
	text := fmt.Sprintf("项目: %s\n状态: %s\n会话ID: %s", project, status, last.SessionID)
	if strings.TrimSpace(summary) != "" {
		text += "\n摘要: " + summary
	}
	return a.sendClientNotify(req.route(), text)
}

func (a *CMDAGent) handleCgStop(req commandRequest) error {
	last, ok := a.getUserCodegenSession(req.UserID)
	if !ok {
		return a.sendClientNotify(req.route(), "❌ 当前没有运行中的编码会话")
	}

	requestID := "cmd_stop_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(last.TargetAgentID, requestID, stopToolName(last.Backend), map[string]any{
		"session_id": last.SessionID,
	})
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 停止失败: "+result.Error)
	}
	return a.sendClientNotify(req.route(), fmt.Sprintf("⏹ 编码会话 %s 已停止", last.SessionID))
}

func (a *CMDAGent) handleCgDeploy(req commandRequest, param string) error {
	if strings.TrimSpace(param) == "" {
		return a.sendClientNotify(req.route(), "⚠️ 请指定 deploy 子命令或项目\n用法: cg deploy <项目[@agent]> [#目标] [!pack]")
	}
	fields := strings.Fields(param)
	switch fields[0] {
	case "list", "ls":
		return a.handleCgDeployList(req)
	case "status":
		return a.handleCgDeployStatus(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "adhoc":
		return a.handleCgDeployAdhoc(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "pipelines":
		return a.handleCgDeployPipelines(req)
	case "pipeline":
		return a.handleCgPipeline(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "read":
		return a.handleCgDeployRead(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "write":
		return a.handleCgDeployWrite(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "exec":
		return a.handleCgDeployExec(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "env":
		return a.handleCgDeployEnv(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "agent-status":
		return a.handleCgDeployAgentStatus(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
	case "agent-stop", "agent-shutdown":
		return a.handleCgDeployAgentStop(req, strings.TrimSpace(strings.TrimPrefix(param, fields[0])))
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

	agent, err := a.resolveDeployAgent(project, agentName)
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}

	requestID := "cmd_deploy_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	route := sessionRoute{
		SourceAgentID: req.SourceAgentID,
		Channel:       req.Channel,
		UserID:        req.UserID,
		TargetAgentID: agent.AgentID,
		Project:       project,
		Kind:          "deploy",
	}
	a.setPendingRoute(requestID, route)

	if err := a.sendClientNotify(route, buildDeployInfo(project, agentNameOrDefault(agentName, agent.Name), deployTarget, packOnly, requestID)); err != nil {
		return err
	}

	resultCh, err := a.callTool(agent.AgentID, requestID, "DeployProject", map[string]any{
		"project":       project,
		"deploy_target": deployTarget,
		"pack_only":     packOnly,
	})
	if err != nil {
		return err
	}
	go a.awaitDeployResult(route, requestID, resultCh)
	return nil
}

func (a *CMDAGent) handleCgDeployList(req commandRequest) error {
	agent, err := a.resolveAnyDeployAgent("")
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	requestID := "cmd_deploy_list_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(agent.AgentID, requestID, "DeployListProjects", map[string]any{})
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 查询失败: "+result.Error)
	}
	var payload struct {
		Projects []struct {
			Name       string   `json:"name"`
			Configured bool     `json:"configured"`
			BuildOnly  bool     `json:"build_only"`
			ProjectDir string   `json:"project_dir"`
			Targets    []string `json:"targets"`
		} `json:"projects"`
	}
	if err := json.Unmarshal([]byte(result.Result), &payload); err != nil {
		return a.sendClientNotify(req.route(), result.Result)
	}
	if len(payload.Projects) == 0 {
		return a.sendClientNotify(req.route(), "当前无可用 deploy 项目")
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🚚 Deploy 项目 (%d个)\n\n", len(payload.Projects)))
	for i, item := range payload.Projects {
		mode := "adhoc"
		if item.Configured {
			mode = "configured"
		}
		sb.WriteString(fmt.Sprintf("%d. %s [%s]", i+1, item.Name, mode))
		if item.BuildOnly {
			sb.WriteString(" build-only")
		}
		if len(item.Targets) > 0 {
			sb.WriteString(fmt.Sprintf(" targets=%s", strings.Join(item.Targets, ",")))
		}
		sb.WriteString("\n")
	}
	return a.sendClientNotify(req.route(), strings.TrimSpace(sb.String()))
}

func (a *CMDAGent) handleCgDeployStatus(req commandRequest, param string) error {
	fields := strings.Fields(param)
	if len(fields) == 0 {
		return a.sendClientNotify(req.route(), "⚠️ 请提供 session_id\n用法: cg deploy status <session_id>")
	}
	agent, err := a.resolveAnyDeployAgent("")
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	requestID := "cmd_deploy_status_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(agent.AgentID, requestID, "DeployGetStatus", map[string]any{"session_id": fields[0]})
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 查询失败: "+result.Error)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(result.Result), &data); err != nil {
		return a.sendClientNotify(req.route(), result.Result)
	}
	text := fmt.Sprintf("session: %v\nstatus: %v", data["session_id"], data["status"])
	if v, ok := data["tool_name"]; ok && fmt.Sprint(v) != "" {
		text += "\ntool: " + fmt.Sprint(v)
	}
	if v, ok := data["project"]; ok && fmt.Sprint(v) != "" {
		text += "\nproject: " + fmt.Sprint(v)
	}
	if v, ok := data["pipeline"]; ok && fmt.Sprint(v) != "" {
		text += "\npipeline: " + fmt.Sprint(v)
	}
	if v, ok := data["error"]; ok && fmt.Sprint(v) != "" {
		text += "\nerror: " + fmt.Sprint(v)
	}
	return a.sendClientNotify(req.route(), text)
}

func (a *CMDAGent) handleCgDeployAdhoc(req commandRequest, param string) error {
	fields := strings.Fields(param)
	if len(fields) < 3 {
		return a.sendClientNotify(req.route(), "⚠️ 参数不足\n用法: cg deploy adhoc <项目> <目录> <ssh_host>")
	}
	agent, err := a.resolveAnyDeployAgent("")
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	project := fields[0]
	projectDir := fields[1]
	sshHost := fields[2]
	requestID := "cmd_deploy_adhoc_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(agent.AgentID, requestID, "DeployAdhoc", map[string]any{
		"project":     project,
		"project_dir": projectDir,
		"ssh_host":    sshHost,
	})
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 提交失败: "+result.Error)
	}
	return a.sendClientNotify(req.route(), result.Result)
}

func (a *CMDAGent) handleCgDeployPipelines(req commandRequest) error {
	agent, err := a.resolveAnyDeployAgent("")
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	requestID := "cmd_deploy_pipelines_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(agent.AgentID, requestID, "DeployListPipelines", map[string]any{})
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 查询失败: "+result.Error)
	}
	var payload []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Steps       int    `json:"steps"`
	}
	if err := json.Unmarshal([]byte(result.Result), &payload); err != nil {
		return a.sendClientNotify(req.route(), result.Result)
	}
	if len(payload) == 0 {
		return a.sendClientNotify(req.route(), "暂无可用 deploy pipeline")
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Deploy Pipelines (%d个)\n\n", len(payload)))
	for i, item := range payload {
		sb.WriteString(fmt.Sprintf("%d. %s (%d steps)", i+1, item.Name, item.Steps))
		if strings.TrimSpace(item.Description) != "" {
			sb.WriteString(" - " + item.Description)
		}
		sb.WriteString("\n")
	}
	return a.sendClientNotify(req.route(), strings.TrimSpace(sb.String()))
}

func (a *CMDAGent) handleCgDeployRead(req commandRequest, param string) error {
	fields := strings.Fields(param)
	if len(fields) < 2 {
		return a.sendClientNotify(req.route(), "⚠️ 参数不足\n用法: cg deploy read <项目[@agent]> <路径>")
	}
	project, agentName := parseProjectAgent(fields[0])
	agent, err := a.resolveDeployProjectAgent(project, agentName)
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	return a.forwardSimpleDeployTool(req, agent, "DeployReadFile", map[string]any{
		"project": project,
		"path":    fields[1],
	})
}

func (a *CMDAGent) handleCgDeployWrite(req commandRequest, param string) error {
	fields := strings.Fields(param)
	if len(fields) < 3 {
		return a.sendClientNotify(req.route(), "⚠️ 参数不足\n用法: cg deploy write <项目[@agent]> <路径> <内容>")
	}
	project, agentName := parseProjectAgent(fields[0])
	agent, err := a.resolveDeployProjectAgent(project, agentName)
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	content := strings.TrimSpace(strings.TrimPrefix(param, fields[0]+" "+fields[1]))
	return a.forwardSimpleDeployTool(req, agent, "DeployWriteFile", map[string]any{
		"project": project,
		"path":    fields[1],
		"content": content,
	})
}

func (a *CMDAGent) handleCgDeployExec(req commandRequest, param string) error {
	fields := strings.Fields(param)
	if len(fields) < 2 {
		return a.sendClientNotify(req.route(), "⚠️ 参数不足\n用法: cg deploy exec <项目[@agent]> <命令>")
	}
	project, agentName := parseProjectAgent(fields[0])
	agent, err := a.resolveDeployProjectAgent(project, agentName)
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	command := strings.TrimSpace(strings.TrimPrefix(param, fields[0]))
	return a.forwardSimpleDeployTool(req, agent, "DeployExecBash", map[string]any{
		"project": project,
		"command": command,
	})
}

func (a *CMDAGent) handleCgDeployEnv(req commandRequest, param string) error {
	command := strings.TrimSpace(param)
	if command == "" {
		return a.sendClientNotify(req.route(), "⚠️ 请提供命令\n用法: cg deploy env <命令>")
	}
	agent, err := a.resolveAnyDeployAgent("")
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	return a.forwardSimpleDeployTool(req, agent, "DeployExecEnvBash", map[string]any{
		"command": command,
	})
}

func (a *CMDAGent) handleCgDeployAgentStatus(req commandRequest, param string) error {
	agentID := strings.TrimSpace(param)
	if agentID == "" {
		return a.sendClientNotify(req.route(), "⚠️ 请提供 agent_id\n用法: cg deploy agent-status <agent_id>")
	}
	agent, err := a.resolveAnyDeployAgent("")
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	return a.forwardSimpleDeployTool(req, agent, "AgentStatus", map[string]any{"agent_id": agentID})
}

func (a *CMDAGent) handleCgDeployAgentStop(req commandRequest, param string) error {
	fields := strings.Fields(param)
	if len(fields) == 0 {
		return a.sendClientNotify(req.route(), "⚠️ 请提供 agent_id\n用法: cg deploy agent-stop <agent_id> [原因]")
	}
	agent, err := a.resolveAnyDeployAgent("")
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}
	reason := ""
	if len(fields) > 1 {
		reason = strings.TrimSpace(strings.TrimPrefix(param, fields[0]))
	}
	return a.forwardSimpleDeployTool(req, agent, "AgentShutdown", map[string]any{
		"agent_id": fields[0],
		"reason":   reason,
	})
}

func (a *CMDAGent) forwardSimpleDeployTool(req commandRequest, agent gatewayAgentSnapshot, toolName string, args map[string]any) error {
	requestID := "cmd_deploy_tool_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	resultCh, err := a.callTool(agent.AgentID, requestID, toolName, args)
	if err != nil {
		return err
	}
	result := <-resultCh
	if !result.Success {
		return a.sendClientNotify(req.route(), "❌ 调用失败: "+result.Error)
	}
	return a.sendClientNotify(req.route(), result.Result)
}

func (a *CMDAGent) awaitDeployResult(route sessionRoute, requestID string, resultCh <-chan toolCallResult) {
	result := <-resultCh
	if !result.Success {
		_ = a.sendClientNotify(route, "❌ 部署启动失败: "+result.Error)
		return
	}
	var data deployAcceptedResult
	if err := json.Unmarshal([]byte(result.Result), &data); err != nil {
		_ = a.sendClientNotify(route, "❌ 部署结果解析失败: "+err.Error())
		return
	}
	if data.SessionID != "" {
		a.associateSessionRoute(data.SessionID, route)
	}
}

func (a *CMDAGent) handleCgPipeline(req commandRequest, param string) error {
	if strings.TrimSpace(param) == "" || param == "list" || param == "ls" {
		return a.handleCgPipelineList(req)
	}

	pipelineName, agentName := parseProjectAgent(strings.Fields(param)[0])
	agent, err := a.resolvePipelineAgent(pipelineName, agentName)
	if err != nil {
		return a.sendClientNotify(req.route(), "❌ "+err.Error())
	}

	requestID := "cmd_pipeline_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	route := sessionRoute{
		SourceAgentID: req.SourceAgentID,
		Channel:       req.Channel,
		UserID:        req.UserID,
		TargetAgentID: agent.AgentID,
		Project:       pipelineName,
		Kind:          "pipeline",
	}
	a.setPendingRoute(requestID, route)

	info := fmt.Sprintf("🔄 Pipeline 已启动\n\n编排: %s", pipelineName)
	if agentName != "" {
		info += fmt.Sprintf("\nAgent: %s", agentName)
	}
	info += fmt.Sprintf("\n请求: %s\n\n进度将通过当前客户端推送", requestID)
	if err := a.sendClientNotify(route, info); err != nil {
		return err
	}

	resultCh, err := a.callTool(agent.AgentID, requestID, "DeployPipeline", map[string]any{
		"pipeline": pipelineName,
	})
	if err != nil {
		return err
	}
	go a.awaitDeployResult(route, requestID, resultCh)
	return nil
}

func (a *CMDAGent) handleCgPipelineList(req commandRequest) error {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return err
	}
	type pipelineItem struct {
		name  string
		agent string
	}
	var items []pipelineItem
	for _, agent := range agents {
		if !hasTool(agent, "DeployPipeline") {
			continue
		}
		for _, name := range stringSliceFromAny(agent.Meta["pipelines"]) {
			items = append(items, pipelineItem{name: name, agent: agent.Name})
		}
	}
	if len(items) == 0 {
		return a.sendClientNotify(req.route(), "暂无可用 pipeline（deploy-agent 未上报或未在线）")
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 可用 Pipeline (%d个)\n\n", len(items)))
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("  🔄 %s (agent: %s)\n", item.name, item.agent))
	}
	sb.WriteString("\n用法: cg pipeline <名称[@agent]>")
	return a.sendClientNotify(req.route(), strings.TrimSpace(sb.String()))
}

func (a *CMDAGent) startAutoDeploy(route sessionRoute, result codegenToolResult) {
	agent, err := a.resolveDeployAgent(route.Project, "")
	if err != nil {
		_ = a.sendClientNotify(route, "⚠️ 编码完成，但自动部署跳过: "+err.Error())
		return
	}

	requestID := "cmd_autodeploy_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	deployRoute := route
	deployRoute.TargetAgentID = agent.AgentID
	deployRoute.Kind = "deploy"
	a.setPendingRoute(requestID, deployRoute)
	_ = a.sendClientNotify(deployRoute, fmt.Sprintf("🚀 自动部署已启动\n\n项目: %s\n请求: %s", route.Project, requestID))

	resultCh, err := a.callTool(agent.AgentID, requestID, "DeployProject", map[string]any{
		"project": route.Project,
	})
	if err != nil {
		_ = a.sendClientNotify(deployRoute, "❌ 自动部署启动失败: "+err.Error())
		return
	}
	a.awaitDeployResult(deployRoute, requestID, resultCh)
}

func (a *CMDAGent) resolveCodingAgent(project, preferredAgent string, allowAny bool) (gatewayAgentSnapshot, error) {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return gatewayAgentSnapshot{}, err
	}
	var candidates []gatewayAgentSnapshot
	var preferredMatches []gatewayAgentSnapshot
	for _, agent := range agents {
		if !supportsCodingAgent(agent) {
			continue
		}
		if preferredAgent != "" {
			if !matchesAgentName(agent, preferredAgent) {
				continue
			}
			preferredMatches = append(preferredMatches, agent)
		}
		projects := a.availableProjectsForAgent(agent)
		if allowAny || containsString(projects, project) {
			candidates = append(candidates, agent)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		if preferredAgent != "" {
			if len(preferredMatches) > 0 {
				return gatewayAgentSnapshot{}, fmt.Errorf("acp-agent %s 已在线，但未找到项目 %s，可先执行 cg list 确认项目列表", preferredAgent, project)
			}
			return gatewayAgentSnapshot{}, fmt.Errorf("未找到在线 coding agent: %s", preferredAgent)
		}
		if allowAny {
			return gatewayAgentSnapshot{}, fmt.Errorf("当前无在线 coding agent")
		}
		return gatewayAgentSnapshot{}, fmt.Errorf("未找到项目 %s，可先执行 cg list 或 cg create %s", project, project)
	}
	var names []string
	for _, agent := range candidates {
		names = append(names, agent.Name)
	}
	return gatewayAgentSnapshot{}, fmt.Errorf("多个 agent 都有项目 %s，请用 %s@<agent> 指定，可选: %s", project, project, strings.Join(uniqueSorted(names), ", "))
}

func (a *CMDAGent) resolveCodegenCreateAgent(project, preferredAgent string) (gatewayAgentSnapshot, error) {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return gatewayAgentSnapshot{}, err
	}
	var candidates []gatewayAgentSnapshot
	for _, agent := range agents {
		if !supportsCreateProject(agent) {
			continue
		}
		if preferredAgent != "" && !matchesAgentName(agent, preferredAgent) {
			continue
		}
		candidates = append(candidates, agent)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		if preferredAgent != "" {
			return gatewayAgentSnapshot{}, fmt.Errorf("未找到支持创建项目的 acp-agent: %s", preferredAgent)
		}
		return gatewayAgentSnapshot{}, fmt.Errorf("当前无在线 acp-agent，无法创建项目 %s", project)
	}
	var names []string
	for _, agent := range candidates {
		names = append(names, agent.Name)
	}
	return gatewayAgentSnapshot{}, fmt.Errorf("多个 acp-agent 在线，请用 %s@<agent> 指定，可选: %s", project, strings.Join(uniqueSorted(names), ", "))
}

func buildCodingStartCall(agent gatewayAgentSnapshot, callerAgentID, project, prompt, model, tool string) (map[string]any, string) {
	if hasTool(agent, "AcpStartSession") && !hasTool(agent, "CodegenStartSession") {
		args := map[string]any{
			"project":         project,
			"prompt":          prompt,
			"caller_agent_id": callerAgentID,
			"keep_session":    true,
		}
		return args, "AcpStartSession"
	}
	args := map[string]any{
		"project": project,
		"prompt":  prompt,
	}
	if model != "" {
		args["model"] = model
	}
	if tool != "" {
		args["tool"] = tool
	}
	return args, "CodegenStartSession"
}

func codingBackendKind(toolName string) string {
	if strings.HasPrefix(toolName, "Acp") {
		return "acp"
	}
	return "codegen"
}

func codingBackendKindForAgent(agentID, fallback string) string {
	if fallback == "acp" || strings.Contains(agentID, "acp") {
		return "acp"
	}
	return "codegen"
}

func statusToolName(backend string) string {
	return "CodegenGetStatus"
}

func stopToolName(backend string) string {
	if backend == "acp" {
		return "AcpStopSession"
	}
	return "CodegenStopSession"
}

func sendToolName(backend string) string {
	return "CodegenSendMessage"
}

func supportsCodingAgent(agent gatewayAgentSnapshot) bool {
	return hasTool(agent, "CodegenListProjects") || hasTool(agent, "AcpListProjects") || hasTool(agent, "AcpStartSession")
}

func supportsCreateProject(agent gatewayAgentSnapshot) bool {
	return hasTool(agent, "CodegenCreateProject") || hasTool(agent, "AcpCreateProject")
}

func createProjectToolName(agent gatewayAgentSnapshot) string {
	if hasTool(agent, "AcpCreateProject") && !hasTool(agent, "CodegenCreateProject") {
		return "AcpCreateProject"
	}
	return "CodegenCreateProject"
}

func (a *CMDAGent) resolveDeployAgent(project, preferredAgent string) (gatewayAgentSnapshot, error) {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return gatewayAgentSnapshot{}, err
	}
	var deployAgents []gatewayAgentSnapshot
	for _, agent := range agents {
		if !hasTool(agent, "DeployProject") {
			continue
		}
		if preferredAgent != "" && !matchesAgentName(agent, preferredAgent) {
			continue
		}
		deployAgents = append(deployAgents, agent)
	}
	if len(deployAgents) == 0 {
		return gatewayAgentSnapshot{}, fmt.Errorf("未找到可用的 deploy-agent")
	}
	if len(deployAgents) == 1 && preferredAgent == "" {
		return deployAgents[0], nil
	}

	var matches []gatewayAgentSnapshot
	for _, agent := range deployAgents {
		requestID := "cmd_probe_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		resultCh, callErr := a.callTool(agent.AgentID, requestID, "DeployListProjects", map[string]any{})
		if callErr != nil {
			continue
		}
		result := <-resultCh
		if !result.Success {
			continue
		}
		var payload struct {
			Projects []struct {
				Name string `json:"name"`
			} `json:"projects"`
		}
		if err := json.Unmarshal([]byte(result.Result), &payload); err != nil {
			continue
		}
		for _, item := range payload.Projects {
			if item.Name == project {
				matches = append(matches, agent)
				break
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		var names []string
		for _, agent := range matches {
			names = append(names, agent.Name)
		}
		return gatewayAgentSnapshot{}, fmt.Errorf("多个 deploy-agent 都配置了项目 %s，请用 %s@<agent> 指定，可选: %s", project, project, strings.Join(uniqueSorted(names), ", "))
	}
	if preferredAgent != "" {
		return deployAgents[0], nil
	}
	return gatewayAgentSnapshot{}, fmt.Errorf("未找到已配置项目 %s 的 deploy-agent", project)
}

func (a *CMDAGent) resolveAnyDeployAgent(preferredAgent string) (gatewayAgentSnapshot, error) {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return gatewayAgentSnapshot{}, err
	}
	var matches []gatewayAgentSnapshot
	for _, agent := range agents {
		if !hasTool(agent, "DeployProject") {
			continue
		}
		if preferredAgent != "" && !matchesAgentName(agent, preferredAgent) {
			continue
		}
		matches = append(matches, agent)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		if preferredAgent != "" {
			return gatewayAgentSnapshot{}, fmt.Errorf("未找到在线 deploy-agent: %s", preferredAgent)
		}
		return gatewayAgentSnapshot{}, fmt.Errorf("当前无在线 deploy-agent")
	}
	var names []string
	for _, agent := range matches {
		names = append(names, agent.Name)
	}
	return gatewayAgentSnapshot{}, fmt.Errorf("多个 deploy-agent 在线，请指定 @agent，可选: %s", strings.Join(uniqueSorted(names), ", "))
}

func (a *CMDAGent) resolveDeployProjectAgent(project, preferredAgent string) (gatewayAgentSnapshot, error) {
	if preferredAgent != "" {
		return a.resolveAnyDeployAgent(preferredAgent)
	}
	return a.resolveDeployAgent(project, preferredAgent)
}

func (a *CMDAGent) resolvePipelineAgent(pipeline, preferredAgent string) (gatewayAgentSnapshot, error) {
	agents, err := a.fetchGatewayAgents()
	if err != nil {
		return gatewayAgentSnapshot{}, err
	}
	var matches []gatewayAgentSnapshot
	for _, agent := range agents {
		if !hasTool(agent, "DeployPipeline") {
			continue
		}
		if preferredAgent != "" && !matchesAgentName(agent, preferredAgent) {
			continue
		}
		pipelines := stringSliceFromAny(agent.Meta["pipelines"])
		if preferredAgent != "" || containsString(pipelines, pipeline) {
			matches = append(matches, agent)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return gatewayAgentSnapshot{}, fmt.Errorf("未找到可用的 deploy-agent pipeline: %s", pipeline)
	}
	var names []string
	for _, agent := range matches {
		names = append(names, agent.Name)
	}
	return gatewayAgentSnapshot{}, fmt.Errorf("多个 deploy-agent 都有 pipeline %s，请用 %s@<agent> 指定，可选: %s", pipeline, pipeline, strings.Join(uniqueSorted(names), ", "))
}

func buildStartInfo(project, agentName, model, tool string, autoDeploy bool, requestID string) string {
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
	info += fmt.Sprintf("\n请求: %s\n\n进度将通过当前客户端推送", requestID)
	return info
}

func buildDeployInfo(project, agentName, deployTarget string, packOnly bool, requestID string) string {
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
	info += fmt.Sprintf("\n请求: %s\n\n进度将通过当前客户端推送", requestID)
	return info
}

func agentNameOrDefault(preferred, actual string) string {
	if strings.TrimSpace(preferred) != "" {
		return preferred
	}
	return actual
}

func matchesAgentName(agent gatewayAgentSnapshot, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return agent.Name == value || agent.AgentID == value
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
