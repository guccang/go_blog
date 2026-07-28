#!/usr/bin/env bash
set -euo pipefail

# Git Bash deployment: build the Linux binary locally, upload, then run remotely.
# Runtime data (data/, blogs_txt/, sys_conf.md) is never included or overwritten.

SERVER=""
SSH_USER="root"
REMOTE_DIR="/opt/blog-agent"
PORT="8080"

usage() {
    cat <<'EOF'
Usage:
  ./scripts/deploy_remote_gitbash.sh -s SERVER [-u USER] [-d REMOTE_DIR] [-p PORT]

Example:
  ./scripts/deploy_remote_gitbash.sh \
    -s 114.115.214.86 -u vibecoding \
    -d /data/program/go/go_blog/cmd/blog-agent -p 8881
EOF
}

while getopts ':s:u:d:p:h' option; do
    case "$option" in
        s) SERVER="$OPTARG" ;;
        u) SSH_USER="$OPTARG" ;;
        d) REMOTE_DIR="$OPTARG" ;;
        p) PORT="$OPTARG" ;;
        h) usage; exit 0 ;;
        *) usage >&2; exit 2 ;;
    esac
done

if [[ -z "$SERVER" || ! "$SERVER" =~ ^[A-Za-z0-9.-]+$ ]]; then
    echo 'A valid -s SERVER is required.' >&2
    exit 2
fi
if [[ ! "$SSH_USER" =~ ^[A-Za-z0-9_-]+$ || ! "$REMOTE_DIR" =~ ^/[A-Za-z0-9._/-]+$ ]]; then
    echo 'Invalid SSH user or remote directory.' >&2
    exit 2
fi
if [[ ! "$PORT" =~ ^[0-9]+$ ]] || (( PORT < 1 || PORT > 65535 )); then
    echo 'Port must be between 1 and 65535.' >&2
    exit 2
fi

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
APP_DIR=$(cd -- "$SCRIPT_DIR/.." && pwd)
TARGET="$SSH_USER@$SERVER"
STAGE_DIR=$(mktemp -d)
ARCHIVE=$(mktemp "${TMPDIR:-/tmp}/blog-agent-release.XXXXXX.tar.gz")
REMOTE_ARCHIVE="/tmp/blog-agent-release-${USER:-deploy}.tar.gz"

cleanup() {
    rm -rf "$STAGE_DIR"
    rm -f "$ARCHIVE"
}
trap cleanup EXIT

echo '[1/4] Building Linux binary locally...'
(
    cd "$APP_DIR"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$STAGE_DIR/blog-agent" .
)

cp -R "$APP_DIR/templates" "$APP_DIR/statics" "$STAGE_DIR/"
cp "$APP_DIR/sys_conf.md" "$STAGE_DIR/sys_conf.md.dist"
mkdir -p "$STAGE_DIR/scripts"
cp "$SCRIPT_DIR/start.sh" "$SCRIPT_DIR/stop.sh" "$SCRIPT_DIR/show.sh" "$SCRIPT_DIR/restart.sh" "$STAGE_DIR/scripts/"

echo '[2/4] Creating release archive...'
tar -C "$STAGE_DIR" -czf "$ARCHIVE" blog-agent templates statics scripts sys_conf.md.dist

echo '[3/4] Uploading release archive...'
scp "$ARCHIVE" "$TARGET:$REMOTE_ARCHIVE"

echo '[4/4] Installing and starting remotely...'
ssh "$TARGET" bash -s -- "$REMOTE_DIR" "$PORT" "$REMOTE_ARCHIVE" <<'REMOTE_SCRIPT'
set -euo pipefail
APP_DIR=$1
PORT=$2
ARCHIVE=$3

cleanup() { rm -f "$ARCHIVE"; }
trap cleanup EXIT

mkdir -p "$APP_DIR/logs" "$APP_DIR/data"
if command -v fuser >/dev/null 2>&1; then
    fuser -k "$PORT/tcp" || true
elif command -v lsof >/dev/null 2>&1; then
    PORT_PIDS=$(lsof -ti "tcp:$PORT" || true)
    [[ -z "$PORT_PIDS" ]] || kill "$PORT_PIDS"
else
    echo "Cannot release port $PORT: install fuser or lsof." >&2
    exit 1
fi
pgrep -f "^$APP_DIR/blog-agent" | xargs -r kill || true
sleep 1

tar -xzf "$ARCHIVE" -C "$APP_DIR"
if [[ ! -f "$APP_DIR/sys_conf.md" ]]; then
    mv "$APP_DIR/sys_conf.md.dist" "$APP_DIR/sys_conf.md"
else
    rm -f "$APP_DIR/sys_conf.md.dist"
fi
chmod +x "$APP_DIR/blog-agent" "$APP_DIR"/scripts/*.sh

if [[ ! -f "$APP_DIR/data/go_blog.db" ]]; then
    [[ -d "$APP_DIR/blogs_txt" ]] || { echo 'Missing remote blogs_txt for first SQLite migration.' >&2; exit 1; }
    "$APP_DIR/blog-agent" migrate-sqlite "$APP_DIR/sys_conf.md"
fi
nohup "$APP_DIR/blog-agent" "$APP_DIR/sys_conf.md" -port "$PORT" > "$APP_DIR/logs/server.stdout.log" 2>&1 < /dev/null &
sleep 2
pgrep -f "^$APP_DIR/blog-agent" >/dev/null
echo "Deployment completed: $APP_DIR"
REMOTE_SCRIPT

echo "Deployment successful: $TARGET:$REMOTE_DIR"
