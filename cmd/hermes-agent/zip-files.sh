#!/usr/bin/env bash
set -euo pipefail

export LC_ALL="${LC_ALL:-C.UTF-8}"
export LANG="${LANG:-C.UTF-8}"

cd "$(dirname "$0")"

timestamp="$(date +"%Y-%m-%d-%H_%M_%S")"
output="hermes-agent_${timestamp}.zip"

rm -f hermes-agent_*.zip
zip -r "$output" \
  hermes-agent \
  main.py \
  config.py \
  runtime.py \
  uap_client.py \
  hermes-agent.json.example \
  publish.sh \
  README.md

echo "generated: ${output}"
