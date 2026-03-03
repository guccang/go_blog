#!/bin/bash
# wechat-agent 发布脚本
# 在远程服务器上执行：kill旧进程 → 启动新进程
cd "$(dirname "$0")"
svr="$(pwd)/wechat-agent"
echo $svr

# 停止旧进程
echo "停止 wechat-agent..."
ps aux | grep "$svr" | grep -v "grep" |  awk '{print $2}' | xargs kill -9
sleep 1

# 确保可执行
chmod +x wechat-agent

# 启动新进程（后台运行，日志写文件）
# 使用 nohup + disown 确保进程与父进程完全分离（macOS 兼容）
echo "启动 wechat-agent..."
nohup "$svr" agent.conf > wechat-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f "$svr" > /dev/null; then
    echo "wechat-agent 启动成功 (PID: $(pgrep -f "$svr"))"
else
    echo "wechat-agent 启动失败，查看日志:"
    tail -20 wechat-agent.log
    exit 1
fi