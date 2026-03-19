@echo off
REM deploy-bridge-server Windows 打包脚本

set APP=deploy-bridge-server
for /f "tokens=1-5 delims=/ " %%a in ('date /t') do set D=%%a%%b%%c
for /f "tokens=1-2 delims=:." %%a in ('time /t') do set T=%%a%%b
set ZIP_NAME=%APP%_%D%_%T%.zip

echo === 打包 %APP% ===

REM 交叉编译 Linux 版本
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -o %APP% .
if errorlevel 1 (
    echo 编译失败
    exit /b 1
)

REM 打包
7z a %ZIP_NAME% %APP% publish.sh bridge-server.json
echo === 打包完成: %ZIP_NAME% ===
