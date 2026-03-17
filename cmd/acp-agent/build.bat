@echo off
echo 构建 ACP Agent...
set GOPROXY=https://goproxy.cn,direct
if exist acp-agent.exe del acp-agent.exe
echo 下载依赖...
go mod download
echo 构建中...
go build -o acp-agent.exe .
if errorlevel 1 (
    echo 构建失败!
    pause
    exit /b 1
)
echo 构建成功!
echo 运行: acp-agent.exe
pause
