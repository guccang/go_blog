#!/usr/bin/env bash
set -euo pipefail

export PYTHONUTF8=1
export PYTHONIOENCODING=UTF-8

cd "$(dirname "$0")"

if [[ ! -f hermes-agent.json ]]; then
  cp hermes-agent.json.example hermes-agent.json
  echo "created hermes-agent.json from example"
fi

hermes_source="${HERMES_AGENT_SOURCE:-$HOME/.hermes/hermes-agent}"
if [[ ! -x "$hermes_source/.venv/bin/python" && ! -x "$hermes_source/venv/bin/python" ]]; then
  echo "Hermes Python environment not found under $hermes_source" >&2
  exit 1
fi

chmod +x hermes-agent publish.sh

echo "Stopping hermes-agent..."
pkill -f "[m]ain.py --config hermes-agent.json" 2>/dev/null || true
sleep 1

echo "Starting hermes-agent..."
nohup ./hermes-agent --config hermes-agent.json > hermes-agent.log 2>&1 < /dev/null &
disown || true

sleep 2
if pgrep -f "[m]ain.py --config hermes-agent.json" >/dev/null 2>&1; then
  echo "hermes-agent started"
else
  echo "hermes-agent failed to start"
  tail -20 hermes-agent.log || true
  exit 1
fi
