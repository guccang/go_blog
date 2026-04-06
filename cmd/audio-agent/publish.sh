#!/bin/bash

cd "$(dirname "$0")"

echo "stopping audio-agent..."
if [ -f audio-agent.pid ]; then
    OLD_PID=$(cat audio-agent.pid)
    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true
    rm -f audio-agent.pid
fi
pkill -f '\./audio-agent' 2>/dev/null || true
sleep 1

chmod +x audio-agent

echo "starting audio-agent..."
nohup ./audio-agent -config audio-agent.json > audio-agent.log 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f '\./audio-agent' > /dev/null; then
    echo "audio-agent started"
else
    echo "audio-agent failed, tail log:"
    tail -20 audio-agent.log
    exit 1
fi
