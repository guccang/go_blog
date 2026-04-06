#!/bin/bash

cd "$(dirname "$0")"

echo "stopping image-agent..."
if [ -f image-agent.pid ]; then
    OLD_PID=$(cat image-agent.pid)
    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true
    rm -f image-agent.pid
fi
pkill -f '\./image-agent' 2>/dev/null || true
sleep 1

chmod +x image-agent

echo "starting image-agent..."
nohup ./image-agent -config image-agent.json > image-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f '\./image-agent' > /dev/null; then
    echo "image-agent started"
else
    echo "image-agent failed, tail log:"
    tail -20 image-agent.log
    exit 1
fi
