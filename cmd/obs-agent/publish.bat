@echo off
cd /d %~dp0
taskkill /F /IM obs-agent.exe >nul 2>nul
start /B obs-agent.exe -config obs-agent.json > obs-agent.log 2>&1
