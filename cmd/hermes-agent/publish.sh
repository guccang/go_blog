#!/usr/bin/env bash
set -euo pipefail

export PYTHONUTF8=1
export PYTHONIOENCODING=UTF-8
if locale -a 2>/dev/null | grep -Eiq '^en_US\.UTF-?8$'; then
  export LC_ALL=en_US.UTF-8
elif locale -a 2>/dev/null | grep -Eiq '^C\.UTF-?8$'; then
  export LC_ALL=C.UTF-8
else
  export LC_ALL=zh_CN.UTF-8
fi
export LANG="$LC_ALL"

cd "$(dirname "$0")"

if [[ ! -f hermes-agent.json ]]; then
  cp hermes-agent.json.example hermes-agent.json
  echo "created hermes-agent.json from example"
fi

chmod +x hermes-agent publish.sh

echo "Stopping hermes-agent..."
pkill -f "[m]ain.py --config hermes-agent.json" 2>/dev/null || true
sleep 1

echo "Starting hermes-agent..."
./hermes-agent --config hermes-agent.json --check-runtime
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
