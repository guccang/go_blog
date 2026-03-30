@echo off
echo Stopping app-agent...
taskkill /F /IM app-agent.exe 2>nul

echo Starting app-agent...
start "app-agent" cmd /c "app-agent.exe -config app-agent.json"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq app-agent.exe" 2>nul | find /I "app-agent.exe" >nul
if %errorlevel%==0 (
    echo app-agent started
) else (
    echo app-agent failed to start
    exit /b 1
)
