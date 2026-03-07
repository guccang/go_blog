#!/bin/bash
set -e

# Get timestamp
TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="deploy-agent_${TIMESTAMP}.zip"

# Cross-compilation: deploy-agent sets GOOS/GOARCH when needed
if [ -z "$GOOS" ]; then
    export GOOS=$(go env GOOS)
    export GOARCH=$(go env GOARCH)
fi
export CGO_ENABLED=0

EXT=""
[ "$GOOS" = "windows" ] && EXT=".exe"
BINNAME="deploy-agent${EXT}"

echo "Building deploy-agent (${GOOS}/${GOARCH})..."
go build -o "$BINNAME" .
if [ $? -ne 0 ]; then
    echo "Build failed"
    exit 1
fi

# Package binary + config
zip -r "${OUTPUT}" "$BINNAME" publish.sh publish.bat deploy.conf settings/

# Clean build artifacts
rm -f "$BINNAME"

echo "Generated: ${OUTPUT}"
