#!/bin/bash
echo "Building MCP package..."
go build -buildmode=plugin -o ../../build/mcp.so .
echo "MCP package built successfully"