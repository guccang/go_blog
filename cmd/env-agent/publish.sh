#!/bin/bash
# env-agent 发布脚本
# 在远程服务器上执行：kill旧进程 → 启动新进程

cd "$(dirname "$0")"

# 停止旧进程
echo "停止 env-agent..."

if [ -f env-agent.pid ]; then
    OLD_PID=$(cat env-agent.pid)
    echo "通过 PID 文件终止进程组: PID=$OLD_PID"
    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true
    rm -f env-agent.pid
fi
pkill -f '\./env-agent' 2>/dev/null || true
sleep 1

# 确保可执行
chmod +x env-agent

# 启动新进程
echo "启动 env-agent..."
nohup ./env-agent -config env-agent.json > env-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f '\./env-agent' > /dev/null; then
    echo "env-agent 启动成功 (PID: $(pgrep -f '\./env-agent'))"
else
    echo "env-agent 启动失败，查看日志:"
    tail -20 env-agent.log
    exit 1
fi
