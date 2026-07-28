

#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP_DIR=$(dirname "$SCRIPT_DIR")

if pgrep -af "^$APP_DIR/blog-agent"; then
    exit 0
fi

echo "blog-agent is not running."
exit 1
