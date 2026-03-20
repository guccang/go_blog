#!/bin/bash
# deploy-bridge-server 发布脚本
# 在目标服务器上执行：停止旧进程 → 启动新进程

APP="deploy-bridge-server"
CONFIG="bridge-server.json"

echo "=== 发布 $APP ==="

# 停止旧进程
OLD_PID=$(pgrep -f "./$APP" | head -1)
if [ -n "$OLD_PID" ]; then
    echo "停止旧进程 PID=$OLD_PID"
    kill "$OLD_PID" 2>/dev/null
    sleep 1
    # 确认已停止
    if kill -0 "$OLD_PID" 2>/dev/null; then
        echo "强制停止 PID=$OLD_PID"
        kill -9 "$OLD_PID" 2>/dev/null
    fi
fi

# 赋予执行权限
chmod +x "$APP"

# 启动新进程（setsid 创建新会话，完全脱离父进程）
setsid ./"$APP" "$CONFIG" </dev/null >/dev/null 2>&1 &
NEW_PID=$!
echo "启动新进程 PID=$NEW_PID"

sleep 1
if kill -0 "$NEW_PID" 2>/dev/null; then
    echo "=== $APP 发布成功 ==="
else
    echo "=== $APP 启动失败 ==="
    exit 1
fi
