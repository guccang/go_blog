package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// InitOptions CLI flags for --init
type InitOptions struct {
	StartArgs      string // --init-args
	SSHHost        string // --init-ssh
	RemoteDir      string // --init-remote-dir
	VerifyURL      string // --init-verify-url
	LinuxDir       string // --init-linux-dir
	MacDir         string // --init-mac-dir
	NonInteractive bool   // --yes
}

// InitConfig merged configuration (detection + CLI flags + user input)
type InitConfig struct {
	ProjectDir      string   // project absolute path
	ProjectName     string   // binary name (from go.mod module path last segment)
	ModulePath      string   // go.mod module path
	ExtraFiles      []string // extra files/dirs to package
	StartArgs       string   // startup arguments
	SSHHost         string   // SSH target (e.g. root@1.2.3.4)
	SSHPort         int      // SSH port, default 22
	RemoteDir       string   // remote deploy directory
	VerifyURL       string   // health check URL
	WinProjectDir   string   // Windows project path
	LinuxProjectDir string   // Linux project path
	MacProjectDir   string   // macOS project path
	SettingsDir     string   // deploy-agent settings dir (absolute)
	NonInteractive  bool     // non-interactive mode
}

// runInit entry point: detect -> merge CLI flags -> interactive/generate
func runInit(goProjectDir, configPath string, opts *InitOptions) error {
	// Resolve absolute path
	absDir, err := filepath.Abs(goProjectDir)
	if err != nil {
		return fmt.Errorf("resolve path: %v", err)
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("directory does not exist: %s", absDir)
	}

	// Detect Go project
	modPath, binName, err := detectGoProject(absDir)
	if err != nil {
		return err
	}

	// Scan extra files
	extras := scanExtraFiles(absDir, binName)

	// Resolve settings dir from deploy-agent.json
	settingsDir, _ := resolveSettingsDir(configPath)

	// Build InitConfig with defaults
	cfg := &InitConfig{
		ProjectDir:  absDir,
		ProjectName: binName,
		ModulePath:  modPath,
		ExtraFiles:  extras,
		SSHPort:     22,
		SettingsDir: settingsDir,
	}

	// Set current platform's project dir
	switch runtime.GOOS {
	case "windows":
		cfg.WinProjectDir = absDir
	case "darwin":
		cfg.MacProjectDir = absDir
	default:
		cfg.LinuxProjectDir = absDir
	}

	// Default Linux dir if not on Linux
	if cfg.LinuxProjectDir == "" {
		cfg.LinuxProjectDir = "/data/program/" + binName
	}

	// Apply CLI flags
	applyInitOptions(cfg, opts)

	if opts.NonInteractive {
		cfg.NonInteractive = true
	} else {
		if err := promptInitConfig(cfg); err != nil {
			return err
		}
	}

	// Generate and write files
	written, err := writeInitFiles(cfg)
	if err != nil {
		return err
	}

	fmt.Printf("\nDone! Generated %d files.\n", len(written))
	for i, f := range written {
		fmt.Printf("  %d. %s\n", i+1, f)
	}

	fmt.Printf("\nDeploy with:\n")
	fmt.Printf("  deploy-agent --project %s\n", binName)
	if cfg.SSHHost != "" {
		fmt.Printf("  deploy-agent --project %s --target ssh-prod\n", binName)
	}

	return nil
}

// detectGoProject reads go.mod to extract module path and binary name
func detectGoProject(dir string) (modulePath, binaryName string, err error) {
	goModPath := filepath.Join(dir, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		return "", "", fmt.Errorf("no go.mod found in %s", dir)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			break
		}
	}

	if modulePath == "" {
		return "", "", fmt.Errorf("module path not found in go.mod")
	}

	// Binary name = last segment of module path
	parts := strings.Split(modulePath, "/")
	binaryName = parts[len(parts)-1]

	return modulePath, binaryName, nil
}

// scanExtraFiles scans project directory for config files and resource directories
func scanExtraFiles(dir, binaryName string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	configExts := map[string]bool{
		".json": true, ".conf": true, ".yaml": true, ".yml": true, ".toml": true,
	}
	excludeFiles := map[string]bool{
		"package.json": true, "package-lock.json": true, "tsconfig.json": true,
		"go.mod": true, "go.sum": true,
		"zip-files.bat": true, "zip-files.sh": true,
		"publish.bat": true, "publish.sh": true,
	}
	resourceDirs := map[string]bool{
		"templates": true, "statics": true, "static": true, "configs": true, "settings": true,
	}
	excludeDirs := map[string]bool{
		".git": true, "vendor": true, "node_modules": true,
	}

	var extras []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if !excludeDirs[name] && resourceDirs[name] {
				extras = append(extras, name+"/")
			}
			continue
		}
		if excludeFiles[name] {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".go" || ext == ".zip" {
			continue
		}
		if name == binaryName || name == binaryName+".exe" {
			continue
		}
		if configExts[ext] {
			extras = append(extras, name)
		}
	}

	return extras
}

// applyInitOptions merges CLI flags into InitConfig
func applyInitOptions(cfg *InitConfig, opts *InitOptions) {
	if opts.StartArgs != "" {
		cfg.StartArgs = opts.StartArgs
	}
	if opts.SSHHost != "" {
		cfg.SSHHost = opts.SSHHost
	}
	if opts.RemoteDir != "" {
		cfg.RemoteDir = opts.RemoteDir
	}
	if opts.VerifyURL != "" {
		cfg.VerifyURL = opts.VerifyURL
	}
	if opts.LinuxDir != "" {
		cfg.LinuxProjectDir = opts.LinuxDir
	}
	if opts.MacDir != "" {
		cfg.MacProjectDir = opts.MacDir
	}
}

// resolveSettingsDir reads settings_dir from deploy-agent.json
func resolveSettingsDir(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("open config %s: %v", configPath, err)
	}

	var raw struct {
		SettingsDir string `json:"settings_dir"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parse config %s: %v", configPath, err)
	}

	sd := raw.SettingsDir
	if sd == "" {
		return "", fmt.Errorf("settings_dir not found in %s", configPath)
	}
	if !filepath.IsAbs(sd) {
		sd = filepath.Join(filepath.Dir(configPath), sd)
	}
	abs, err := filepath.Abs(sd)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// --- Interactive prompts ---

// promptInitConfig collects configuration interactively
func promptInitConfig(cfg *InitConfig) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("=== deploy-agent --init ===")
	fmt.Printf("Go project: %s\n", cfg.ModulePath)
	fmt.Printf("Binary: %s\n", cfg.ProjectName)
	fmt.Printf("Directory: %s\n", cfg.ProjectDir)

	// Extra files
	if len(cfg.ExtraFiles) > 0 {
		fmt.Println()
		fmt.Println("Extra files detected:")
		for _, f := range cfg.ExtraFiles {
			fmt.Printf("  - %s\n", f)
		}
		if !promptYesNo(reader, "Include?", true) {
			cfg.ExtraFiles = nil
		}
	}

	// Startup arguments
	fmt.Println()
	cfg.StartArgs = promptLine(reader, "Startup arguments (e.g. \"config.json\", empty for none)", cfg.StartArgs)

	// SSH target
	fmt.Println()
	fmt.Println("--- SSH Target (optional) ---")
	if promptYesNo(reader, "Add SSH target?", cfg.SSHHost != "") {
		cfg.SSHHost = promptLine(reader, "  Host (e.g. root@1.2.3.4)", cfg.SSHHost)
		defaultRemote := cfg.RemoteDir
		if defaultRemote == "" {
			defaultRemote = "/data/program/" + cfg.ProjectName
		}
		cfg.RemoteDir = promptLine(reader, "  Remote dir", defaultRemote)
		cfg.VerifyURL = promptLine(reader, "  Verify URL (optional)", cfg.VerifyURL)
	} else {
		cfg.SSHHost = ""
	}

	// Platform paths
	fmt.Println()
	fmt.Println("--- Platform Paths ---")
	fmt.Printf("Current: %s\n", platformSubdir())
	cfg.WinProjectDir = promptLine(reader, "  Win dir", cfg.WinProjectDir)
	cfg.LinuxProjectDir = promptLine(reader, "  Linux dir", cfg.LinuxProjectDir)
	cfg.MacProjectDir = promptLine(reader, "  macOS dir (empty to skip)", cfg.MacProjectDir)

	return nil
}

// promptLine reads a line from stdin with optional default value
func promptLine(reader *bufio.Reader, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// promptYesNo asks a yes/no question with a default
func promptYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	suffix := " [Y/n]: "
	if !defaultYes {
		suffix = " [y/N]: "
	}
	fmt.Printf("%s%s", prompt, suffix)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

// ensurePackScripts 确保打包+发布脚本存在（不覆盖已有文件）
// 用于 adhoc 部署模式，在执行部署前确保必要的脚本文件存在
func ensurePackScripts(cfg *InitConfig) error {
	type scriptFile struct {
		path    string
		content string
		mode    os.FileMode
	}

	scripts := []scriptFile{
		{filepath.Join(cfg.ProjectDir, "zip-files.bat"), generateZipFilesBat(cfg), 0644},
		{filepath.Join(cfg.ProjectDir, "zip-files.sh"), generateZipFilesSh(cfg), 0755},
		{filepath.Join(cfg.ProjectDir, "publish.bat"), generatePublishBat(cfg), 0644},
		{filepath.Join(cfg.ProjectDir, "publish.sh"), generatePublishSh(cfg), 0755},
	}

	for _, s := range scripts {
		if _, err := os.Stat(s.path); err == nil {
			continue // 文件已存在，不覆盖
		}
		if err := os.WriteFile(s.path, []byte(s.content), s.mode); err != nil {
			return fmt.Errorf("write %s: %v", s.path, err)
		}
	}

	return nil
}

// --- Script generators ---

// generateZipFilesBat generates Windows build/pack script content
func generateZipFilesBat(cfg *InitConfig) string {
	name := cfg.ProjectName

	// Build 7z file list: binary + publish scripts + extra files
	packItems := []string{"%BINNAME%", "publish.sh", "publish.bat"}
	for _, f := range cfg.ExtraFiles {
		packItems = append(packItems, f)
	}
	packList := strings.Join(packItems, " ")

	content := `@echo off
setlocal enabledelayedexpansion

taskkill /F /IM {{NAME}}.exe 2>nul

del /q *.zip 2>nul

:: Get timestamp
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT={{NAME}}_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

:: Cross-compilation: GOOS set by deploy-agent
set EXT=.exe
if defined GOOS (
    if not "%GOOS%"=="windows" set EXT=
)
set BINNAME={{NAME}}%EXT%

:: Clean old build artifacts
del {{NAME}}.exe 2>nul
del {{NAME}} 2>nul

go build -o %BINNAME%
if errorlevel 1 (
    echo Build failed
    exit /b 1
)

:: Package binary + config
%SEVENZIP% a -tzip "%OUTPUT%" {{PACK_LIST}}

:: Clean build artifacts
del %BINNAME%

echo Generated: %OUTPUT%
`

	content = strings.ReplaceAll(content, "{{NAME}}", name)
	content = strings.ReplaceAll(content, "{{PACK_LIST}}", packList)

	// Ensure CRLF line endings for .bat
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "\r\n")

	return content
}

// generateZipFilesSh generates Linux/macOS build/pack script content
func generateZipFilesSh(cfg *InitConfig) string {
	name := cfg.ProjectName

	// Build zip file list: binary + publish scripts + extra files
	packItems := []string{`"$BINNAME"`, "publish.sh", "publish.bat"}
	for _, f := range cfg.ExtraFiles {
		packItems = append(packItems, f)
	}
	packList := strings.Join(packItems, " ")

	content := `#!/bin/bash
set -e

# Get timestamp
TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="{{NAME}}_${TIMESTAMP}.zip"

# Cross-compilation: deploy-agent sets GOOS/GOARCH when needed
if [ -z "$GOOS" ]; then
    export GOOS=$(go env GOOS)
    export GOARCH=$(go env GOARCH)
fi
export CGO_ENABLED=0

EXT=""
[ "$GOOS" = "windows" ] && EXT=".exe"
BINNAME="{{NAME}}${EXT}"

echo "Building {{NAME}} (${GOOS}/${GOARCH})..."
go build -o "$BINNAME" .
if [ $? -ne 0 ]; then
    echo "Build failed"
    exit 1
fi

# Package binary + config
zip -r "${OUTPUT}" {{PACK_LIST}}

# Clean build artifacts
rm -f "$BINNAME"

echo "Generated: ${OUTPUT}"
`

	content = strings.ReplaceAll(content, "{{NAME}}", name)
	content = strings.ReplaceAll(content, "{{PACK_LIST}}", packList)

	// Ensure LF line endings for .sh
	content = strings.ReplaceAll(content, "\r\n", "\n")

	return content
}

// generatePublishBat generates Windows publish/restart script content
func generatePublishBat(cfg *InitConfig) string {
	name := cfg.ProjectName

	// Start command with or without args (no title param to avoid quote parsing issues)
	var startCmd string
	if cfg.StartArgs != "" {
		startCmd = fmt.Sprintf(`start cmd /c "%s.exe %s"`, name, cfg.StartArgs)
	} else {
		startCmd = fmt.Sprintf(`start cmd /c "%s.exe"`, name)
	}

	content := `@echo off
:: {{NAME}} local publish script
:: kill old process -> start new process
:: Note: use ping instead of timeout (timeout fails in non-interactive mode)

echo Stopping {{NAME}}...
taskkill /F /IM {{NAME}}.exe 2>nul

echo Starting {{NAME}}...
{{START_CMD}}

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq {{NAME}}.exe" 2>nul | find /I "{{NAME}}.exe" >nul
if %errorlevel%==0 (
    echo {{NAME}} started successfully
) else (
    echo {{NAME}} failed to start
    exit /b 1
)
`

	content = strings.ReplaceAll(content, "{{NAME}}", name)
	content = strings.ReplaceAll(content, "{{START_CMD}}", startCmd)

	// Ensure CRLF line endings for .bat
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "\r\n")

	return content
}

// generatePublishSh generates Linux/macOS publish/restart script content
func generatePublishSh(cfg *InitConfig) string {
	name := cfg.ProjectName

	// nohup command with or without args
	var nohupCmd string
	if cfg.StartArgs != "" {
		nohupCmd = fmt.Sprintf(`nohup "$svr" %s > %s.log 2>&1 < /dev/null &`, cfg.StartArgs, name)
	} else {
		nohupCmd = fmt.Sprintf(`nohup "$svr" > %s.log 2>&1 < /dev/null &`, name)
	}

	content := `#!/bin/bash
# {{NAME}} publish script
# kill old process -> start new process
cd "$(dirname "$0")"
svr="$(pwd)/{{NAME}}"
echo $svr

# Stop old process
echo "Stopping {{NAME}}..."
ps aux | grep "$svr" | grep -v "grep" | awk '{print $2}' | xargs kill -9
sleep 1

# Ensure executable
chmod +x {{NAME}}

# Start new process (background, detached)
# nohup + disown ensures process survives parent exit (macOS compatible)
echo "Starting {{NAME}}..."
{{NOHUP_CMD}}
disown

sleep 1
if pgrep -f "$svr" > /dev/null; then
    echo "{{NAME}} started (PID: $(pgrep -f "$svr"))"
else
    echo "{{NAME}} failed to start, check log:"
    tail -20 {{NAME}}.log
    exit 1
fi
`

	content = strings.ReplaceAll(content, "{{NAME}}", name)
	content = strings.ReplaceAll(content, "{{NOHUP_CMD}}", nohupCmd)

	// Ensure LF line endings for .sh
	content = strings.ReplaceAll(content, "\r\n", "\n")

	return content
}

// generateProjectJSON generates deploy-agent project .json content
func generateProjectJSON(cfg *InitConfig) string {
	name := cfg.ProjectName

	pj := projectJSON{
		PackPattern: name + "_{date}.zip",
		Build:       make(map[string]buildJSON),
		Targets:     make(map[string]targetJSON),
	}

	// build 配置
	if cfg.WinProjectDir != "" {
		pj.Build["win"] = buildJSON{ProjectDir: cfg.WinProjectDir, PackScript: "zip-files.bat"}
	}
	if cfg.LinuxProjectDir != "" {
		pj.Build["linux"] = buildJSON{ProjectDir: cfg.LinuxProjectDir, PackScript: "zip-files.sh"}
	}
	if cfg.MacProjectDir != "" {
		pj.Build["macos"] = buildJSON{ProjectDir: cfg.MacProjectDir, PackScript: "zip-files.sh"}
	}

	// target 配置
	if cfg.WinProjectDir != "" {
		pj.Targets["local.win"] = targetJSON{RemoteDir: cfg.WinProjectDir, RemoteScript: "publish.bat"}
	}
	if cfg.LinuxProjectDir != "" {
		pj.Targets["local.linux"] = targetJSON{RemoteDir: cfg.LinuxProjectDir, RemoteScript: "publish.sh"}
	}
	if cfg.MacProjectDir != "" {
		pj.Targets["local.macos"] = targetJSON{RemoteDir: cfg.MacProjectDir, RemoteScript: "publish.sh"}
	}

	// SSH target (optional)
	if cfg.SSHHost != "" {
		t := targetJSON{
			Platform:     "linux",
			Host:         cfg.SSHHost,
			RemoteScript: "publish.sh",
		}
		if cfg.RemoteDir != "" {
			t.RemoteDir = cfg.RemoteDir
		}
		if cfg.VerifyURL != "" {
			t.VerifyURL = cfg.VerifyURL
			t.VerifyTimeout = 10
		}
		pj.Targets["ssh-prod"] = t
	}

	data, _ := json.MarshalIndent(pj, "", "  ")
	return string(data) + "\n"
}

// writeInitFiles writes all generated files to disk
func writeInitFiles(cfg *InitConfig) ([]string, error) {
	type fileEntry struct {
		path    string
		content string
		mode    os.FileMode
	}

	var files []fileEntry

	// Script files in project directory
	files = append(files,
		fileEntry{filepath.Join(cfg.ProjectDir, "zip-files.bat"), generateZipFilesBat(cfg), 0644},
		fileEntry{filepath.Join(cfg.ProjectDir, "zip-files.sh"), generateZipFilesSh(cfg), 0755},
		fileEntry{filepath.Join(cfg.ProjectDir, "publish.bat"), generatePublishBat(cfg), 0644},
		fileEntry{filepath.Join(cfg.ProjectDir, "publish.sh"), generatePublishSh(cfg), 0755},
	)

	// Project json file
	confDir := ""
	if cfg.SettingsDir != "" {
		confDir = filepath.Join(cfg.SettingsDir, "projects")
	} else {
		// Fallback: put .json in project directory
		confDir = cfg.ProjectDir
		fmt.Println("Warning: Could not read deploy-agent.json to find settings_dir.")
		fmt.Println("         .json file will be written to project directory.")
		fmt.Println("         Move it to settings/projects/ for deploy-agent to recognize it.")
	}
	confPath := filepath.Join(confDir, cfg.ProjectName+".json")
	files = append(files, fileEntry{confPath, generateProjectJSON(cfg), 0644})

	// Show files to generate (interactive mode)
	if !cfg.NonInteractive {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println()
		fmt.Println("Files to generate:")
		for i, f := range files {
			exists := ""
			if _, err := os.Stat(f.path); err == nil {
				exists = " (exists, will overwrite)"
			}
			fmt.Printf("  %d. %s%s\n", i+1, f.path, exists)
		}
		if !promptYesNo(reader, "\nProceed?", true) {
			return nil, fmt.Errorf("cancelled by user")
		}
	}

	// Write files
	var written []string
	for _, f := range files {
		// Ensure directory exists
		dir := filepath.Dir(f.path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return written, fmt.Errorf("create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(f.path, []byte(f.content), f.mode); err != nil {
			return written, fmt.Errorf("write %s: %v", f.path, err)
		}
		written = append(written, f.path)
	}

	return written, nil
}
