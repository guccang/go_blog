@echo off
setlocal enabledelayedexpansion

taskkill /F /IM gateway.exe 2>nul

del /q *.zip 2>nul

:: 获取时间戳
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

set OUTPUT=gateway_%TIMESTAMP%.zip
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

:: 清理编译产物
del gateway.exe

go build -o gateway.exe
if errorlevel 1 (
    echo 编译失败
    exit /b 1
)

:: 打包二进制 + 配置
%SEVENZIP% a -tzip "%OUTPUT%" gateway.exe gateway.json

:: 清理编译产物
del gateway.exe

echo 成功生成: %OUTPUT%
