#!/bin/bash
set -e

cd "$(dirname "$0")"
svr="$(pwd)/app-agent"

echo "Stopping app-agent..."
pkill -f "$svr" || true
lsof -ti:8883 | xargs kill -9 2>/dev/null || true
sleep 1

chmod +x app-agent

echo "Starting app-agent..."
nohup "$svr" -config app-agent.json > app-agent.log 2>&1 < /dev/null &
disown || true

sleep 1
if pgrep -f "$svr" > /dev/null; then
    echo "app-agent started"
else
    echo "app-agent failed to start"
    tail -20 app-agent.log || true
    exit 1
fi
