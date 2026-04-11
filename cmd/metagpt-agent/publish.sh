#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$ROOT_DIR"

if [ ! -d ".venv" ]; then
  python3 -m venv .venv
fi

. .venv/bin/activate
pip install -r requirements.txt

pkill -f "python.*main.py --config metagpt-agent.json" 2>/dev/null || true
nohup python3 main.py --config metagpt-agent.json > metagpt-agent.log 2>&1 < /dev/null &
disown || true

sleep 1
if pgrep -f "python.*main.py --config metagpt-agent.json" >/dev/null 2>&1; then
  echo "metagpt-agent started"
else
  echo "metagpt-agent failed to start"
  tail -20 metagpt-agent.log || true
  exit 1
fi
