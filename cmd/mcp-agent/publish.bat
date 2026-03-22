@echo off
:: mcp-agent 本机发布脚本
:: kill 旧进程 → 启动新进程

echo 停止 mcp-agent...
taskkill /F /IM mcp-agent.exe 2>nul

echo 启动 mcp-agent...
start "mcp-agent" cmd /c "mcp-agent.exe -config mcp-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq mcp-agent.exe" 2>nul | find /I "mcp-agent.exe" >nul
if %errorlevel%==0 (
    echo mcp-agent 启动成功
) else (
    echo mcp-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
