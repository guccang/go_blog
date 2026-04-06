@echo off
setlocal enabledelayedexpansion

cd /d "%~dp0"

echo Building Flutter web...
call flutter build web

set TIMESTAMP=%date:~0,4%-%date:~5,2%-%date:~8,2%-%time:~0,2%_%time:~3,2%_%time:~6,2%
set TIMESTAMP=%TIMESTAMP: =0%
set OUTPUT=flutter-web_%TIMESTAMP%.zip

echo Packaging build/web to %OUTPUT%...
cd build\web
powershell -command "Compress-Archive -Path * -DestinationPath '../../%OUTPUT%' -Force"

echo Generated: %OUTPUT%
