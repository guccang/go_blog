@echo off
setlocal enabledelayedexpansion

for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyyy-MM-dd-HH_mm_ss"') do set TIMESTAMP=%%i
set OUTPUT=image-agent_%TIMESTAMP%.zip

if "%GOOS%"=="" set GOOS=windows
if "%GOARCH%"=="" for /f %%i in ('go env GOARCH') do set GOARCH=%%i
set CGO_ENABLED=0

set BINNAME=image-agent.exe

echo building image-agent (%GOOS%/%GOARCH%)...
go build -o %BINNAME% .
if errorlevel 1 exit /b 1

powershell -NoProfile -Command "Compress-Archive -Path '%BINNAME%','image-agent.json','publish.bat' -DestinationPath '%OUTPUT%' -Force"
del /q %BINNAME%
echo generated %OUTPUT%
