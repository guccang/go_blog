@echo off
setlocal enabledelayedexpansion
for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyyy-MM-dd-HH_mm_ss"') do set TS=%%i
set OUTPUT=obs-agent_%TS%.zip
set BINNAME=obs-agent.exe

go build -o %BINNAME% .
powershell -NoProfile -Command "Compress-Archive -Force -Path '%BINNAME%','obs-agent.json','publish.bat' -DestinationPath '%OUTPUT%'"
del /q %BINNAME%
echo generated %OUTPUT%
