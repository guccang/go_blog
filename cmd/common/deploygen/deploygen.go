package deploygen

import (
	"fmt"
	"os"
	"strings"
)

// DeployOptions 部署脚本生成选项
type DeployOptions struct {
	AgentName  string   // 二进制名称，如 "llm-agent"
	ConfigFile string   // 配置文件名，如 "llm-agent.json"（空=不打包配置文件）
	ZipExtras  []string // zip 额外打包的文件/目录，如 ["workspace/", "settings/"]
	UsePIDFile bool     // 是否使用 PID 文件管理进程（含子进程树的 agent）
	StartArgs  string   // 启动参数覆盖（默认 "-config {ConfigFile}"）
}

// GenerateDeployFiles 生成 publish.bat、publish.sh、zip-files.bat、zip-files.sh
// 已存在的文件不会覆盖
func GenerateDeployFiles(opts DeployOptions) error {
	if opts.AgentName == "" {
		return fmt.Errorf("AgentName 不能为空")
	}
	if opts.StartArgs == "" && opts.ConfigFile != "" {
		opts.StartArgs = "-config " + opts.ConfigFile
	}

	// zip 打包文件列表
	zipFiles := buildZipFileList(opts)

	files := map[string]string{
		"publish.bat":   genPublishBat(opts),
		"publish.sh":    genPublishSh(opts),
		"zip-files.bat": genZipFilesBat(opts, zipFiles),
		"zip-files.sh":  genZipFilesSh(opts, zipFiles),
	}

	var created []string
	for name, content := range files {
		if _, err := os.Stat(name); err == nil {
			fmt.Printf("  跳过（已存在）: %s\n", name)
			continue
		}
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			return fmt.Errorf("写入 %s 失败: %v", name, err)
		}
		created = append(created, name)
	}

	if len(created) > 0 {
		fmt.Printf("已生成部署脚本: %s\n", strings.Join(created, ", "))
	} else {
		fmt.Println("所有部署脚本已存在，未生成新文件")
	}
	return nil
}

func buildZipFileList(opts DeployOptions) string {
	var parts []string
	if opts.ConfigFile != "" {
		parts = append(parts, opts.ConfigFile)
	}
	parts = append(parts, opts.ZipExtras...)
	return strings.Join(parts, " ")
}

// ========================= publish.bat =========================

func genPublishBat(opts DeployOptions) string {
	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	sb.WriteString(fmt.Sprintf(":: %s 本机发布脚本\r\n", opts.AgentName))
	sb.WriteString(":: kill 旧进程 → 启动新进程\r\n\r\n")

	sb.WriteString(fmt.Sprintf("echo 停止 %s...\r\n", opts.AgentName))

	if opts.UsePIDFile {
		sb.WriteString(fmt.Sprintf("\r\nif exist %s.pid (\r\n", opts.AgentName))
		sb.WriteString(fmt.Sprintf("    set /p PID=<%s.pid\r\n", opts.AgentName))
		sb.WriteString("    echo 通过 PID 文件终止进程树: PID=%%PID%%\r\n")
		sb.WriteString("    taskkill /F /T /PID %%PID%% 2>nul\r\n")
		sb.WriteString(fmt.Sprintf("    del /f %s.pid 2>nul\r\n", opts.AgentName))
		sb.WriteString(")\r\n")
	}

	sb.WriteString(fmt.Sprintf("taskkill /F /IM %s.exe 2>nul\r\n\r\n", opts.AgentName))

	startCmd := fmt.Sprintf("%s.exe", opts.AgentName)
	if opts.StartArgs != "" {
		startCmd += " " + opts.StartArgs
	}
	sb.WriteString(fmt.Sprintf("echo 启动 %s...\r\n", opts.AgentName))
	sb.WriteString(fmt.Sprintf("start \"%s\" cmd /c \"%s\"\r\n\r\n", opts.AgentName, startCmd))

	sb.WriteString("ping -n 3 127.0.0.1 >nul\r\n\r\n")

	sb.WriteString(fmt.Sprintf("tasklist /FI \"IMAGENAME eq %s.exe\" 2>nul | find /I \"%s.exe\" >nul\r\n", opts.AgentName, opts.AgentName))
	sb.WriteString("if %errorlevel%==0 (\r\n")
	sb.WriteString(fmt.Sprintf("    echo %s 启动成功\r\n", opts.AgentName))
	sb.WriteString(") else (\r\n")
	sb.WriteString(fmt.Sprintf("    echo %s 启动失败，请检查新窗口中的输出\r\n", opts.AgentName))
	sb.WriteString("    exit /b 1\r\n")
	sb.WriteString(")\r\n")

	return sb.String()
}

// ========================= publish.sh =========================

func genPublishSh(opts DeployOptions) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString(fmt.Sprintf("# %s 发布脚本\n", opts.AgentName))
	sb.WriteString("cd \"$(dirname \"$0\")\"\n\n")

	sb.WriteString(fmt.Sprintf("echo \"停止 %s...\"\n", opts.AgentName))

	if opts.UsePIDFile {
		sb.WriteString(fmt.Sprintf("\nif [ -f %s.pid ]; then\n", opts.AgentName))
		sb.WriteString(fmt.Sprintf("    OLD_PID=$(cat %s.pid)\n", opts.AgentName))
		sb.WriteString("    echo \"通过 PID 文件终止进程组: PID=$OLD_PID\"\n")
		sb.WriteString("    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true\n")
		sb.WriteString(fmt.Sprintf("    rm -f %s.pid\n", opts.AgentName))
		sb.WriteString("fi\n")
	}

	sb.WriteString(fmt.Sprintf("pkill -f '\\./%s' 2>/dev/null || true\n", opts.AgentName))
	sb.WriteString("sleep 1\n\n")

	sb.WriteString(fmt.Sprintf("chmod +x %s\n\n", opts.AgentName))

	startCmd := fmt.Sprintf("./%s", opts.AgentName)
	if opts.StartArgs != "" {
		startCmd += " " + opts.StartArgs
	}
	sb.WriteString(fmt.Sprintf("echo \"启动 %s...\"\n", opts.AgentName))
	sb.WriteString(fmt.Sprintf("nohup %s > %s.log 2>&1 < /dev/null &\n", startCmd, opts.AgentName))
	sb.WriteString("disown\n\n")

	sb.WriteString("sleep 1\n")
	sb.WriteString(fmt.Sprintf("if pgrep -f '\\./%s' > /dev/null; then\n", opts.AgentName))
	sb.WriteString(fmt.Sprintf("    echo \"%s 启动成功 (PID: $(pgrep -f '\\./%s'))\"\n", opts.AgentName, opts.AgentName))
	sb.WriteString("else\n")
	sb.WriteString(fmt.Sprintf("    echo \"%s 启动失败，查看日志:\"\n", opts.AgentName))
	sb.WriteString(fmt.Sprintf("    tail -20 %s.log\n", opts.AgentName))
	sb.WriteString("    exit 1\n")
	sb.WriteString("fi\n")

	return sb.String()
}

// ========================= zip-files.bat =========================

func genZipFilesBat(opts DeployOptions, zipFiles string) string {
	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	sb.WriteString("setlocal enabledelayedexpansion\r\n\r\n")

	sb.WriteString("del /q *.zip 2>nul\r\n\r\n")

	sb.WriteString(":: 获取时间戳\r\n")
	sb.WriteString("for /f %%a in ('powershell -command \"Get-Date -Format \\\"yyyy-MM-dd-HH_mm_ss\\\"\"') do (\r\n")
	sb.WriteString("    set TIMESTAMP=%%a\r\n")
	sb.WriteString(")\r\n\r\n")

	sb.WriteString(fmt.Sprintf("set OUTPUT=%s_%%TIMESTAMP%%.zip\r\n", opts.AgentName))
	sb.WriteString("set SEVENZIP=\"C:\\Program Files\\7-Zip\\7z.exe\"\r\n\r\n")

	sb.WriteString(":: 根据目标平台决定二进制扩展名（交叉编译时 GOOS 由 deploy-agent 设置）\r\n")
	sb.WriteString("set EXT=.exe\r\n")
	sb.WriteString("if defined GOOS (\r\n")
	sb.WriteString("    if not \"%GOOS%\"==\"windows\" set EXT=\r\n")
	sb.WriteString(")\r\n")
	sb.WriteString(fmt.Sprintf("set BINNAME=%s%%EXT%%\r\n\r\n", opts.AgentName))

	sb.WriteString(fmt.Sprintf("taskkill /f /im %s.exe >nul 2>&1\r\n", opts.AgentName))
	sb.WriteString("go build -o %BINNAME%\r\n")
	sb.WriteString("if errorlevel 1 (\r\n")
	sb.WriteString("    echo 编译失败\r\n")
	sb.WriteString("    exit /b 1\r\n")
	sb.WriteString(")\r\n\r\n")

	sb.WriteString(":: 打包二进制 + 配置\r\n")
	sb.WriteString(fmt.Sprintf("%%SEVENZIP%% a -tzip \"%%OUTPUT%%\" %%BINNAME%% %s\r\n\r\n", zipFiles))

	sb.WriteString(":: 清理编译产物\r\n")
	sb.WriteString("del %BINNAME%\r\n\r\n")

	sb.WriteString("echo 成功生成: %OUTPUT%\r\n")

	return sb.String()
}

// ========================= zip-files.sh =========================

func genZipFilesSh(opts DeployOptions, zipFiles string) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("set -e\n\n")

	sb.WriteString("# 获取时间戳\n")
	sb.WriteString(fmt.Sprintf("TIMESTAMP=$(date +\"%%Y-%%m-%%d-%%H_%%M_%%S\")\n"))
	sb.WriteString(fmt.Sprintf("OUTPUT=\"%s_${TIMESTAMP}.zip\"\n\n", opts.AgentName))

	sb.WriteString("# 交叉编译支持：deploy-agent 会在需要时设置 GOOS/GOARCH\n")
	sb.WriteString("if [ -z \"$GOOS\" ]; then\n")
	sb.WriteString("    export GOOS=$(go env GOOS)\n")
	sb.WriteString("    export GOARCH=$(go env GOARCH)\n")
	sb.WriteString("fi\n")
	sb.WriteString("export CGO_ENABLED=0\n\n")

	sb.WriteString("EXT=\"\"\n")
	sb.WriteString("[ \"$GOOS\" = \"windows\" ] && EXT=\".exe\"\n")
	sb.WriteString(fmt.Sprintf("BINNAME=\"%s${EXT}\"\n\n", opts.AgentName))

	sb.WriteString(fmt.Sprintf("echo \"正在编译 %s (${GOOS}/${GOARCH})...\"\n", opts.AgentName))
	sb.WriteString("go build -o \"$BINNAME\" .\n")
	sb.WriteString("if [ $? -ne 0 ]; then\n")
	sb.WriteString("    echo \"编译失败\"\n")
	sb.WriteString("    exit 1\n")
	sb.WriteString("fi\n\n")

	sb.WriteString("# 打包二进制 + 配置\n")
	sb.WriteString(fmt.Sprintf("zip -r \"${OUTPUT}\" \"$BINNAME\" %s\n\n", zipFiles))

	sb.WriteString("# 清理编译产物\n")
	sb.WriteString("rm -f \"$BINNAME\"\n\n")

	sb.WriteString("echo \"成功生成: ${OUTPUT}\"\n")

	return sb.String()
}
