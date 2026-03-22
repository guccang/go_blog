@echo off
:: env-agent 本机发布脚本
:: kill 旧进程（含子进程树） → 启动新进程

echo 停止 env-agent...

if exist env-agent.pid (
    set /p PID=<env-agent.pid
    echo 通过 PID 文件终止进程树: PID=%PID%
    taskkill /F /T /PID %PID% 2>nul
    del /f env-agent.pid 2>nul
)
taskkill /F /IM env-agent.exe 2>nul

echo 启动 env-agent...
start "env-agent" cmd /c "env-agent.exe -config env-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq env-agent.exe" 2>nul | find /I "env-agent.exe" >nul
if %errorlevel%==0 (
    echo env-agent 启动成功
) else (
    echo env-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
