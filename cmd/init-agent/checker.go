package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// SoftwareCheckResult holds the result of checking one software dependency.
type SoftwareCheckResult struct {
	Software         string `json:"software"`
	Installed        bool   `json:"installed"`
	Version          string `json:"version"`
	Path             string `json:"path"`
	MeetsRequirement bool   `json:"meets_requirement"`
	MinVersion       string `json:"min_version"`
	InstallHint      string `json:"install_hint"`
}

// SoftwareRequirement defines a required software and its minimum version.
type SoftwareRequirement struct {
	Software   string
	MinVersion string
}

// DefaultRequirements returns the software requirements for this monorepo.
func DefaultRequirements() []SoftwareRequirement {
	return []SoftwareRequirement{
		{Software: "go", MinVersion: "1.23.0"},
		{Software: "node", MinVersion: "18.0.0"},
		{Software: "redis", MinVersion: "6.0.0"},
		{Software: "git", MinVersion: "2.0.0"},
		{Software: "python", MinVersion: "3.6.0"},
		{Software: "docker", MinVersion: ""},
		{Software: "claude", MinVersion: ""},
	}
}

// softwareCheckCommands maps software names to version check commands.
var softwareCheckCommands = map[string]string{
	"go":      "go version",
	"node":    "node --version",
	"redis":   "redis-server --version",
	"git":     "git --version",
	"python":  "python3 --version",
	"docker":  "docker --version",
	"claude":  "claude --version",
	"npm":     "npm --version",
	"npx":     "npx --version",
	"gcc":     "gcc --version",
}

// softwarePathCommands maps software names to the command for finding the path.
// On Windows, checker_windows.go overrides lookPath.
var softwarePathCommands = map[string]string{
	"go":      "go",
	"node":    "node",
	"redis":   "redis-server",
	"git":     "git",
	"python":  "python3",
	"docker":  "docker",
	"claude":  "claude",
	"npm":     "npm",
	"npx":     "npx",
}

// installHints provides platform-specific install suggestions.
var installHints = map[string]map[string]string{
	"go": {
		"windows": "下载安装: https://go.dev/dl/  或  winget install GoLang.Go",
		"darwin":  "brew install go",
		"linux":   "sudo apt install golang  或  sudo snap install go --classic",
	},
	"node": {
		"windows": "下载安装: https://nodejs.org/  或  winget install OpenJS.NodeJS.LTS",
		"darwin":  "brew install node",
		"linux":   "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && sudo apt install -y nodejs",
	},
	"redis": {
		"windows": "下载: https://github.com/tporadowski/redis/releases  或使用 WSL",
		"darwin":  "brew install redis",
		"linux":   "sudo apt install redis-server",
	},
	"git": {
		"windows": "下载安装: https://git-scm.com/download/win  或  winget install Git.Git",
		"darwin":  "xcode-select --install  或  brew install git",
		"linux":   "sudo apt install git",
	},
	"python": {
		"windows": "下载安装: https://www.python.org/downloads/  或  winget install Python.Python.3.12",
		"darwin":  "brew install python3",
		"linux":   "sudo apt install python3",
	},
	"docker": {
		"windows": "下载 Docker Desktop: https://www.docker.com/products/docker-desktop/",
		"darwin":  "brew install --cask docker",
		"linux":   "curl -fsSL https://get.docker.com | sh",
	},
	"claude": {
		"windows": "npm install -g @anthropic-ai/claude-code",
		"darwin":  "npm install -g @anthropic-ai/claude-code",
		"linux":   "npm install -g @anthropic-ai/claude-code",
	},
}

func getInstallHint(software string) string {
	osKey := runtime.GOOS
	if hints, ok := installHints[software]; ok {
		if hint, ok := hints[osKey]; ok {
			return hint
		}
	}
	return ""
}

// RunEnvironmentChecks checks all default requirements.
func RunEnvironmentChecks() []SoftwareCheckResult {
	reqs := DefaultRequirements()
	return CheckSoftware(reqs)
}

// CheckSoftware runs checks for each requirement.
func CheckSoftware(reqs []SoftwareRequirement) []SoftwareCheckResult {
	results := make([]SoftwareCheckResult, 0, len(reqs))
	for _, req := range reqs {
		results = append(results, checkOne(req))
	}
	return results
}

func checkOne(req SoftwareRequirement) SoftwareCheckResult {
	r := SoftwareCheckResult{
		Software:   req.Software,
		MinVersion: req.MinVersion,
	}

	// Get check command
	cmd := softwareCheckCommands[req.Software]
	if cmd == "" {
		cmd = req.Software + " --version"
	}

	// Execute version check
	output, err := runShellCommand(cmd)
	if err != nil {
		r.Installed = false
		r.InstallHint = getInstallHint(req.Software)
		return r
	}

	r.Installed = true
	r.Version = parseVersion(output)

	// Find path
	r.Path = lookPath(req.Software)

	// Check minimum version
	if req.MinVersion == "" {
		r.MeetsRequirement = true
	} else if r.Version != "" {
		r.MeetsRequirement = compareVersions(r.Version, req.MinVersion) >= 0
	}

	if !r.MeetsRequirement && r.Installed {
		r.InstallHint = getInstallHint(req.Software)
	}

	return r
}

// runShellCommand executes a shell command and returns combined output.
func runShellCommand(command string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// lookPath finds the executable path. Overridden on Windows.
func lookPath(software string) string {
	binName := softwarePathCommands[software]
	if binName == "" {
		binName = software
	}
	p, err := exec.LookPath(binName)
	if err != nil {
		return ""
	}
	return p
}

// parseVersion extracts the first semver-like version from output.
var versionRegex = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

func parseVersion(output string) string {
	match := versionRegex.FindString(output)
	return match
}

// compareVersions compares two semantic version strings.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(partsA) {
			va, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			vb, _ = strconv.Atoi(partsB[i])
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

// PrintCheckResults prints environment check results to stdout.
func PrintCheckResults(results []SoftwareCheckResult) {
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────┐")
	fmt.Println("  │            环境依赖检测结果                          │")
	fmt.Println("  └─────────────────────────────────────────────────────┘")
	fmt.Println()

	for _, r := range results {
		status := colorGreen("✓")
		detail := ""
		if !r.Installed {
			status = colorRed("✗")
			detail = " (未安装)"
			if r.InstallHint != "" {
				detail += fmt.Sprintf("\n      安装: %s", r.InstallHint)
			}
		} else if !r.MeetsRequirement {
			status = colorYellow("!")
			detail = fmt.Sprintf(" (版本 %s < 要求 %s)", r.Version, r.MinVersion)
			if r.InstallHint != "" {
				detail += fmt.Sprintf("\n      升级: %s", r.InstallHint)
			}
		} else {
			if r.Version != "" {
				detail = fmt.Sprintf(" v%s", r.Version)
			}
			if r.Path != "" {
				detail += fmt.Sprintf("  (%s)", r.Path)
			}
		}
		fmt.Printf("  %s %-10s%s\n", status, r.Software, detail)
	}
	fmt.Println()
}
