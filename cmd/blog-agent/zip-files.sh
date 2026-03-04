#!/bin/bash
set -e  # 遇到错误立即退出

# 获取时间戳
TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="blog-agent_${TIMESTAMP}.zip"

# 交叉编译支持：deploy-agent 会在需要时设置 GOOS/GOARCH
# 未设置时使用 Go 默认值（当前平台）
if [ -z "$GOOS" ]; then
    export GOOS=$(go env GOOS)
    export GOARCH=$(go env GOARCH)
fi
export CGO_ENABLED=0

EXT=""
[ "$GOOS" = "windows" ] && EXT=".exe"
BINNAME="blog-agent${EXT}"

echo "正在编译 blog-agent (${GOOS}/${GOARCH})..."
go build -o "$BINNAME" .
if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

# 打包二进制 + 配置
zip -r "${OUTPUT}" "$BINNAME" publish.sh publish.bat templates/ statics/

# 清理编译产物
rm -f "$BINNAME"

echo "成功生成: ${OUTPUT}"
