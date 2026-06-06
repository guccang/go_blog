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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE="${1:-${HERMES_AGENT_SOURCE:-$HOME/.hermes/hermes-agent}}"
TARGET="$SCRIPT_DIR/vendor/hermes_runtime"
RSYNC="$(command -v rsync || true)"

if [[ ! -f "$SOURCE/run_agent.py" || ! -f "$SOURCE/LICENSE" ]]; then
  echo "Hermes source not found: $SOURCE" >&2
  exit 1
fi
if [[ -z "$RSYNC" ]]; then
  echo "rsync is required to synchronize Hermes source" >&2
  exit 1
fi

mkdir -p "$TARGET"
for path in agent tools hermes_cli cron gateway plugins providers skills acp_adapter acp_registry assets locales; do
  "$RSYNC" -a --delete --delete-excluded \
    --exclude '__pycache__/' \
    --exclude '*.pyc' \
    --exclude '*.egg-info/' \
    --exclude 'build/' \
    --exclude 'tests/' \
    --exclude 'test_*.py' \
    --exclude '*_test.py' \
    --exclude 'web_dist/' \
    --exclude 'tui_dist/' \
    "$SOURCE/$path/" "$TARGET/$path/"
done

for file in LICENSE pyproject.toml setup.py run_agent.py model_tools.py toolsets.py \
  toolset_distributions.py trajectory_compressor.py hermes_bootstrap.py \
  hermes_constants.py hermes_logging.py hermes_state.py hermes_time.py utils.py \
  mcp_serve.py batch_runner.py mini_swe_runner.py cli-config.yaml.example; do
  cp "$SOURCE/$file" "$TARGET/$file"
done

find "$TARGET" -type d -name __pycache__ -prune -exec rm -rf {} +
find "$TARGET" -type d -name '*.egg-info' -prune -exec rm -rf {} +
rm -rf "$TARGET/build"
find "$TARGET" -type f -name '*.pyc' -delete

version="$(awk -F '\"' '/^version = / {print $2; exit}' "$SOURCE/pyproject.toml")"
commit="$(git -C "$SOURCE" rev-parse HEAD 2>/dev/null || true)"
python3 - "$TARGET/VENDOR_INFO.json" "$version" "$commit" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

target = Path(sys.argv[1])
value = {
    "name": "Nous Research Hermes Agent",
    "license": "MIT",
    "version": sys.argv[2] or "unknown",
    "commit": sys.argv[3] or "unknown",
    "synced_at": datetime.now(timezone.utc).isoformat(),
}
target.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY

(
  cd "$TARGET"
  if command -v sha256sum >/dev/null 2>&1; then
    find . -type f ! -name SHA256SUMS -print0 \
      | sort -z \
      | xargs -0 sha256sum > SHA256SUMS
  else
    find . -type f ! -name SHA256SUMS -print0 \
      | sort -z \
      | xargs -0 shasum -a 256 > SHA256SUMS
  fi
)

echo "Synced embedded Hermes runtime to $TARGET"
