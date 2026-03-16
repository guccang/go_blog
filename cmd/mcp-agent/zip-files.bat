@echo off
setlocal enabledelayedexpansion

del /q *.zip 2>nul

for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=mcp-agent_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

set EXT=.exe
if defined GOOS (
    if not "%GOOS%"=="windows" set EXT=
)
set BINNAME=mcp-agent%EXT%

taskkill /f /im mcp-agent.exe >nul 2>&1
go build -o %BINNAME%
if errorlevel 1 (
    echo 编译失败
    exit /b 1
)

%SEVENZIP% a -tzip "%OUTPUT%" %BINNAME% mcp-agent.json publish.sh

del %BINNAME%

echo 成功生成: %OUTPUT%
