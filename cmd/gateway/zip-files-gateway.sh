#!/bin/bash
set -e  # 遇到错误立即退出

# 获取时间戳
TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="gateway_${TIMESTAMP}.zip"

# 交叉编译 Linux amd64（用于远程服务器）
echo "正在编译 gateway (linux/amd64)..."
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
go build -o gateway .
if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

# 打包二进制 + 配置文件
zip -r "${OUTPUT}" gateway gateway.json

# 清理编译产物
rm -f gateway

echo "成功生成: ${OUTPUT}"