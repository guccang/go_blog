@echo off
cd /d %~dp0 

start "go_blog" cmd /c  "go_blog.exe %~dp0\blogs_txt\ztt\sys_conf.md"
start "gateway" cmd /c "cd /d %~dp0\cmd\gateway && gateway.exe"
start "codegen-agent" cmd /c "cd /d %~dp0\cmd\codegen-agent && codegen-agent.exe"
start "deploy-agent" cmd /c "cd /d %~dp0\cmd\deploy-agent && deploy-agent.exe"
start "wechat-agent" cmd /c "cd /d %~dp0\cmd\wechat-agent && wechat-agent.exe"
start "llm-mcp-agent" cmd /c "cd /d %~dp0\cmd\llm-mcp-agent && llm-mcp-agent.exe"
echo 所有服务已启动
