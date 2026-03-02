#!/bin/bash
# gateway 发布脚本
# 在远程服务器上执行：kill旧进程 → 启动新进程

cd "$(dirname "$0")"

# 停止旧进程
echo "停止 gateway..."
pkill -f './gateway' 2>/dev/null || true
sleep 1

# 确保可执行
chmod +x gateway

# 启动新进程（后台运行，日志写文件）
# 使用 nohup + disown 确保进程与父进程完全分离
echo "启动 gateway..."
nohup ./gateway > gateway.log 2>&1 < /dev/null &
disown

sleep 2  # 给gateway更多时间启动
if pgrep -f './gateway' > /dev/null; then
    echo "gateway 启动成功 (PID: $(pgrep -f './gateway'))"
    echo "监听端口: 9000 (根据gateway.json配置)"
else
    echo "gateway 启动失败，查看日志:"
    tail -20 gateway.log
    exit 1
fi