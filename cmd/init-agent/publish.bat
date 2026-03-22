@echo off
:: init-agent 本机发布脚本
:: kill 旧进程 → 启动新进程

echo 停止 init-agent...
taskkill /F /IM init-agent.exe 2>nul

echo 启动 init-agent...
start "init-agent" cmd /c "init-agent.exe -config init-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq init-agent.exe" 2>nul | find /I "init-agent.exe" >nul
if %errorlevel%==0 (
    echo init-agent 启动成功
) else (
    echo init-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
