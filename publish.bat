@echo off
:: go_blog local publish script
:: kill old process -> start new process
:: Note: timeout will report error in non-interactive environments, using ping instead.

echo stopping go_blog
taskkill /F /IM go_blog.exe 2>nul

echo starting go_blog
start "go_blog" cmd /c "go_blog.exe blogs_txt\ztt\sys_conf.md"

ping -n 3 127.0.0.1 >nul

tasklist /FI "IMAGENAME eq go_blog.exe" 2>nul | find /I "go_blog.exe" >nul
if %errorlevel%==0 (
    echo go_blog started successfully
) else (
    echo go_blog failed to start, please check the output in the new window
    exit /b 1
)