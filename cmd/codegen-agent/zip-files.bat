@echo off
setlocal enabledelayedexpansion

:: 获取时间戳
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=codegen_agent_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

:: 交叉编译 Linux amd64
echo 正在编译 codegen-agent (linux/amd64)...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -o codegen-agent .
if errorlevel 1 (
    echo 编译失败
    exit /b 1
)

:: 打包二进制 + 配置
%SEVENZIP% a -tzip "%OUTPUT%" codegen-agent agent.conf.example settings\

:: 清理编译产物
del codegen-agent

echo 成功生成: %OUTPUT%
