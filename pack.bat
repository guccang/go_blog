@echo off
setlocal

:: Get current date and time for filename
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value') do set datetime=%%I
set "DATE=%datetime:~0,8%_%datetime:~8,6%"

:: Output filename
set "OUTPUT=go_blog_%DATE%.zip"

:: Files to archive
set "FILES=pkgs templates statics main.go go.mod go.sum"

:: Check if 7z.exe exists
if not exist "7z.exe" (
    echo Error: 7z.exe not found in current directory.
    exit /b 1
)

:: Remove old zip if exists
if exist "%OUTPUT%" (
    echo Deleting existing %OUTPUT%
    del "%OUTPUT%"
)

:: Create zip archive
echo Packing to %OUTPUT% ...
:: -xr! excludes files recursively
.\7z.exe a -tzip "%OUTPUT%" %FILES% -xr!*.DS_Store -xr!__pycache__ -xr!*.pyc

if %ERRORLEVEL% EQU 0 (
    echo ✅ Packing complete: %OUTPUT%
) else (
    echo ❌ Packing failed
    exit /b 1
)

:: Upload and unzip
echo Uploading to server...
scp "%OUTPUT%" root@114.115.214.86:/data/program/go/go_blog
if %ERRORLEVEL% NEQ 0 (
    echo ❌ Upload failed
    exit /b 1
)

echo Unzipping on server...
ssh root@114.115.214.86 "cd /data/program/go/go_blog; unzip -o %OUTPUT%;"
if %ERRORLEVEL% NEQ 0 (
    echo ❌ Remote unzip failed
    exit /b 1
)

echo Done.
endlocal
