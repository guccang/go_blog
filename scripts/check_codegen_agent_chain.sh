#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FLUTTER_DIR="$ROOT_DIR/cmd/flutter-client-for-appagent/flutter_client_for_appagent"

export LC_ALL="${LC_ALL:-en_US.UTF-8}"
export LANG="${LANG:-en_US.UTF-8}"

if [[ "$(locale charmap)" != "UTF-8" ]]; then
  export LC_ALL="en_US.UTF-8"
  export LANG="en_US.UTF-8"
fi

if [[ "$(locale charmap)" != "UTF-8" ]]; then
  echo "console encoding must be UTF-8" >&2
  exit 1
fi

run_go_tests() {
  local dir="$1"
  shift
  echo "==> go test $dir $*"
  (cd "$ROOT_DIR/$dir" && go test "$@")
}

echo "==> validating codegen agent chain contracts"
run_go_tests cmd/acp-agent ./...
run_go_tests cmd/cmd-agent ./...
run_go_tests cmd/app-agent ./...

echo "==> probing Flutter codegen stream consumer"
(
  cd "$FLUTTER_DIR"
  dart run tool/codegen_stream_probe.dart --min-codegen-updates 3
  flutter test test/chat_message_persistence_test.dart test/codegen_stream_probe_test.dart
  flutter analyze
)

echo "==> codegen agent chain checks passed"
