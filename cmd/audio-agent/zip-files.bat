@echo off
setlocal enabledelayedexpansion

for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyyy-MM-dd-HH_mm_ss"') do set TIMESTAMP=%%i
set OUTPUT=audio-agent_%TIMESTAMP%.zip

if "%GOOS%"=="" set GOOS=windows
if "%GOARCH%"=="" for /f %%i in ('go env GOARCH') do set GOARCH=%%i
set CGO_ENABLED=0

set BINNAME=audio-agent.exe

echo building audio-agent (%GOOS%/%GOARCH%)...
go build -o %BINNAME% .
if errorlevel 1 exit /b 1

powershell -NoProfile -Command "Compress-Archive -Path '%BINNAME%','audio-agent.json','publish.bat' -DestinationPath '%OUTPUT%' -Force"
del /q %BINNAME%
echo generated %OUTPUT%
