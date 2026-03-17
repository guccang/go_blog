package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// LLMRequestPayload 发送给 llm-agent 的任务载荷
type LLMRequestPayload struct {
	TaskType      string        `json:"task_type"`       // "llm_request"
	Messages      []LLMMessage  `json:"messages"`
	SelectedTools []string      `json:"selected_tools"`  // 限定工具列表
	Account       string        `json:"account"`         // 调用方标识
}

// LLMMessage LLM 对话消息
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Orchestrator 环境编排器
type Orchestrator struct {
	conn *Connection
}

// NewOrchestrator 创建编排器
func NewOrchestrator(conn *Connection) *Orchestrator {
	return &Orchestrator{conn: conn}
}

// ========================= 工具处理入口 =========================

// handleEnvCheck 处理 EnvCheck 工具调用
func (o *Orchestrator) handleEnvCheck(args map[string]interface{}) (string, bool) {
	targetAgent, _ := args["target_agent"].(string)
	software, _ := args["software"].(string)
	minVersion, _ := args["min_version"].(string)

	if targetAgent == "" || software == "" {
		return marshalResult(false, "缺少 target_agent 或 software 参数"), false
	}

	result := o.checkSoftware(targetAgent, nil, Requirement{Software: software, MinVersion: minVersion})
	data, _ := json.Marshal(result)
	return string(data), result.Installed
}

// handleEnvInstall 处理 EnvInstall 工具调用
func (o *Orchestrator) handleEnvInstall(args map[string]interface{}) (string, bool) {
	targetAgent, _ := args["target_agent"].(string)
	software, _ := args["software"].(string)
	minVersion, _ := args["min_version"].(string)

	if targetAgent == "" || software == "" {
		return marshalResult(false, "缺少 target_agent 或 software 参数"), false
	}

	result := o.installSoftware(targetAgent, Requirement{Software: software, MinVersion: minVersion})
	data, _ := json.Marshal(result)
	return string(data), result.Success
}

// handleEnvCheckAll 处理 EnvCheckAll 工具调用
func (o *Orchestrator) handleEnvCheckAll(args map[string]interface{}) (string, bool) {
	targetAgent, _ := args["target_agent"].(string)
	if targetAgent == "" {
		return marshalResult(false, "缺少 target_agent 参数"), false
	}

	// 先检测 OS
	osInfo, err := o.detectOS(targetAgent)
	if err != nil {
		return marshalResult(false, fmt.Sprintf("OS 检测失败: %v", err)), false
	}

	results := make([]CheckResult, 0, len(commonSoftwareList))
	for _, sw := range commonSoftwareList {
		result := o.checkSoftware(targetAgent, osInfo, Requirement{Software: sw})
		results = append(results, result)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"os_info": osInfo,
		"results": results,
	})
	return string(data), true
}

// handleEnvSetup 处理 EnvSetup 工具调用
func (o *Orchestrator) handleEnvSetup(args map[string]interface{}) (string, bool) {
	targetAgent, _ := args["target_agent"].(string)
	if targetAgent == "" {
		return marshalResult(false, "缺少 target_agent 参数"), false
	}

	// 解析 requirements
	reqsRaw, ok := args["requirements"]
	if !ok {
		return marshalResult(false, "缺少 requirements 参数"), false
	}
	reqsJSON, _ := json.Marshal(reqsRaw)
	var requirements []Requirement
	if err := json.Unmarshal(reqsJSON, &requirements); err != nil {
		return marshalResult(false, "requirements 格式错误: "+err.Error()), false
	}

	results := o.setup(targetAgent, requirements)

	allSuccess := true
	for _, r := range results {
		if !r.Success {
			allSuccess = false
			break
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"results": results,
	})
	return string(data), allSuccess
}

// ========================= 核心编排逻辑 =========================

// detectOS 远程检测目标机器操作系统
func (o *Orchestrator) detectOS(targetAgent string) (*OSInfo, error) {
	result, err := o.execRemote(targetAgent, "uname -s -m && cat /etc/os-release 2>/dev/null || ver 2>nul", 30)
	if err != nil {
		return nil, fmt.Errorf("OS 检测命令执行失败: %v", err)
	}
	return parseOSInfo(result), nil
}

// checkSoftware 远程检测软件安装状态
// osInfo 可以为 nil，会自动检测
func (o *Orchestrator) checkSoftware(targetAgent string, osInfo *OSInfo, req Requirement) CheckResult {
	result := CheckResult{Software: req.Software}

	// 获取检测命令
	checkCmd := getCheckCommand(req.Software)

	// 如果有 osInfo，尝试用预置脚本的检测命令
	if osInfo != nil {
		if script, found := findScript(osInfo.OS, osInfo.Distro, req.Software, "check"); found {
			checkCmd = script
		}
	}

	// 执行检测
	output, err := o.execRemote(targetAgent, checkCmd, 30)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// 解析版本
	version := parseVersion(output)
	if version == "" {
		// 可能未安装
		if strings.Contains(strings.ToLower(output), "not found") ||
			strings.Contains(strings.ToLower(output), "no such file") ||
			strings.Contains(strings.ToLower(output), "command not found") {
			result.Installed = false
			return result
		}
		// 有输出但无法解析版本，认为已安装
		result.Installed = true
		result.Version = "unknown"
	} else {
		result.Installed = true
		result.Version = version
	}

	// 检测路径
	whichCmd := getWhichCommand(req.Software)
	if pathOutput, err := o.execRemote(targetAgent, whichCmd, 10); err == nil {
		result.Path = strings.TrimSpace(pathOutput)
	}

	// 版本比较
	if req.MinVersion != "" && result.Version != "" && result.Version != "unknown" {
		result.MeetsRequirement = compareVersions(result.Version, req.MinVersion) >= 0
	} else if result.Installed && req.MinVersion == "" {
		result.MeetsRequirement = true
	}

	return result
}

// installSoftware 安装软件的完整流程
func (o *Orchestrator) installSoftware(targetAgent string, req Requirement) SetupResult {
	result := SetupResult{Software: req.Software}

	// 1. 检测 OS
	osInfo, err := o.detectOS(targetAgent)
	if err != nil {
		result.Error = fmt.Sprintf("OS 检测失败: %v", err)
		return result
	}

	// 2. 先检测是否已安装
	check := o.checkSoftware(targetAgent, osInfo, req)
	if check.MeetsRequirement {
		result.Success = true
		result.Version = check.Version
		result.Path = check.Path
		result.Method = "already_installed"
		return result
	}

	// 3. 查找预置安装脚本
	installScript, found := findScript(osInfo.OS, osInfo.Distro, req.Software, "install")
	if found {
		log.Printf("[EnvAgent] 使用预置脚本安装 %s: %s", req.Software, installScript)
		timeout := o.conn.cfg.InstallTimeout
		_, err := o.execRemote(targetAgent, installScript, timeout)
		if err == nil {
			// 安装后重新检测
			check = o.checkSoftware(targetAgent, osInfo, req)
			if check.Installed {
				result.Success = true
				result.Version = check.Version
				result.Path = check.Path
				result.Method = "preset_script"
				if req.MinVersion != "" && !check.MeetsRequirement {
					result.Success = false
					result.Error = fmt.Sprintf("已安装版本 %s 不满足最低要求 %s", check.Version, req.MinVersion)
				}
				return result
			}
		}
		log.Printf("[EnvAgent] 预置脚本安装失败，降级到 LLM: %v", err)
	}

	// 4. 路径 B：委托 llm-agent
	return o.delegateToLLM(targetAgent, osInfo, req)
}

// setup 批量检测+安装
func (o *Orchestrator) setup(targetAgent string, requirements []Requirement) []SetupResult {
	// 先检测 OS（只检测一次）
	osInfo, err := o.detectOS(targetAgent)
	if err != nil {
		// OS 检测失败，所有需求都标记失败
		results := make([]SetupResult, len(requirements))
		for i, req := range requirements {
			results[i] = SetupResult{
				Software: req.Software,
				Error:    fmt.Sprintf("OS 检测失败: %v", err),
			}
		}
		return results
	}

	log.Printf("[EnvAgent] 目标机器 OS: %s %s %s (%s)", osInfo.OS, osInfo.Distro, osInfo.Version, osInfo.Arch)

	results := make([]SetupResult, 0, len(requirements))
	for _, req := range requirements {
		// 先检测
		check := o.checkSoftware(targetAgent, osInfo, req)
		if check.MeetsRequirement {
			results = append(results, SetupResult{
				Software: req.Software,
				Success:  true,
				Version:  check.Version,
				Path:     check.Path,
				Method:   "already_installed",
			})
			continue
		}

		// 需要安装
		log.Printf("[EnvAgent] %s 需要安装 (installed=%v version=%s minVersion=%s)",
			req.Software, check.Installed, check.Version, req.MinVersion)

		// 查找预置脚本
		installScript, found := findScript(osInfo.OS, osInfo.Distro, req.Software, "install")
		if found {
			log.Printf("[EnvAgent] 使用预置脚本: %s", installScript)
			_, execErr := o.execRemote(targetAgent, installScript, o.conn.cfg.InstallTimeout)
			if execErr == nil {
				recheck := o.checkSoftware(targetAgent, osInfo, req)
				if recheck.MeetsRequirement {
					results = append(results, SetupResult{
						Software: req.Software,
						Success:  true,
						Version:  recheck.Version,
						Path:     recheck.Path,
						Method:   "preset_script",
					})
					continue
				}
			}
			log.Printf("[EnvAgent] 预置脚本失败，降级到 LLM")
		}

		// 委托 LLM
		llmResult := o.delegateToLLM(targetAgent, osInfo, req)
		results = append(results, llmResult)
	}

	return results
}

// delegateToLLM 将安装任务委托给 llm-agent
func (o *Orchestrator) delegateToLLM(targetAgent string, osInfo *OSInfo, req Requirement) SetupResult {
	result := SetupResult{Software: req.Software, Method: "llm_generated"}

	// 查找 ExecEnvBash 工具的完整名称（带前缀）
	execToolName := o.findExecEnvBashTool(targetAgent)
	if execToolName == "" {
		execToolName = "ExecExecEnvBash" // 默认前缀 "Exec"
	}

	minVersionStr := "任意版本"
	if req.MinVersion != "" {
		minVersionStr = fmt.Sprintf(">= %s", req.MinVersion)
	}

	systemPrompt := fmt.Sprintf(
		"你是 Linux 环境安装专家。目标机器信息: %s %s %s (%s)。\n"+
			"你可以通过 %s 工具在目标机器上执行命令。\n"+
			"任务: 检测并安装 %s (%s)。\n"+
			"要求:\n"+
			"1. 先检测是否已安装及版本\n"+
			"2. 未安装或版本不满足则安装\n"+
			"3. 安装后验证版本\n"+
			"4. 最终输出 JSON: {\"success\":true/false, \"version\":\"x.y.z\", \"path\":\"/usr/bin/xxx\"}",
		osInfo.OS, osInfo.Distro, osInfo.Version, osInfo.Arch,
		execToolName,
		req.Software, minVersionStr,
	)

	// 查找 llm-agent 的 agent ID
	llmAgentID := o.conn.cfg.LLMAgentID

	taskPayload := LLMRequestPayload{
		TaskType: "llm_request",
		Messages: []LLMMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("请在目标机器上检测并安装 %s", req.Software)},
		},
		SelectedTools: []string{execToolName},
		Account:       "env-agent",
	}

	timeout := time.Duration(o.conn.cfg.LLMTaskTimeout) * time.Second
	log.Printf("[EnvAgent] 委托 LLM 安装 %s (timeout=%ds)", req.Software, o.conn.cfg.LLMTaskTimeout)

	taskResult, err := o.conn.sendTaskAssign(llmAgentID, taskPayload, timeout)
	if err != nil {
		result.Error = fmt.Sprintf("LLM 任务失败: %v", err)
		return result
	}

	if taskResult.Status != "success" {
		result.Error = fmt.Sprintf("LLM 任务状态: %s, error: %s", taskResult.Status, taskResult.Error)
		return result
	}

	// 解析 LLM 返回结果
	var llmResult struct {
		Success bool   `json:"success"`
		Version string `json:"version"`
		Path    string `json:"path"`
	}
	// 尝试从 result 中提取 JSON
	if err := json.Unmarshal([]byte(taskResult.Result), &llmResult); err == nil && llmResult.Success {
		result.Success = true
		result.Version = llmResult.Version
		result.Path = llmResult.Path
		return result
	}

	// LLM 返回的可能是纯文本，做最后一次检测确认
	check := o.checkSoftware(targetAgent, osInfo, Requirement{Software: req.Software, MinVersion: req.MinVersion})
	if check.MeetsRequirement {
		result.Success = true
		result.Version = check.Version
		result.Path = check.Path
		return result
	}

	result.Error = fmt.Sprintf("LLM 安装后验证失败: %s", taskResult.Result)
	return result
}

// ========================= 辅助方法 =========================

// execRemote 在目标 agent 上执行命令
func (o *Orchestrator) execRemote(targetAgent string, command string, timeoutSec int) (string, error) {
	// 构建 ExecEnvBash 工具调用参数
	args := map[string]interface{}{
		"command": command,
		"timeout": timeoutSec,
	}

	// 查找目标 agent 的 ExecEnvBash 工具名
	toolName := o.findExecEnvBashTool(targetAgent)
	if toolName == "" {
		return "", fmt.Errorf("target agent %s 没有 ExecEnvBash 工具", targetAgent)
	}

	timeout := time.Duration(timeoutSec+10) * time.Second // 额外 10s 网络余量
	result, err := o.conn.callRemoteTool(toolName, args, timeout)
	if err != nil {
		return "", err
	}

	// 解析结果
	var res struct {
		Success  bool   `json:"success"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode int    `json:"exit_code"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal([]byte(result), &res); err != nil {
		return result, nil // 无法解析就原样返回
	}

	if !res.Success {
		errMsg := res.Error
		if errMsg == "" && res.Stderr != "" {
			errMsg = res.Stderr
		}
		return res.Stdout, fmt.Errorf("exit_code=%d: %s", res.ExitCode, errMsg)
	}

	return strings.TrimSpace(res.Stdout), nil
}

// findExecEnvBashTool 查找目标 agent 的 ExecEnvBash 工具名
// 仅精确匹配 targetAgent，找不到则返回空（不执行命令）
func (o *Orchestrator) findExecEnvBashTool(targetAgent string) string {
	all := o.conn.toolCatalog.GetAll()
	for toolName, agentID := range all {
		if strings.HasSuffix(toolName, "ExecEnvBash") && agentID == targetAgent {
			return toolName
		}
	}
	return ""
}

// marshalResult 构建简单的 JSON 结果
func marshalResult(success bool, errMsg string) string {
	data, _ := json.Marshal(map[string]interface{}{
		"success": success,
		"error":   errMsg,
	})
	return string(data)
}
