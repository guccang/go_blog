@echo off
:: log-agent 本机发布脚本
:: kill 旧进程 → 启动新进程

echo 停止 log-agent...
taskkill /F /IM log-agent.exe 2>nul

echo 启动 log-agent...
start "log-agent" cmd /c "log-agent.exe -config log-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq log-agent.exe" 2>nul | find /I "log-agent.exe" >nul
if %errorlevel%==0 (
    echo log-agent 启动成功
) else (
    echo log-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
