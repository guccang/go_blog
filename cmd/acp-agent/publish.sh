#!/bin/bash
# acp-agent 发布脚本
# 在远程服务器上执行：kill旧进程（含子进程树） → 启动新进程

cd "$(dirname "$0")"

# 停止旧进程
echo "停止 acp-agent..."

# 优先通过 PID 文件杀进程树（含 claude 子进程）
if [ -f acp-agent.pid ]; then
    OLD_PID=$(cat acp-agent.pid)
    echo "通过 PID 文件终止进程组: PID=$OLD_PID"
    # 杀掉进程组（负号表示进程组）
    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true
    rm -f acp-agent.pid
fi
# 兜底：按名称杀残留
pkill -f '\./acp-agent' 2>/dev/null || true
sleep 1

# 确保可执行
chmod +x acp-agent

# 启动新进程（后台运行，日志写文件）
# 使用 nohup + disown 确保进程与父进程完全分离（macOS 兼容）
echo "启动 acp-agent..."
nohup ./acp-agent acp-agent.json > codegen-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f '\./acp-agent' > /dev/null; then
    echo "acp-agent 启动成功 (PID: $(pgrep -f '\./acp-agent'))"
else
    echo "acp-agent 启动失败，查看日志:"
    tail -20 acp-agent.log
    exit 1
fi
