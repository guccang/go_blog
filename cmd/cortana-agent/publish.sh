#!/bin/bash
# cortana-agent 发布脚本
cd "$(dirname "$0")"

echo "停止 cortana-agent..."
pkill -f '\./cortana-agent' 2>/dev/null || true
sleep 1

chmod +x cortana-agent

echo "启动 cortana-agent..."
nohup ./cortana-agent -config cortana-agent.json > cortana-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f '\./cortana-agent' > /dev/null; then
    echo "cortana-agent 启动成功 (PID: $(pgrep -f '\./cortana-agent'))"
else
    echo "cortana-agent 启动失败，查看日志:"
    tail -20 cortana-agent.log
    exit 1
fi
