#!/bin/bash
echo "Building statistics module..."
go build -buildmode=plugin -o statistics.so statistics.go
echo "Statistics module built successfully." 