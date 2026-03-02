#!/bin/bash
for proc in gateway codegen-agent deploy-agent wechat-agent llm-mcp-agent go_blog; do
    pkill -f "${proc}" 2>/dev/null && echo "${proc} stopped" || echo "${proc} not running"
done
