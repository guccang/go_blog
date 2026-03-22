@echo off
setlocal enabledelayedexpansion

del /q *.zip 2>nul

:: 获取时间戳
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=init-agent_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

:: 根据目标平台决定二进制扩展名（交叉编译时 GOOS 由 deploy-agent 设置）
set EXT=.exe
if defined GOOS (
    if not "%GOOS%"=="windows" set EXT=
)
set BINNAME=init-agent%EXT%

taskkill /f /im init-agent.exe >nul 2>&1
go build -o %BINNAME%
if errorlevel 1 (
    echo 编译失败
    exit /b 1
)

:: 打包二进制 + 配置
%SEVENZIP% a -tzip "%OUTPUT%" %BINNAME% init-agent.json publish.sh

:: 清理编译产物
del %BINNAME%

echo 成功生成: %OUTPUT%
