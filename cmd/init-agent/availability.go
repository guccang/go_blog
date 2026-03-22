package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// StatusLevel represents the health status of a check item.
type StatusLevel string

const (
	StatusGreen  StatusLevel = "green"
	StatusYellow StatusLevel = "yellow"
	StatusRed    StatusLevel = "red"
)

// CheckItem represents a single check within a layer.
type CheckItem struct {
	Name    string      `json:"name"`
	Status  StatusLevel `json:"status"`
	Detail  string      `json:"detail"`
	Target  string      `json:"target,omitempty"` // 所属 target（pipeline 模式下使用）
}

// PipelineAvailResult 单个 pipeline 的可用性检测结果
type PipelineAvailResult struct {
	PipelineName string              `json:"pipeline_name"`
	Description  string              `json:"description"`
	Targets      []TargetAvailResult `json:"targets"`
}

// TargetAvailResult 单台 target 在某 pipeline 中的可用性
type TargetAvailResult struct {
	TargetName string      `json:"target_name"`
	Host       string      `json:"host"`
	Platform   string      `json:"platform"`
	Projects   []string    `json:"projects"`
	EnvItems   []CheckItem `json:"env_items"`
	SvcItems   []CheckItem `json:"svc_items"`
	SSHError   string      `json:"ssh_error,omitempty"`
}

// LayerStatus represents one of the 5 availability layers.
type LayerStatus struct {
	Name   string      `json:"name"`
	Label  string      `json:"label"`
	Status StatusLevel `json:"status"`
	Items  []CheckItem `json:"items"`
}

// RunAvailabilityChecks runs all 5 layers of availability detection.
// 当有 pipeline 数据时，Layer 1 (环境) 和 Layer 3 (服务) 从 pipelineResults 提取；
// 无 pipeline 数据时保持原有本地检测逻辑。
func RunAvailabilityChecks(rootDir string, configs map[string]map[string]any,
	pipelineResults []PipelineAvailResult) []LayerStatus {

	hasPipeline := len(pipelineResults) > 0

	var envLayer LayerStatus
	var svcLayer LayerStatus

	if hasPipeline {
		envLayer = buildEnvLayerFromPipeline(pipelineResults)
		svcLayer = buildSvcLayerFromPipeline(pipelineResults)
	} else {
		envLayer = checkEnvironmentLayer()
		svcLayer = checkServiceLayer(rootDir, configs)
	}

	layers := []LayerStatus{
		envLayer,
		checkConfigLayer(rootDir),
		svcLayer,
		checkConnectivityLayer(configs),
		checkPortLayer(configs),
	}

	return layers
}

// buildEnvLayerFromPipeline 从 pipeline 检测结果提取环境依赖 layer
func buildEnvLayerFromPipeline(pipelineResults []PipelineAvailResult) LayerStatus {
	layer := LayerStatus{
		Name:  "environment",
		Label: "环境依赖",
	}

	for _, pr := range pipelineResults {
		for _, tr := range pr.Targets {
			if tr.SSHError != "" {
				layer.Items = append(layer.Items, CheckItem{
					Name:   tr.TargetName,
					Status: StatusRed,
					Detail: "SSH 错误: " + tr.SSHError,
					Target: tr.TargetName,
				})
				continue
			}
			for _, item := range tr.EnvItems {
				item.Target = tr.TargetName
				layer.Items = append(layer.Items, item)
			}
		}
	}

	layer.Status = aggregateStatus(layer.Items)
	return layer
}

// buildSvcLayerFromPipeline 从 pipeline 检测结果提取服务状态 layer
func buildSvcLayerFromPipeline(pipelineResults []PipelineAvailResult) LayerStatus {
	layer := LayerStatus{
		Name:  "service",
		Label: "服务状态",
	}

	for _, pr := range pipelineResults {
		for _, tr := range pr.Targets {
			if tr.SSHError != "" {
				continue // 环境层已报错
			}
			for _, item := range tr.SvcItems {
				item.Target = tr.TargetName
				layer.Items = append(layer.Items, item)
			}
		}
	}

	layer.Status = aggregateStatus(layer.Items)
	return layer
}

// RunPipelineAvailChecks 按 pipeline × target 分组执行可用性检测
// 环境数据从已有的 targetEnvResults 中查找，服务状态通过 SSH/本地检测进程
func RunPipelineAvailChecks(deployState *DeployConfigState, targetEnvResults []TargetEnvResult,
	sshPassword, sshKeyPath string) []PipelineAvailResult {

	if deployState == nil || len(deployState.Pipelines) == 0 {
		return nil
	}

	// 建立 targetEnvResults 的快速查找 map
	envResultMap := make(map[string]*TargetEnvResult)
	for i := range targetEnvResults {
		envResultMap[targetEnvResults[i].TargetName] = &targetEnvResults[i]
	}

	var results []PipelineAvailResult

	for _, pipeline := range deployState.Pipelines {
		pr := PipelineAvailResult{
			PipelineName: pipeline.Name,
			Description:  pipeline.Description,
		}

		// 按 target 分组该 pipeline 的 steps
		targetProjects := make(map[string][]string)
		for _, step := range pipeline.Steps {
			if step.PackOnly {
				continue
			}
			targetName := step.Target
			if targetName == "" {
				// 从 project 的 targets 推导
				if proj, ok := deployState.Projects[step.Project]; ok {
					for tn := range proj.Targets {
						addUniquePipeline(&targetProjects, tn, step.Project)
					}
				}
				continue
			}
			addUniquePipeline(&targetProjects, targetName, step.Project)
		}

		// 对每个 target 做检测
		for targetName, projNames := range targetProjects {
			target, ok := deployState.Targets[targetName]
			if !ok {
				continue
			}

			tr := TargetAvailResult{
				TargetName: targetName,
				Host:       target.Host,
				Platform:   target.Platform,
				Projects:   projNames,
			}

			if tr.Host == "" {
				tr.Host = "本机"
			}
			if tr.Platform == "" {
				tr.Platform = runtime.GOOS
			}

			// 1. 环境依赖：从 targetEnvResults 查找
			if envResult, ok := envResultMap[targetName]; ok {
				if envResult.SSHError != "" {
					tr.SSHError = envResult.SSHError
				} else {
					for _, cr := range envResult.Results {
						item := CheckItem{Name: cr.Software}
						if !cr.Installed {
							item.Status = StatusRed
							item.Detail = "未安装"
							if cr.InstallHint != "" {
								item.Detail += " — " + cr.InstallHint
							}
						} else if !cr.MeetsRequirement {
							item.Status = StatusYellow
							item.Detail = fmt.Sprintf("v%s < 要求 %s", cr.Version, cr.MinVersion)
						} else {
							item.Status = StatusGreen
							item.Detail = "v" + cr.Version
						}
						tr.EnvItems = append(tr.EnvItems, item)
					}
				}
			}

			// 2. 服务状态：按 project 检测进程
			if tr.SSHError == "" {
				tr.SvcItems = checkTargetServices(target, targetName, projNames, deployState.Projects, sshPassword, sshKeyPath)
			}

			pr.Targets = append(pr.Targets, tr)
		}

		results = append(results, pr)
	}

	return results
}

// checkTargetServices 检测目标机器上 project 对应的服务进程
func checkTargetServices(target DeployTarget, targetName string, projNames []string,
	projects map[string]*DeployProject, sshPassword, sshKeyPath string) []CheckItem {

	var items []CheckItem
	isLocal := isLocalTarget(targetName, target)

	// Bridge 类型无法检测
	if target.Type == "bridge" {
		items = append(items, CheckItem{
			Name:   targetName,
			Status: StatusYellow,
			Detail: "bridge 类型不支持进程检测",
		})
		return items
	}

	// 推导每个 project 对应的进程名
	for _, projName := range projNames {
		processName := resolveAgentName(projName, projects)
		item := CheckItem{Name: processName}

		if isLocal {
			if isProcessRunning(processName) {
				item.Status = StatusGreen
				item.Detail = "进程运行中"
			} else {
				item.Status = StatusRed
				item.Detail = "未检测到进程"
			}
		} else {
			// 远程 SSH 检测进程
			running, err := checkRemoteProcess(target, processName, sshPassword, sshKeyPath)
			if err != nil {
				item.Status = StatusYellow
				item.Detail = "检测失败: " + err.Error()
			} else if running {
				item.Status = StatusGreen
				item.Detail = "进程运行中"
			} else {
				item.Status = StatusRed
				item.Detail = "未检测到进程"
			}
		}

		items = append(items, item)
	}

	return items
}

// checkRemoteProcess 通过 SSH 检测远程机器上的进程
func checkRemoteProcess(target DeployTarget, processName, sshPassword, sshKeyPath string) (bool, error) {
	port := target.Port
	if port == 0 {
		port = 22
	}

	user := "root"
	host := target.Host
	if idx := strings.Index(host, "@"); idx >= 0 {
		user = host[:idx]
		host = host[idx+1:]
	}

	client, err := dialSSH(user, host, port, sshPassword, sshKeyPath)
	if err != nil {
		return false, err
	}
	defer client.Close()

	cmd := fmt.Sprintf("pgrep -f %s", processName)
	_, err = runSSHCommand(client, cmd)
	return err == nil, nil
}

// addUniquePipeline 向 map 中添加不重复的值（pipeline 内部使用）
func addUniquePipeline(m *map[string][]string, key, value string) {
	for _, v := range (*m)[key] {
		if v == value {
			return
		}
	}
	(*m)[key] = append((*m)[key], value)
}

// Layer 1: Environment
func checkEnvironmentLayer() LayerStatus {
	layer := LayerStatus{
		Name:  "environment",
		Label: "环境依赖",
	}

	results := RunEnvironmentChecks()
	for _, r := range results {
		item := CheckItem{Name: r.Software}
		if !r.Installed {
			item.Status = StatusRed
			item.Detail = "未安装"
			if r.InstallHint != "" {
				item.Detail += " — " + r.InstallHint
			}
		} else if !r.MeetsRequirement {
			item.Status = StatusYellow
			item.Detail = fmt.Sprintf("v%s < 要求 %s", r.Version, r.MinVersion)
		} else {
			item.Status = StatusGreen
			item.Detail = "v" + r.Version
		}
		layer.Items = append(layer.Items, item)
	}

	layer.Status = aggregateStatus(layer.Items)
	return layer
}

// Layer 2: Config files
func checkConfigLayer(rootDir string) LayerStatus {
	layer := LayerStatus{
		Name:  "config",
		Label: "配置文件",
	}

	for _, schema := range AllAgentSchemas() {
		path := filepath.Join(rootDir, schema.Dir, schema.ConfigFileName)
		item := CheckItem{Name: schema.Name}

		if !FileExists(path) {
			item.Status = StatusRed
			item.Detail = fmt.Sprintf("%s 不存在", schema.ConfigFileName)
		} else {
			// Try to parse the config
			_, err := LoadExistingConfig(rootDir, &schema)
			if err != nil {
				item.Status = StatusYellow
				item.Detail = fmt.Sprintf("文件存在但解析失败: %v", err)
			} else {
				item.Status = StatusGreen
				item.Detail = schema.ConfigFileName + " ✓"
			}
		}

		layer.Items = append(layer.Items, item)
	}

	layer.Status = aggregateStatus(layer.Items)
	return layer
}

// Layer 3: Service status (process detection)
func checkServiceLayer(rootDir string, configs map[string]map[string]any) LayerStatus {
	layer := LayerStatus{
		Name:  "service",
		Label: "服务状态",
	}

	// Try to query gateway's agent list first
	gatewayHTTP := resolveGatewayHTTP(configs)
	registeredAgents := queryGatewayAgents(gatewayHTTP)

	for _, schema := range AllAgentSchemas() {
		item := CheckItem{Name: schema.Name}

		if registeredAgents != nil {
			// Check if agent is registered in gateway
			if isAgentRegistered(registeredAgents, schema.Name) {
				item.Status = StatusGreen
				item.Detail = "已注册到 Gateway"
			} else {
				item.Status = StatusRed
				item.Detail = "未注册到 Gateway"
			}
		} else {
			// Fallback: check if process is running
			if isProcessRunning(schema.Name) {
				item.Status = StatusGreen
				item.Detail = "进程运行中"
			} else {
				item.Status = StatusRed
				item.Detail = "未检测到进程"
			}
		}

		layer.Items = append(layer.Items, item)
	}

	layer.Status = aggregateStatus(layer.Items)
	return layer
}

// Layer 4: Connectivity
func checkConnectivityLayer(configs map[string]map[string]any) LayerStatus {
	layer := LayerStatus{
		Name:  "connectivity",
		Label: "连通性",
	}

	gatewayHTTP := resolveGatewayHTTP(configs)

	// Check Gateway HTTP
	item := CheckItem{Name: "Gateway HTTP"}
	if checkHTTPHealth(gatewayHTTP + "/api/gateway/health") {
		item.Status = StatusGreen
		item.Detail = gatewayHTTP + " 可达"
	} else if checkHTTPReachable(gatewayHTTP) {
		item.Status = StatusYellow
		item.Detail = gatewayHTTP + " HTTP 可达但健康检查失败"
	} else {
		item.Status = StatusRed
		item.Detail = gatewayHTTP + " 不可达"
	}
	layer.Items = append(layer.Items, item)

	// Check Gateway WebSocket (just TCP connectivity)
	gatewayWS := resolveGatewayWS(configs)
	wsItem := CheckItem{Name: "Gateway WebSocket"}
	wsHost := extractHost(gatewayWS)
	if wsHost != "" && checkTCPConnectivity(wsHost) {
		wsItem.Status = StatusGreen
		wsItem.Detail = gatewayWS + " TCP 可达"
	} else {
		wsItem.Status = StatusRed
		wsItem.Detail = gatewayWS + " 不可达"
	}
	layer.Items = append(layer.Items, wsItem)

	layer.Status = aggregateStatus(layer.Items)
	return layer
}

// Layer 5: Port availability
func checkPortLayer(configs map[string]map[string]any) LayerStatus {
	layer := LayerStatus{
		Name:  "ports",
		Label: "端口占用",
	}

	// Collect all configured ports
	portsToCheck := map[int]string{}
	for _, schema := range AllAgentSchemas() {
		if schema.DefaultPort > 0 {
			portKey := schema.Name
			port := schema.DefaultPort

			// Check if there's a user-configured port
			if configs != nil {
				if cfg, ok := configs[schema.Name]; ok {
					if p, ok := cfg["port"]; ok {
						switch v := p.(type) {
						case float64:
							port = int(v)
						case int:
							port = v
						}
					}
					if p, ok := cfg["http_port"]; ok {
						switch v := p.(type) {
						case float64:
							port = int(v)
						case int:
							port = v
						}
					}
				}
			}

			if port > 0 {
				portsToCheck[port] = portKey
			}
		}
	}

	for port, agentName := range portsToCheck {
		item := CheckItem{
			Name: fmt.Sprintf(":%d (%s)", port, agentName),
		}
		if CheckPortAvailable(port) {
			item.Status = StatusGreen
			item.Detail = "端口可用"
		} else {
			item.Status = StatusYellow
			item.Detail = "端口已被占用（可能是服务已在运行）"
		}
		layer.Items = append(layer.Items, item)
	}

	layer.Status = aggregateStatus(layer.Items)
	return layer
}

// Helper functions

func aggregateStatus(items []CheckItem) StatusLevel {
	hasYellow := false
	for _, item := range items {
		if item.Status == StatusRed {
			return StatusRed
		}
		if item.Status == StatusYellow {
			hasYellow = true
		}
	}
	if hasYellow {
		return StatusYellow
	}
	return StatusGreen
}

func resolveGatewayHTTP(configs map[string]map[string]any) string {
	if configs != nil {
		if gw, ok := configs["gateway"]; ok {
			if port, ok := gw["port"]; ok {
				switch v := port.(type) {
				case float64:
					return fmt.Sprintf("http://127.0.0.1:%d", int(v))
				case int:
					return fmt.Sprintf("http://127.0.0.1:%d", v)
				}
			}
		}
	}
	return "http://127.0.0.1:10086"
}

func resolveGatewayWS(configs map[string]map[string]any) string {
	if configs != nil {
		if gw, ok := configs["gateway"]; ok {
			if port, ok := gw["port"]; ok {
				switch v := port.(type) {
				case float64:
					return fmt.Sprintf("ws://127.0.0.1:%d/ws/uap", int(v))
				case int:
					return fmt.Sprintf("ws://127.0.0.1:%d/ws/uap", v)
				}
			}
		}
	}
	return "ws://127.0.0.1:10086/ws/uap"
}

func extractHost(wsURL string) string {
	// ws://host:port/path -> host:port
	s := strings.TrimPrefix(wsURL, "ws://")
	s = strings.TrimPrefix(s, "wss://")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func checkHTTPHealth(url string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func checkHTTPReachable(baseURL string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(baseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

func checkTCPConnectivity(host string) bool {
	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func queryGatewayAgents(gatewayHTTP string) []map[string]any {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(gatewayHTTP + "/api/gateway/agents")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Success bool             `json:"success"`
		Agents  []map[string]any `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}
	if !result.Success {
		return nil
	}
	return result.Agents
}

func isAgentRegistered(agents []map[string]any, name string) bool {
	for _, a := range agents {
		if id, ok := a["agent_id"].(string); ok {
			if id == name || strings.Contains(id, name) {
				return true
			}
		}
		if n, ok := a["name"].(string); ok {
			if n == name || strings.Contains(n, name) {
				return true
			}
		}
	}
	return false
}

func isProcessRunning(name string) bool {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = fmt.Sprintf("tasklist /FI \"IMAGENAME eq %s.exe\" 2>NUL | findstr /I \"%s\"", name, name)
	} else {
		cmd = fmt.Sprintf("pgrep -f %s", name)
	}
	_, err := runShellCommand(cmd)
	return err == nil
}

// PrintAvailabilityDashboard prints the 5-layer availability status.
// 有 pipelineResults 时按 Pipeline → Target 分组输出，否则保持原有 5 层平铺。
func PrintAvailabilityDashboard(layers []LayerStatus, pipelineResults []PipelineAvailResult) {
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────┐")
	fmt.Println("  │            系统可用性面板                            │")
	fmt.Println("  └─────────────────────────────────────────────────────┘")
	fmt.Println()

	// 有 pipeline 数据时，先输出 pipeline 分组视图
	if len(pipelineResults) > 0 {
		printPipelineAvailDashboard(pipelineResults)
		fmt.Println()
	}

	// 然后输出 Layer 2 (配置), Layer 4 (连通性), Layer 5 (端口) — 这些始终平铺
	for _, layer := range layers {
		// 有 pipeline 数据时跳过 Layer 1/3（已在上面分组输出）
		if len(pipelineResults) > 0 && (layer.Name == "environment" || layer.Name == "service") {
			continue
		}
		icon := statusIcon(layer.Status)
		fmt.Printf("  %s %s\n", icon, colorBold(layer.Label))

		for _, item := range layer.Items {
			itemIcon := statusIcon(item.Status)
			fmt.Printf("    %s %-25s %s\n", itemIcon, item.Name, colorDim(item.Detail))
		}
		fmt.Println()
	}

	// Summary
	greenCount, yellowCount, redCount := 0, 0, 0
	for _, l := range layers {
		switch l.Status {
		case StatusGreen:
			greenCount++
		case StatusYellow:
			yellowCount++
		case StatusRed:
			redCount++
		}
	}

	fmt.Printf("  总结: %s %d 正常  %s %d 警告  %s %d 异常\n",
		colorGreen("●"), greenCount,
		colorYellow("●"), yellowCount,
		colorRed("●"), redCount,
	)

	if redCount > 0 {
		fmt.Println()
		fmt.Println("  建议:")
		fmt.Println("    1. 运行 init-agent --check 检查环境依赖")
		fmt.Println("    2. 运行 init-agent --mode cli 生成缺失的配置文件")
		fmt.Println("    3. 启动 gateway 和所需的 agent")
	}

	fmt.Println()
}

// printPipelineAvailDashboard 按 Pipeline → Target 分组输出环境+服务检测
func printPipelineAvailDashboard(pipelineResults []PipelineAvailResult) {
	for _, pr := range pipelineResults {
		desc := ""
		if pr.Description != "" {
			desc = " — " + pr.Description
		}
		fmt.Printf("  %s Pipeline: %s%s\n", colorCyan("▸"), colorBold(pr.PipelineName), colorDim(desc))

		for _, tr := range pr.Targets {
			hostInfo := tr.Host
			if hostInfo == "" {
				hostInfo = "本机"
			}
			platformInfo := tr.Platform
			if platformInfo == "" {
				platformInfo = "unknown"
			}

			title := fmt.Sprintf("%s (%s, %s)", tr.TargetName, hostInfo, platformInfo)
			projectList := strings.Join(tr.Projects, ", ")

			titleLen := displayWidth(title)
			contentWidth := titleLen + 4
			if contentWidth < 50 {
				contentWidth = 50
			}

			fmt.Printf("    ┌─ %s %s┐\n", title, strings.Repeat("─", contentWidth-titleLen-2))
			fmt.Printf("    │  部署项目: %-*s│\n", contentWidth-4, projectList)

			if tr.SSHError != "" {
				fmt.Printf("    │  %s SSH 错误: %-*s│\n", colorRed("✗"), contentWidth-12, tr.SSHError)
			} else {
				// 环境依赖
				if len(tr.EnvItems) > 0 {
					fmt.Printf("    │  %s\n", colorBold("环境依赖:"))
					for _, item := range tr.EnvItems {
						icon := statusIcon(item.Status)
						fmt.Printf("    │  %s %-15s %s\n", icon, item.Name, colorDim(item.Detail))
					}
				}
				// 服务状态
				if len(tr.SvcItems) > 0 {
					fmt.Printf("    │  %s\n", colorBold("服务状态:"))
					for _, item := range tr.SvcItems {
						icon := statusIcon(item.Status)
						fmt.Printf("    │  %s %-15s %s\n", icon, item.Name, colorDim(item.Detail))
					}
				}
			}

			fmt.Printf("    └%s┘\n", strings.Repeat("─", contentWidth))
		}
		fmt.Println()
	}
}

func statusIcon(status StatusLevel) string {
	switch status {
	case StatusGreen:
		return colorGreen("●")
	case StatusYellow:
		return colorYellow("●")
	case StatusRed:
		return colorRed("●")
	default:
		return "○"
	}
}

// CheckEnvironmentForWeb runs environment checks and returns results for web API.
func CheckEnvironmentForWeb() []SoftwareCheckResult {
	return RunEnvironmentChecks()
}

// CheckAvailabilityForWeb runs availability checks and returns results for web API.
func CheckAvailabilityForWeb(rootDir string, configs map[string]map[string]any,
	pipelineResults []PipelineAvailResult) []LayerStatus {
	return RunAvailabilityChecks(rootDir, configs, pipelineResults)
}

// findMonorepoRoot searches for the monorepo root from a given starting directory.
func findMonorepoRoot(startDir string) (string, error) {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "cmd", "gateway")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("not found")
}
