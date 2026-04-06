@echo off
setlocal
cd /d %~dp0

echo stopping image-agent...
if exist image-agent.pid (
  set /p OLD_PID=<image-agent.pid
  taskkill /F /T /PID %OLD_PID% >nul 2>nul
  del /q image-agent.pid
)
taskkill /F /IM image-agent.exe >nul 2>nul

echo starting image-agent...
start "" /b image-agent.exe -config image-agent.json > image-agent.log 2>&1
