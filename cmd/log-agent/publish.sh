#!/bin/bash
# log-agent 发布脚本
# 在远程服务器上执行：kill旧进程 → 启动新进程
cd "$(dirname "$0")"
svr="$(pwd)/log-agent"
echo $svr

# 停止旧进程
echo "停止 log-agent..."
ps aux | grep "$svr" | grep -v "grep" | awk '{print $2}' | xargs kill -9 2>/dev/null
sleep 1

# 确保可执行
chmod +x log-agent

# 启动新进程
echo "启动 log-agent..."
nohup "$svr" -config log-agent.json > log-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f "$svr" > /dev/null; then
    echo "log-agent 启动成功 (PID: $(pgrep -f "$svr"))"
else
    echo "log-agent 启动失败，查看日志:"
    tail -20 log-agent.log
    exit 1
fi
