@echo off
:: cmd-agent 本机发布脚本
:: kill 旧进程 → 启动新进程

echo 停止 cmd-agent...

if exist cmd-agent.pid (
    set /p PID=<cmd-agent.pid
    echo 通过 PID 文件终止进程树: PID=%%PID%%
    taskkill /F /T /PID %%PID%% 2>nul
    del /f cmd-agent.pid 2>nul
)
taskkill /F /IM cmd-agent.exe 2>nul

echo 启动 cmd-agent...
start "cmd-agent" cmd /c "cmd-agent.exe -config cmd-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq cmd-agent.exe" 2>nul | find /I "cmd-agent.exe" >nul
if %errorlevel%==0 (
    echo cmd-agent 启动成功
) else (
    echo cmd-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
