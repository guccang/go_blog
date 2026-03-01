@echo off
:: codegen-agent 本机发布脚本
:: kill 旧进程 → 启动新进程

echo 停止 codegen-agent...
taskkill /F /IM codegen-agent.exe 2>nul
timeout /t 2 /nobreak >nul

echo 启动 codegen-agent...
start "" /B cmd /c "codegen-agent.exe -config agent.conf >codegen-agent.log 2>&1"
timeout /t 2 /nobreak >nul

tasklist /FI "IMAGENAME eq codegen-agent.exe" 2>nul | find /I "codegen-agent.exe" >nul
if %errorlevel%==0 (
    echo codegen-agent 启动成功
) else (
    echo codegen-agent 启动失败
    type codegen-agent.log
    exit /b 1
)
