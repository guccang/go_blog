package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InitConfig holds init-agent's own runtime configuration.
type InitConfig struct {
	Mode           string // "cli" or "web"
	WebPort        int
	RootDir        string // monorepo root directory
	CheckOnly      bool
	DashboardOnly  bool
	NonInteractive bool
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
