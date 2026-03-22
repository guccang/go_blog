#!/bin/bash
set -e

# 获取时间戳
TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="init-agent_${TIMESTAMP}.zip"

# 交叉编译支持：deploy-agent 会在需要时设置 GOOS/GOARCH
if [ -z "$GOOS" ]; then
    export GOOS=$(go env GOOS)
    export GOARCH=$(go env GOARCH)
fi
export CGO_ENABLED=0

EXT=""
[ "$GOOS" = "windows" ] && EXT=".exe"
BINNAME="init-agent${EXT}"

echo "正在编译 init-agent (${GOOS}/${GOARCH})..."
go build -o "$BINNAME" .
if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

# 打包二进制 + 配置
zip -r "${OUTPUT}" "$BINNAME" init-agent.json publish.sh

# 清理编译产物
rm -f "$BINNAME"

echo "成功生成: ${OUTPUT}"
