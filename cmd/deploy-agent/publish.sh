#!/bin/bash
# deploy-agent publish script
# kill old process -> start new process
cd "$(dirname "$0")"
svr="$(pwd)/deploy-agent"
echo $svr

# Stop old process
echo "Stopping deploy-agent..."
ps aux | grep "$svr" | grep -v "grep" | awk '{print $2}' | xargs kill -9
sleep 1

# Ensure executable
chmod +x deploy-agent

# Start new process (background, detached)
# nohup + disown ensures process survives parent exit (macOS compatible)
echo "Starting deploy-agent..."
nohup "$svr" > deploy-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f "$svr" > /dev/null; then
    echo "deploy-agent started (PID: $(pgrep -f "$svr"))"
else
    echo "deploy-agent failed to start, check log:"
    tail -20 deploy-agent.log
    exit 1
fi
