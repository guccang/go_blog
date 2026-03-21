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
}

// LayerStatus represents one of the 5 availability layers.
type LayerStatus struct {
	Name   string      `json:"name"`
	Label  string      `json:"label"`
	Status StatusLevel `json:"status"`
	Items  []CheckItem `json:"items"`
}

// RunAvailabilityChecks runs all 5 layers of availability detection.
func RunAvailabilityChecks(rootDir string, configs map[string]map[string]any) []LayerStatus {
	layers := []LayerStatus{
		checkEnvironmentLayer(),
		checkConfigLayer(rootDir),
		checkServiceLayer(rootDir, configs),
		checkConnectivityLayer(configs),
		checkPortLayer(configs),
	}

	return layers
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
func PrintAvailabilityDashboard(layers []LayerStatus) {
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────┐")
	fmt.Println("  │            系统可用性面板                            │")
	fmt.Println("  └─────────────────────────────────────────────────────┘")
	fmt.Println()

	for _, layer := range layers {
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
func CheckAvailabilityForWeb(rootDir string, configs map[string]map[string]any) []LayerStatus {
	return RunAvailabilityChecks(rootDir, configs)
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
