#!/bin/bash
for proc in gateway codegen-agent deploy-agent wechat-agent llm-mcp-agent go_blog; do
    ps aux | grep "${proc}" | grep -v grep
done
