package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxInstructionFileChars = 12000
	maxGitStatusChars       = 2000
)

type instructionFile struct {
	Path    string
	Content string
}

func loadTrimmedFile(path string, limit int) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	content := string(data)
	if limit > 0 && len(content) > limit {
		content = content[:limit]
	}
	return strings.TrimSpace(content), true
}

func discoverInstructionFiles(startDir string) []instructionFile {
	startDir = strings.TrimSpace(startDir)
	if startDir == "" {
		return nil
	}

	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil
	}

	var dirs []string
	for dir := absStart; ; dir = filepath.Dir(dir) {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	for i, j := 0, len(dirs)-1; i < j; i, j = i+1, j-1 {
		dirs[i], dirs[j] = dirs[j], dirs[i]
	}

	seen := make(map[string]bool)
	var files []instructionFile

	addFile := func(path string) {
		if seen[path] {
			return
		}
		content, ok := loadTrimmedFile(path, maxInstructionFileChars)
		if !ok || content == "" {
			return
		}
		seen[path] = true
		files = append(files, instructionFile{
			Path:    path,
			Content: content,
		})
	}

	for _, dir := range dirs {
		for _, name := range []string{"AGENTS.md", "CLAUDE.md", "CLAUDE.local.md"} {
			addFile(filepath.Join(dir, name))
		}

		addFile(filepath.Join(dir, ".claude", "CLAUDE.md"))

		ruleDir := filepath.Join(dir, ".claude", "rules")
		entries, err := os.ReadDir(ruleDir)
		if err != nil {
			continue
		}
		var ruleFiles []string
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			ruleFiles = append(ruleFiles, filepath.Join(ruleDir, entry.Name()))
		}
		sort.Strings(ruleFiles)
		for _, path := range ruleFiles {
			addFile(path)
		}
	}

	return files
}

func buildInstructionBlock(startDir string) string {
	files := discoverInstructionFiles(startDir)
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 项目指令\n")
	sb.WriteString("以下文件是项目/本地指令，越靠后优先级越高，必须严格遵守：\n\n")
	for _, file := range files {
		sb.WriteString(fmt.Sprintf("### %s\n", file.Path))
		sb.WriteString(file.Content)
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String()) + "\n\n"
}

func runGitCommand(repoDir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func buildGitStatusBlock(startDir string) string {
	startDir = strings.TrimSpace(startDir)
	if startDir == "" {
		return ""
	}

	repoRoot := runGitCommand(startDir, "rev-parse", "--show-toplevel")
	if repoRoot == "" {
		return ""
	}

	branch := runGitCommand(repoRoot, "branch", "--show-current")
	status := runGitCommand(repoRoot, "--no-optional-locks", "status", "--short")
	logText := runGitCommand(repoRoot, "--no-optional-locks", "log", "--oneline", "-n", "5")

	if len(status) > maxGitStatusChars {
		status = status[:maxGitStatusChars] + "\n... (truncated)"
	}
	if status == "" {
		status = "(clean)"
	}

	var sb strings.Builder
	sb.WriteString("## Git 快照\n")
	sb.WriteString("这是当前轮开始时的仓库快照，不会自动更新：\n")
	sb.WriteString(fmt.Sprintf("- repo: %s\n", repoRoot))
	if branch != "" {
		sb.WriteString(fmt.Sprintf("- branch: %s\n", branch))
	}
	sb.WriteString("- status:\n")
	sb.WriteString(status)
	if logText != "" {
		sb.WriteString("\n- recent commits:\n")
		sb.WriteString(logText)
	}
	sb.WriteString("\n\n")
	return sb.String()
}
