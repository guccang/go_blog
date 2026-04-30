#!/bin/bash
# cmd-agent 发布脚本
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "停止 cmd-agent..."

if [ -f cmd-agent.pid ]; then
    OLD_PID=$(cat cmd-agent.pid)
    echo "通过 PID 文件终止进程组: PID=$OLD_PID"
    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true
    rm -f cmd-agent.pid
fi
pkill -f '\./cmd-agent' 2>/dev/null || true
pkill -f "$SCRIPT_DIR/cmd-agent" 2>/dev/null || true
sleep 1

chmod +x "$SCRIPT_DIR/cmd-agent"

echo "启动 cmd-agent..."
nohup "$SCRIPT_DIR/cmd-agent" -config "$SCRIPT_DIR/cmd-agent.json" > "$SCRIPT_DIR/cmd-agent.log" 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f "$SCRIPT_DIR/cmd-agent" > /dev/null; then
    echo "cmd-agent 启动成功 (PID: $(pgrep -f "$SCRIPT_DIR/cmd-agent"))"
else
    echo "cmd-agent 启动失败，查看日志:"
    tail -20 "$SCRIPT_DIR/cmd-agent.log"
    exit 1
fi
