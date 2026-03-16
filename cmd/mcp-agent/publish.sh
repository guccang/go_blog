#!/bin/bash
# mcp-agent 发布脚本
# 在远程服务器上执行：kill旧进程 → 启动新进程
cd "$(dirname "$0")"
svr="$(pwd)/mcp-agent"
echo $svr

# 停止旧进程
echo "停止 mcp-agent..."
ps aux | grep "$svr" | grep -v "grep" | awk '{print $2}' | xargs kill -9 2>/dev/null
sleep 1

# 确保可执行
chmod +x mcp-agent

# 启动新进程
echo "启动 mcp-agent..."
nohup "$svr" -config mcp-agent.json > mcp-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f "$svr" > /dev/null; then
    echo "mcp-agent 启动成功 (PID: $(pgrep -f "$svr"))"
else
    echo "mcp-agent 启动失败，查看日志:"
    tail -20 mcp-agent.log
    exit 1
fi
