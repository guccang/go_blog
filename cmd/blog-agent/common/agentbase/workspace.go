package agentbase

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceInfo workspace 加载结果
type WorkspaceInfo struct {
	Dir     string // workspace 目录路径
	Summary string // AGENT.md 首行（用作 Description）
	Detail  string // AGENT.md 全文（详细描述）
}

// LoadWorkspace 从指定目录加载 workspace 信息
// - 读取 AGENT.md: 首行作为 Summary, 全文作为 Detail
// - 文件不存在时对应字段为空（不报错）
func LoadWorkspace(workspaceDir string) *WorkspaceInfo {
	ws := &WorkspaceInfo{
		Dir: workspaceDir,
	}

	agentMDPath := filepath.Join(workspaceDir, "AGENT.md")
	data, err := os.ReadFile(agentMDPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[Workspace] 读取 AGENT.md 失败: %v", err)
		}
		return ws
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return ws
	}

	ws.Detail = content

	// 首行提取：按 \n split，取第一个非空行
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			ws.Summary = trimmed
			break
		}
	}

	return ws
}
