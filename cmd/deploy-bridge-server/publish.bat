@echo off
:: deploy-bridge-server 本机发布脚本
:: kill 旧进程 → 启动新进程

echo 停止 deploy-bridge-server...
taskkill /F /IM deploy-bridge-server.exe 2>nul

echo 启动 deploy-bridge-server...
start "deploy-bridge-server" cmd /c "deploy-bridge-server.exe bridge-server.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq deploy-bridge-server.exe" 2>nul | find /I "deploy-bridge-server.exe" >nul
if %errorlevel%==0 (
    echo deploy-bridge-server 启动成功
) else (
    echo deploy-bridge-server 启动失败，请检查新窗口中的输出
    exit /b 1
)
