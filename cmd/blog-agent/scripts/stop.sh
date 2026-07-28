

#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP_DIR=$(dirname "$SCRIPT_DIR")
PID_FILE="$APP_DIR/blog-agent.pid"

if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    kill "$(cat "$PID_FILE")"
    rm -f "$PID_FILE"
    echo "blog-agent stopped."
    exit 0
fi

PIDS=$(pgrep -f "^$APP_DIR/blog-agent" || true)
if [ -n "$PIDS" ]; then
    kill $PIDS
    echo "blog-agent stopped."
else
    echo "blog-agent is not running."
fi
