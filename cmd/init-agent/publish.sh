#!/bin/bash
# init-agent 发布脚本
cd "$(dirname "$0")"

echo "停止 init-agent..."
pkill -f '\./init-agent' 2>/dev/null || true
sleep 1

chmod +x init-agent

echo "启动 init-agent..."
nohup ./init-agent -config init-agent.json > init-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f '\./init-agent' > /dev/null; then
    echo "init-agent 启动成功 (PID: $(pgrep -f '\./init-agent'))"
else
    echo "init-agent 启动失败，查看日志:"
    tail -20 init-agent.log
    exit 1
fi
