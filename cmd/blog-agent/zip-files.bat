@echo off
setlocal enabledelayedexpansion

del /q *.zip 2>nul

:: 关闭运行中的进程，清理过往zip
taskkill /f /im blog-agent.exe >nul 2>&1
del /q *.zip >nul 2>&1

:: 获取时间戳
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=blog-agent_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

:: 根据目标平台决定二进制扩展名（交叉编译时 GOOS 由 deploy-agent 设置）
set EXT=.exe
if defined GOOS (
    if not "%GOOS%"=="windows" set EXT=
)
set BINNAME=blog-agent%EXT%

go build -o %BINNAME%
if errorlevel 1 (
    echo 编译失败
    exit /b 1
)

:: 打包二进制 + 配置
%SEVENZIP% a -tzip "%OUTPUT%" publish.sh pkgs/ statics/ templates/ ./main.go ./go.mod ./go.sum
 
:: 清理编译产物
del %BINNAME%

echo 成功生成: %OUTPUT%

