#!/bin/bash
set -e

TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="mcp-agent_${TIMESTAMP}.zip"

if [ -z "$GOOS" ]; then
    export GOOS=$(go env GOOS)
    export GOARCH=$(go env GOARCH)
fi
export CGO_ENABLED=0

EXT=""
[ "$GOOS" = "windows" ] && EXT=".exe"
BINNAME="mcp-agent${EXT}"

echo "正在编译 mcp-agent (${GOOS}/${GOARCH})..."
go build -o "$BINNAME" .
if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

zip -r "${OUTPUT}" "$BINNAME" mcp-agent.json publish.sh

rm -f "$BINNAME"

echo "成功生成: ${OUTPUT}"
