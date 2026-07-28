
#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP_DIR=$(dirname "$SCRIPT_DIR")
PID_FILE="$APP_DIR/blog-agent.pid"

if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "blog-agent is already running (pid $(cat "$PID_FILE"))."
    exit 0
fi

mkdir -p "$APP_DIR/logs" "$APP_DIR/data"
cd "$APP_DIR"
nohup "$APP_DIR/blog-agent" "$APP_DIR/sys_conf.md" > "$APP_DIR/logs/server.stdout.log" 2>&1 < /dev/null &
echo $! > "$PID_FILE"
echo "blog-agent started (pid $(cat "$PID_FILE"))."
