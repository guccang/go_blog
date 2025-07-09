#!/bin/bash
echo "Building reading module..."
go build -o reading.exe reading.go
if [ $? -eq 0 ]; then
    echo "Reading module built successfully!"
else
    echo "Failed to build reading module!"
    exit 1
fi 