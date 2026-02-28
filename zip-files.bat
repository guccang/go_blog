

@echo off
setlocal enabledelayedexpansion

:: 获取当前时间，使用 PowerShell 提取精确格式
for /f %%a in ('powershell -command "Get-Date -Format \"yyyy-MM-dd-HH_mm_ss\""') do (
    set TIMESTAMP=%%a
)

:: 压缩文件名
set OUTPUT=go_blog_%TIMESTAMP%.zip

:: 7z 路径（修改为你本机安装路径）
set SEVENZIP="C:\Program Files\7-Zip\7z.exe"

:: 要打包的文件夹
set FOLDERS=pkgs statics/css statics/js templates  ./main.go ./go.mod

:: 执行压缩
%SEVENZIP% a -tzip "%OUTPUT%" %FOLDERS%

echo 成功生成压缩文件：%OUTPUT%
:: pause
