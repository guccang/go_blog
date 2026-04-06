@echo off
setlocal
cd /d %~dp0

echo stopping audio-agent...
if exist audio-agent.pid (
  set /p OLD_PID=<audio-agent.pid
  taskkill /F /T /PID %OLD_PID% >nul 2>nul
  del /q audio-agent.pid
)
taskkill /F /IM audio-agent.exe >nul 2>nul

echo starting audio-agent...
start "" /b audio-agent.exe -config audio-agent.json > audio-agent.log 2>&1
