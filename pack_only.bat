@echo off
setlocal

:: 仅打包 go_blog 项目文件（不上传）
:: 供 deploy-agent 调用，上传/解压由 deploy-agent 编排

:: 使用 PowerShell 获取日期（兼容性更好）
for /f %%A in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "DATE=%%A"

:: Output filename
set "OUTPUT=go_blog_%DATE%.zip"

:: Files to archive
set "FILES=pkgs templates statics main.go go.mod go.sum"

:: Remove old zip if exists
if exist "%OUTPUT%" (
    echo Deleting existing %OUTPUT%
    del "%OUTPUT%"
)

:: Create zip archive (7z from PATH)
echo Packing to %OUTPUT% ...
7z a -tzip "%OUTPUT%" %FILES% -xr!*.DS_Store -xr!__pycache__ -xr!*.pyc

if %ERRORLEVEL% EQU 0 (
    echo Packing complete: %OUTPUT%
) else (
    echo Packing failed
    exit /b 1
)

endlocal
