@echo off
:: deploy-agent local publish script
:: kill old process -> start new process
:: Note: use ping instead of timeout (timeout fails in non-interactive mode)

echo Stopping deploy-agent...
taskkill /F /IM deploy-agent.exe 2>nul

echo Starting deploy-agent...
start cmd /c "deploy-agent.exe"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq deploy-agent.exe" 2>nul | find /I "deploy-agent.exe" >nul
if %errorlevel%==0 (
    echo deploy-agent started successfully
) else (
    echo deploy-agent failed to start
    exit /b 1
)
