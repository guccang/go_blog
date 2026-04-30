#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "stopping obs-agent..."
if [ -f obs-agent.pid ]; then
    OLD_PID=$(cat obs-agent.pid)
    kill -9 -$OLD_PID 2>/dev/null || kill -9 $OLD_PID 2>/dev/null || true
    rm -f obs-agent.pid
fi
pkill -f '\./obs-agent' 2>/dev/null || true
pkill -f "$SCRIPT_DIR/obs-agent" 2>/dev/null || true
sleep 1

chmod +x "$SCRIPT_DIR/obs-agent"
if [ -f "$SCRIPT_DIR/obsutil/linux/obsutil" ]; then
    chmod +x "$SCRIPT_DIR/obsutil/linux/obsutil"
fi
if [ -f "$SCRIPT_DIR/obsutil/macos/obsutil" ]; then
    chmod +x "$SCRIPT_DIR/obsutil/macos/obsutil"
fi

echo "starting obs-agent..."
nohup "$SCRIPT_DIR/obs-agent" -config "$SCRIPT_DIR/obs-agent.json" > "$SCRIPT_DIR/obs-agent.log" 2>&1 < /dev/null &
disown

sleep 1
if pgrep -f "$SCRIPT_DIR/obs-agent" > /dev/null; then
    echo "obs-agent started"
else
    echo "obs-agent failed, tail log:"
    tail -20 "$SCRIPT_DIR/obs-agent.log"
    exit 1
fi
