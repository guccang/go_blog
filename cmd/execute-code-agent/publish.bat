@echo off
:: execute-code-agent 本机发布脚本
:: kill 旧进程（含子进程树） → 启动新进程
:: 注意：timeout 在非交互式环境(deploy-agent子进程)下会报"不支持输入重新定向"，用 ping 替代

echo 停止 execute-code-agent...

:: 优先通过 PID 文件杀进程树（/T 杀子进程，如 claude.exe）
if exist execute-code-agent.pid (
    set /p PID=<execute-code-agent.pid
    echo 通过 PID 文件终止进程树: PID=%PID%
    taskkill /F /T /PID %PID% 2>nul
    del /f execute-code-agent.pid 2>nul
)
:: 兜底：按进程名杀残留
taskkill /F /IM execute-code-agent.exe 2>nul

echo 启动 execute-code-agent...
start "execute-code-agent" cmd /c "execute-code-agent.exe"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq execute-code-agent.exe" 2>nul | find /I "execute-code-agent.exe" >nul
if %errorlevel%==0 (
    echo execute-code-agent 启动成功
) else (
    echo execute-code-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
