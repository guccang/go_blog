@echo off
chcp 65001 > nul
setlocal enabledelayedexpansion

echo ========================================
echo     开始编译 goblog 所有进程
echo ========================================
echo.

:: 设置输出目录
set OUTPUT_DIR=.\
if not exist %OUTPUT_DIR% mkdir %OUTPUT_DIR%

:: 清理旧的编译文件
echo 清理旧文件...
if exist %OUTPUT_DIR%\*.exe del /q %OUTPUT_DIR%\*.exe 2>nul

:: 设置变量
set BUILD_TIME=%date% %time%
set BUILD_USER=%username%
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

:: 显示配置信息
echo [配置信息]
echo 输出目录: %OUTPUT_DIR%
echo 编译时间: %BUILD_TIME%
echo 编译用户: %BUILD_USER%
echo 目标平台: %GOOS%/%GOARCH%
echo.

echo [开始编译]
echo --------------------------------

:: 设置编译成功计数
set SUCCESS_COUNT=0
set FAIL_COUNT=0

go build 

:: 编译 codegen-agent
echo 正在编译 codegen-agent...
cd .\cmd\codegen-agent
go build -ldflags="-s -w" -o codegen-agent.exe
if !errorlevel! equ 0 (
    set /a SUCCESS_COUNT+=1
    echo   ✓ codegen-agent 编译成功
) else (
    set /a FAIL_COUNT+=1
    echo   ✗ codegen-agent 编译失败
)
cd ..\..

:: 编译 deploy-agent
echo 正在编译 deploy-agent...
cd .\cmd\deploy-agent
go build -ldflags="-s -w" -o deploy-agent.exe
if !errorlevel! equ 0 (
    set /a SUCCESS_COUNT+=1
    echo   ✓ deploy-agent 编译成功
) else (
    set /a FAIL_COUNT+=1
    echo   ✗ deploy-agent 编译失败
)
cd ..\..

:: 编译 gateway
echo 正在编译 gateway...
cd .\cmd\gateway
go build -ldflags="-s -w" -o gateway.exe
if !errorlevel! equ 0 (
    set /a SUCCESS_COUNT+=1
    echo   ✓ gateway 编译成功
) else (
    set /a FAIL_COUNT+=1
    echo   ✗ gateway 编译失败
)
cd ..\..

:: 编译 wechat-agent
echo 正在编译 wechat-agent...
cd .\cmd\wechat-agent
go build -ldflags="-s -w" -o wechat-agent.exe
if !errorlevel! equ 0 (
    set /a SUCCESS_COUNT+=1
    echo   ✓ wechat-agent 编译成功
) else (
    set /a FAIL_COUNT+=1
    echo   ✗ wechat-agent 编译失败
)
cd ..\..

:: 编译 llm
echo 正在编译 llm
cd .\cmd\llm-mcp-agent
go build -ldflags="-s -w"
if !errorlevel! equ 0 (
    set /a SUCCESS_COUNT+=1
    echo   ✓ llm 编译成功
) else (
    set /a FAIL_COUNT+=1
    echo   ✗ llm  编译失败
)
cd ..\..

echo --------------------------------
echo.

:: 显示编译结果
echo [编译结果]
echo 成功: %SUCCESS_COUNT% / 4
echo 失败: %FAIL_COUNT% / 4
echo.

:: 如果全部成功，显示文件信息
if %FAIL_COUNT% equ 0 (
    echo [生成文件]
    dir %OUTPUT_DIR% /b
    
    echo.
    echo ✓ 所有进程编译成功！
) else (
    echo.
    echo ✗ 编译过程中出现错误，请检查日志
)

echo.
echo ========================================
pause