package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// OSInfo 目标机器操作系统信息
type OSInfo struct {
	OS      string `json:"os"`      // "linux", "darwin", "windows"
	Distro  string `json:"distro"`  // "ubuntu", "centos", "debian", "fedora", ""
	Version string `json:"version"` // "22.04", "9", "14.0"
	Arch    string `json:"arch"`    // "x86_64", "aarch64"
	Raw     string `json:"raw"`     // 原始 uname 输出
}

// Requirement 软件需求
type Requirement struct {
	Software   string `json:"software"`    // "python", "go", "redis", ...
	MinVersion string `json:"min_version"` // "3.0", "1.21", "6.0"
}

// CheckResult 检测结果
type CheckResult struct {
	Software         string `json:"software"`
	Installed        bool   `json:"installed"`
	Version          string `json:"version,omitempty"`
	Path             string `json:"path,omitempty"`
	MeetsRequirement bool   `json:"meets_requirement"`
	Error            string `json:"error,omitempty"`
}

// SetupResult 安装结果
type SetupResult struct {
	Software string `json:"software"`
	Success  bool   `json:"success"`
	Version  string `json:"version,omitempty"`
	Path     string `json:"path,omitempty"`
	Method   string `json:"method,omitempty"` // "already_installed", "preset_script", "llm_generated"
	Error    string `json:"error,omitempty"`
}

// parseOSInfo 解析 OS 检测命令的输出
func parseOSInfo(output string) *OSInfo {
	info := &OSInfo{Raw: output}

	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return info
	}

	// 第一行是 uname -s -m 的输出
	firstLine := strings.TrimSpace(lines[0])
	parts := strings.Fields(firstLine)

	if len(parts) >= 1 {
		switch {
		case strings.Contains(strings.ToLower(parts[0]), "linux"):
			info.OS = "linux"
		case strings.Contains(strings.ToLower(parts[0]), "darwin"):
			info.OS = "darwin"
		case strings.Contains(strings.ToLower(parts[0]), "mingw"),
			strings.Contains(strings.ToLower(parts[0]), "msys"),
			strings.Contains(strings.ToLower(parts[0]), "cygwin"),
			strings.Contains(strings.ToLower(parts[0]), "windows"):
			info.OS = "windows"
		default:
			info.OS = strings.ToLower(parts[0])
		}
	}
	if len(parts) >= 2 {
		info.Arch = parts[len(parts)-1]
	}

	// 解析 /etc/os-release 获取发行版信息
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			info.Distro = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			info.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}

	return info
}

// versionRegex 通用版本号提取正则
var versionRegex = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

// parseVersion 从命令输出中提取版本号
func parseVersion(output string) string {
	match := versionRegex.FindString(output)
	return match
}

// compareVersions 比较两个版本号
// 返回: -1 (a < b), 0 (a == b), 1 (a > b)
func compareVersions(a, b string) int {
	if a == "" || b == "" {
		if a == b {
			return 0
		}
		if a == "" {
			return -1
		}
		return 1
	}

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			numA, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			numB, _ = strconv.Atoi(partsB[i])
		}
		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
	}
	return 0
}

// softwareCheckCommands 各软件的版本检测命令
var softwareCheckCommands = map[string]string{
	"python": "python3 --version 2>&1 || python --version 2>&1",
	"go":     "go version 2>&1",
	"node":   "node --version 2>&1",
	"redis":  "redis-server --version 2>&1",
	"mysql":  "mysql --version 2>&1",
	"docker": "docker --version 2>&1",
	"git":    "git --version 2>&1",
	"java":   "java -version 2>&1",
	"nginx":  "nginx -v 2>&1",
	"curl":   "curl --version 2>&1",
}

// softwareWhichCommands 各软件的路径检测命令
var softwareWhichCommands = map[string]string{
	"python": "which python3 2>/dev/null || which python 2>/dev/null",
	"go":     "which go 2>/dev/null",
	"node":   "which node 2>/dev/null",
	"redis":  "which redis-server 2>/dev/null",
	"mysql":  "which mysql 2>/dev/null",
	"docker": "which docker 2>/dev/null",
	"git":    "which git 2>/dev/null",
	"java":   "which java 2>/dev/null",
	"nginx":  "which nginx 2>/dev/null",
	"curl":   "which curl 2>/dev/null",
}

// getCheckCommand 获取软件版本检测命令
func getCheckCommand(software string) string {
	if cmd, ok := softwareCheckCommands[software]; ok {
		return cmd
	}
	return fmt.Sprintf("%s --version 2>&1", software)
}

// getWhichCommand 获取软件路径检测命令
func getWhichCommand(software string) string {
	if cmd, ok := softwareWhichCommands[software]; ok {
		return cmd
	}
	return fmt.Sprintf("which %s 2>/dev/null", software)
}

// commonSoftwareList 常用软件列表（用于 EnvCheckAll）
var commonSoftwareList = []string{
	"python", "go", "node", "git", "docker",
	"redis", "mysql", "java", "nginx", "curl",
}
