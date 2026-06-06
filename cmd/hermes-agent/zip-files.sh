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

timestamp="$(date +"%Y-%m-%d-%H_%M_%S")"
output="hermes-agent_${timestamp}.zip"

rm -f hermes-agent_*.zip
zip -r "$output" \
  hermes-agent \
  main.py \
  config.py \
  cron_bridge.py \
  native_runtime.py \
  runtime.py \
  uap_client.py \
  requirements.lock \
  sync-hermes-runtime.sh \
  vendor \
  hermes-agent.json.example \
  publish.sh \
  README.md \
  -x '*/__pycache__/*' '*.pyc' '*/hermes_agent.egg-info/*' 'vendor/hermes_runtime/build/*'

echo "generated: ${output}"
