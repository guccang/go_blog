#!/bin/bash

cd "$(dirname "$0")"

echo "stopping obs-agent..."
if [ -f obs-agent.pid ]; then
    OLD_PID=$(cat obs-agent.pid)
    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true
    rm -f obs-agent.pid
fi
pkill -f '\./obs-agent' 2>/dev/null || true
sleep 1

chmod +x obs-agent

echo "starting obs-agent..."
nohup ./obs-agent -config obs-agent.json > obs-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f '\./obs-agent' > /dev/null; then
    echo "obs-agent started"
else
    echo "obs-agent failed, tail log:"
    tail -20 obs-agent.log
    exit 1
fi
