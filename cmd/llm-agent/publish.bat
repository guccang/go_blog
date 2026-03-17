@echo off
:: llm-agent 本机发布脚本
:: kill 旧进程 → 启动新进程
:: 注意：timeout 在非交互式环境(deploy-agent子进程)下会报"不支持输入重新定向"，用 ping 替代

echo 停止 llm-agent...
taskkill /F /IM llm-agent.exe 2>nul

echo 启动 llm-agent...
start "llm-agent" cmd /c "llm-agent.exe --config llm-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq llm-agent.exe" 2>nul | find /I "llm-agent.exe" >nul
if %errorlevel%==0 (
    echo llm-agent 启动成功
) else (
    echo llm-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
