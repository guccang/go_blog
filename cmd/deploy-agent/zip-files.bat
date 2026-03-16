@echo off
setlocal enabledelayedexpansion

taskkill /F /IM deploy-agent.exe 2>nul

del /q *.zip 2>nul

:: Get timestamp
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=deploy-agent_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

:: Cross-compilation: GOOS set by deploy-agent
set EXT=.exe
if defined GOOS (
    if not "%GOOS%"=="windows" set EXT=
)
set BINNAME=deploy-agent%EXT%

:: Clean old build artifacts
del deploy-agent.exe 2>nul
del deploy-agent 2>nul

go build -o %BINNAME%
if errorlevel 1 (
    echo Build failed
    exit /b 1
)

:: Package binary + config
%SEVENZIP% a -tzip "%OUTPUT%" %BINNAME% publish.sh publish.bat deploy.conf settings/

:: Clean build artifacts
del %BINNAME%

echo Generated: %OUTPUT%
