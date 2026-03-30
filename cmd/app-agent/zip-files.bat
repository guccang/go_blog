@echo off
setlocal enabledelayedexpansion

del /q *.zip 2>nul

for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=app-agent_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

set EXT=.exe
if defined GOOS (
    if not "%GOOS%"=="windows" set EXT=
)
set BINNAME=app-agent%EXT%

taskkill /f /im app-agent.exe >nul 2>&1
go build -o %BINNAME% .
if errorlevel 1 (
    echo build failed
    exit /b 1
)

%SEVENZIP% a -tzip "%OUTPUT%" %BINNAME% publish.sh app-agent.json

del %BINNAME%

echo generated: %OUTPUT%
