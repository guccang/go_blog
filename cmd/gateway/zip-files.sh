#!/bin/bash
set -e  # 遇到错误立即退出

# 获取时间戳
TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="codegen_agent_${TIMESTAMP}.zip"

# 交叉编译 macos amd64
echo "正在编译 codegen-agent (macos/amd64)..."
# 如果是 Intel 芯片：
export GOOS=darwin
export GOARCH=amd64
export CGO_ENABLED=0
go build -o codegen-agent .
if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

# 打包二进制 + 配置
zip -r "${OUTPUT}" codegen-agent agent.conf.example settings/

# 清理编译产物
rm -f codegen-agent

echo "成功生成: ${OUTPUT}"
