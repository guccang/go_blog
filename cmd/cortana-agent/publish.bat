@echo off
:: cortana-agent 本机发布脚本
:: kill 旧进程 → 启动新进程

echo 停止 cortana-agent...
taskkill /F /IM cortana-agent.exe 2>nul

echo 启动 cortana-agent...
start "cortana-agent" cmd /c "cortana-agent.exe -config cortana-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq cortana-agent.exe" 2>nul | find /I "cortana-agent.exe" >nul
if %errorlevel%==0 (
    echo cortana-agent 启动成功
) else (
    echo cortana-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
