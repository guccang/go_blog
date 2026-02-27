package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectInfo 项目信息
type ProjectInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Workspace string `json:"workspace"`
	FileCount int    `json:"file_count"`
	ModTime   string `json:"mod_time"`
}

// DirNode 目录树节点
type DirNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Size     int64      `json:"size,omitempty"`
	Children []*DirNode `json:"children,omitempty"`
}

// ListProjects 列出所有工作区下的项目
func ListProjects() ([]ProjectInfo, error) {
	projects := make([]ProjectInfo, 0)
	seen := make(map[string]bool)

	for _, ws := range workspaces {
		entries, err := os.ReadDir(ws)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			// 去重（同名项目只取第一个工作区的）
			if seen[entry.Name()] {
				continue
			}
			seen[entry.Name()] = true

			info, _ := entry.Info()
			modTime := ""
			if info != nil {
				modTime = info.ModTime().Format("2006-01-02 15:04")
			}

			fileCount := countFiles(filepath.Join(ws, entry.Name()))

			projects = append(projects, ProjectInfo{
				Name:      entry.Name(),
				Path:      entry.Name(),
				Workspace: ws,
				FileCount: fileCount,
				ModTime:   modTime,
			})
		}
	}

	// 按修改时间倒序
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ModTime > projects[j].ModTime
	})

	return projects, nil
}

// CreateProject 创建新项目（在默认工作区）
func CreateProject(name string) error {
	if name == "" {
		return fmt.Errorf("project name is empty")
	}

	// 安全检查
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid project name: %s", name)
	}

	// 检查是否已存在于任何工作区
	if _, err := ResolveProjectPath(name); err == nil {
		return fmt.Errorf("project already exists: %s", name)
	}

	projectPath := filepath.Join(GetDefaultWorkspace(), name)
	return os.MkdirAll(projectPath, 0755)
}

// GetProjectTree 获取项目目录树
func GetProjectTree(project string, maxDepth int) (*DirNode, error) {
	projectPath, err := ResolveProjectPath(project)
	if err != nil {
		return nil, err
	}

	return buildTree(projectPath, project, 0, maxDepth)
}

// buildTree 递归构建目录树
func buildTree(absPath, relPath string, depth, maxDepth int) (*DirNode, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	node := &DirNode{
		Name:  filepath.Base(absPath),
		Path:  relPath,
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		node.Size = info.Size()
		return node, nil
	}

	if depth >= maxDepth {
		return node, nil
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return node, nil
	}

	for _, entry := range entries {
		// 跳过隐藏文件和常见忽略目录
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor" {
			continue
		}

		childAbs := filepath.Join(absPath, name)
		childRel := filepath.Join(relPath, name)
		child, _ := buildTree(childAbs, childRel, depth+1, maxDepth)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	// 目录在前，文件在后
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})

	return node, nil
}

// ReadProjectFile 读取项目文件内容
func ReadProjectFile(project, filePath string) (string, error) {
	projectPath, err := ResolveProjectPath(project)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(projectPath, filePath)
	// 安全检查
	if !isSubPath(projectPath, fullPath) {
		return "", fmt.Errorf("invalid file path")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// countFiles 统计目录下文件数量
func countFiles(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}
