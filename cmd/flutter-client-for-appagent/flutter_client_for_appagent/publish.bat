@echo off
setlocal enabledelayedexpansion

cd /d "%~dp0"

:: Kill any existing process on port 8883
echo Stopping existing web server on port 8883...
for /f "tokens=5" %%a in ('netstat -ano ^| findstr :8883 ^| findstr LISTENING') do (
    taskkill /F /PID %%a 2>nul
)

:: Extract the web files if not already extracted
for %%f in (flutter-web_*.zip) do (
    if exist "%%f" (
        echo Extracting Flutter web files...
        powershell -command "Expand-Archive -Path '%%f' -DestinationPath 'build/web' -Force"
    )
)

:: Start Python static server on port 8883
echo Starting Flutter web server on port 8883...
cd build\web
start /B cmd /c "python -m http.server 8883 > ..\..\flutter-web.log 2>&1"

timeout /t 2 /nobreak >nul

:: Verify server is running
netstat -ano | findstr ":8883" | findstr LISTENING >nul
if !errorlevel! equ 0 (
    echo Flutter web server started on port 8883
) else (
    echo Failed to start server, check flutter-web.log
    type flutter-web.log
    exit /b 1
)
