@echo off
setlocal enabledelayedexpansion

del /q *.zip 2>nul

:: 关闭运行中的进程，清理过往zip
taskkill /f /im llm-mcp-agent.exe >nul 2>&1
del /q *.zip >nul 2>&1

:: 获取时间戳
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=llm-mcp-agent_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

go build -o llm-mcp-agent.exe
if errorlevel 1 (
    echo 编译失败
    exit /b 1
)

:: 打包二进制 + 配置
%SEVENZIP% a -tzip "%OUTPUT%" llm-mcp-agent.exe llm-mcp-agent.json

:: 清理编译产物
del llm-mcp-agent.exe

echo 成功生成: %OUTPUT%
