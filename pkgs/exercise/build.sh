#!/bin/bash

echo "Building exercise module..."

# 构建模块
go build -o exercise.so -buildmode=plugin .

if [ $? -eq 0 ]; then
    echo "Exercise module built successfully"
else
    echo "Failed to build exercise module"
    exit 1
fi 