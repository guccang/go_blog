#!/bin/bash
# deploy-bridge-server Linux 打包脚本

APP="deploy-bridge-server"
DATE=$(date +%Y%m%d_%H%M%S)
ZIP_NAME="${APP}_${DATE}.zip"

echo "=== 打包 $APP ==="

# 编译
go build -o "$APP" .
if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

# 打包
zip "$ZIP_NAME" "$APP" publish.sh bridge-server.json
echo "=== 打包完成: $ZIP_NAME ==="
