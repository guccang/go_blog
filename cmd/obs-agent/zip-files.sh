#!/bin/bash
set -e

TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="obs-agent_${TIMESTAMP}.zip"

if [ -z "$GOOS" ]; then
    export GOOS=$(go env GOOS)
    export GOARCH=$(go env GOARCH)
fi
export CGO_ENABLED=0

EXT=""
[ "$GOOS" = "windows" ] && EXT=".exe"
BINNAME="obs-agent${EXT}"

echo "building obs-agent (${GOOS}/${GOARCH})..."
go build -o "$BINNAME" .

zip -r "${OUTPUT}" "$BINNAME" obs-agent.json publish.sh
rm -f "$BINNAME"

echo "generated ${OUTPUT}"
