#!/bin/bash
# codegen-agent 发布脚本
# 在远程服务器上执行：kill旧进程 → 启动新进程

cd "$(dirname "$0")"

# 停止旧进程
echo "停止 codegen-agent..."
pkill -f './codegen-agent' 2>/dev/null || true
sleep 1

# 确保可执行
chmod +x codegen-agent

# 启动新进程（后台运行，日志写文件）
# 使用 nohup + disown 确保进程与父进程完全分离（macOS 兼容）
echo "启动 codegen-agent..."
nohup ./codegen-agent agent.conf > codegen-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f './codegen-agent' > /dev/null; then
    echo "codegen-agent 启动成功 (PID: $(pgrep -f './codegen-agent'))"
else
    echo "codegen-agent 启动失败，查看日志:"
    tail -20 codegen-agent.log
    exit 1
fi
