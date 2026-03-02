@echo off
:: codegen-agent 本机发布脚本
:: kill 旧进程 → 启动新进程
:: 注意：timeout 在非交互式环境(deploy-agent子进程)下会报"不支持输入重新定向"，用 ping 替代

echo 停止 codegen-agent...
taskkill /F /IM codegen-agent.exe 2>nul

echo 启动 codegen-agent...
start "codegen-agent" cmd /c "codegen-agent.exe -config agent.conf"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq codegen-agent.exe" 2>nul | find /I "codegen-agent.exe" >nul
if %errorlevel%==0 (
    echo codegen-agent 启动成功
) else (
    echo codegen-agent 启动失败，请检查新窗口中的输出
    exit /b 1
)
