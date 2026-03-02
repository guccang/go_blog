@echo off
taskkill /F /IM gateway.exe
taskkill /F /IM codegen-agent.exe
taskkill /F /IM deploy-agent.exe
taskkill /F /IM wechat-agent.exe
taskkill /F /IM llm-mcp-agent.exe
taskkill /F /IM go_blog.exe