package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InitConfig holds init-agent's own runtime configuration.
type InitConfig struct {
	Mode           string `json:"mode"`            // "cli" or "web"
	WebPort        int    `json:"web_port"`         // Web 模式监听端口
	RootDir        string `json:"root_dir"`         // monorepo root directory
	CheckOnly      bool   `json:"check_only"`       // 仅运行环境检测
	DashboardOnly  bool   `json:"dashboard_only"`   // 仅显示可用性面板
	NonInteractive bool   `json:"non_interactive"`  // 非交互模式

	// Gateway 共享配置（向导默认值）
	ServerURL   string `json:"server_url"`   // Gateway WebSocket URL
	GatewayHTTP string `json:"gateway_http"` // Gateway HTTP URL
	AuthToken   string `json:"auth_token"`   // Gateway 认证令牌
}

// DefaultConfig 默认配置
func DefaultConfig() *InitConfig {
	return &InitConfig{
		Mode:    "cli",
		WebPort: 9090,
	}
}

// LoadInitConfig reads an init-agent.json config file and returns an InitConfig.
func LoadInitConfig(path string) (*InitConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &InitConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析 init-agent 配置失败: %v", err)
	}
	return cfg, nil
}

// detectMonorepoRoot walks up from CWD looking for the root go.mod
// that contains the gateway, env-agent, etc.
func detectMonorepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if cmd/gateway exists — this is a reliable marker for the monorepo root
		if _, err := os.Stat(filepath.Join(dir, "cmd", "gateway")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("找不到包含 cmd/gateway 的父目录，请使用 --root 指定")
}

// listAgentDirs returns the agent directories found under cmd/.
func listAgentDirs(rootDir string) ([]string, error) {
	cmdDir := filepath.Join(rootDir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil, fmt.Errorf("读取 cmd/ 失败: %v", err)
	}

	var agents []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "common" || name == "init-agent" {
			continue
		}
		// Must contain at least a main.go or a go file
		if hasGoFiles(filepath.Join(cmdDir, name)) {
			agents = append(agents, name)
		}
	}
	return agents, nil
}

func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}

// writeDefaultConfig 将默认配置序列化为 JSON 并写入指定路径
func writeDefaultConfig(path string, cfg interface{}) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("配置文件已存在: %s（不会覆盖）", path)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}
	fmt.Printf("已生成默认配置文件: %s\n", path)
	return nil
}
