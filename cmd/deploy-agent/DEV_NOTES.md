# Deploy Agent 工程实现注意细节

本文档记录 `deploy-agent` 项目在开发和维护过程中的一些关键工程实现细节和踩坑记录，方便后续维护。

## 1. CLI 模式与 Daemon (守护进程) 模式的兼容

`deploy-agent` 支持通过 WebSocket 作为长连接 Agent 运行（Daemon 模式），也支持直接作为命令行工具运行（CLI 模式）。

**实现细节**：
在 `cmd/deploy-agent/main.go` 中，即使配置文件 `deploy.conf` 中填写了 `server_url`（即启用了 Daemon 模式），应用也需要判断用户是否显式传入了部署参数（如 `--project`、`--target` 或 `--pack-only`）。如果传入了这些参数，说明用户的意图是单次本地执行发布任务，此时必须**跳过 Daemon 模式**的启动逻辑，直接进入 CLI 部署流程。

```go
// 是否强制使用 CLI 模式（当用户显式指定了项目、目标或只打包时）
isCliMode := *projectName != "" || *targetName != "" || *packOnly

// daemon 模式（WebSocket）
if cfg.ServerURL != "" && !isCliMode {
    // 启动长连接...
}
```

## 2. Windows 下本地脚本执行 (新窗口运行)

当 `deploy-agent` 进行本地部署（Local Deploy）并需要执行目标目录下的发布脚本（`RemoteScript`）时，如果是在 Windows 下执行 `.bat` 或 `.cmd` 脚本，默认的 `cmd /c script.bat` 会在后台静默执行，导致无法直观看到持续运行型服务（如 `codegen-agent`、`gateway`）的启动日志。

**实现细节**：
在 `cmd/deploy-agent/deploy.go` 的 `runLocalScript` 方法中，针对 Windows 批处理脚本，我们通过 `start` 命令来新开一个 CMD 窗口执行脚本。

**踩坑记录（`start` 命令的引号解析 Bug）**：
Windows 的 `start` 命令对双引号 `"` 非常敏感。如果使用 `cmd /c start "WindowTitle" cmd /c script` 或 `cmd /c start "" cmd /c script`，在某些 Go 的 `exec.Command` 参数转义场景下，极易引发 **“系统找不到文件 Deploy”** 或 **“拒绝访问 (Access Denied)”** 的错误。

**正确做法**：
完全舍弃 title 参数，直接使用最简形式的 `cmd /c start cmd /c script.bat`，即可完美规避引号转义带来的各种执行异常。

```go
func (d *Deployer) runLocalScript(scriptPath, workDir string) error {
	if strings.HasSuffix(scriptPath, ".bat") || strings.HasSuffix(scriptPath, ".cmd") {
		// 注意这里不要加额外的 "" 或 title 参数，避免引发 Windows 的引号解析 Bug
		return d.runLocalCmd("cmd", []string{"/c", "start", "cmd", "/c", scriptPath}, workDir)
	}
    // ...
}
```

## 3. 本地打包 (Pack) 时的文件占用冲突

在实现了“新窗口运行”服务后，旧的服务进程会在独立的命令行窗口中持续运行。此时，如果重新触发打包（如 `codegen-agent` 的 `zip-files.bat` 执行 `go build`），因为旧的 `.exe` 可执行文件正被前一个窗口的进程占用，会导致 **“拒绝访问 (Access Denied)”** 编译失败。

**实现细节**：
在项目的打包脚本（如 `zip-files.bat`）中，在执行编译（`go build`）之前，需增加终止相关进程的命令，强制释放被占用的二进制文件。

```bat
:: 关闭运行中的实例以防文件被占用报错拒绝访问
taskkill /f /im codegen-agent.exe >nul 2>&1
go build -o codegen-agent.exe
```

## 4. publish.bat 脚本的中文乱码引发执行崩溃 (闪退)

当用户自建发布脚本（如 `publish.bat`）或者系统生成脚本并包含 UTF-8 编码的中文注释时：
```bat
:: go_blog 本机发布脚本
taskkill /F /IM go_blog.exe 2>nul
```
如果在默认的 Windows GBK 编码环境 (`cmd.exe`) 中运行，中文字符的多字节序列会与换行符或空格发生截断解析冲突。这会导致 `cmd.exe` 将本来合法的命令错认为非法字符串，甚至吞掉下一行的有效指令（例如把 `taskkill` 吃掉一半变成 `IM go_blog.exe 2>nul`），从而抛出“不是内部或外部命令”并直接闪退！

**正确做法**：
1. **纯英文/ASCII 化**：在提供或编写供 `cmd.exe` 执行的发布脚本时，**尽量全篇使用纯英文的注释和提示 (ASCII字符)**，彻底避开由于编辑器保存的 UTF-8 编码或平台环境引起的乱码与崩溃风险。
2. 或手动将 `.bat` 文件另存为 ANSI (GB2312/GBK) 编码。
