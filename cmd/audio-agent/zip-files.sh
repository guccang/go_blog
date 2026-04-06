#!/bin/bash
set -e

TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="audio-agent_${TIMESTAMP}.zip"

if [ -z "$GOOS" ]; then
    export GOOS=$(go env GOOS)
    export GOARCH=$(go env GOARCH)
fi
export CGO_ENABLED=0

EXT=""
[ "$GOOS" = "windows" ] && EXT=".exe"
BINNAME="audio-agent${EXT}"

echo "building audio-agent (${GOOS}/${GOARCH})..."
go build -o "$BINNAME" .

zip -r "${OUTPUT}" "$BINNAME" audio-agent.json publish.sh
rm -f "$BINNAME"

echo "generated ${OUTPUT}"
