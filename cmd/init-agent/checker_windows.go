//go:build windows

package main

import (
	"os/exec"
	"strings"
)

func init() {
	// On Windows, python3 may not exist; try python first
	softwareCheckCommands["python"] = "python --version 2>&1 || python3 --version 2>&1"
	softwarePathCommands["python"] = "python"

	// Redis on Windows may use different binary names
	softwareCheckCommands["redis"] = "redis-server --version 2>&1 || redis-server.exe --version 2>&1"
}

// lookPathWindows uses 'where' command as fallback on Windows.
func lookPathWindows(software string) string {
	binName := softwarePathCommands[software]
	if binName == "" {
		binName = software
	}

	// Try exec.LookPath first
	if p, err := exec.LookPath(binName); err == nil {
		return p
	}

	// Fallback: use 'where' command
	out, err := exec.Command("where", binName).CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}
