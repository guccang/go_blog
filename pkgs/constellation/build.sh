#!/bin/bash

# Constellation module build script
# 星座占卜模块构建脚本

set -e

echo "🌟 开始构建星座占卜模块..."

# 检查Go版本
echo "检查Go版本..."
go version

# 下载依赖
echo "下载模块依赖..."
go mod download

# 格式化代码
echo "格式化代码..."
go fmt ./...

# 运行测试（如果有）
echo "运行测试..."
if [ -f "*_test.go" ]; then
    go test -v ./...
else
    echo "暂无测试文件"
fi

# 检查代码质量
echo "检查代码质量..."
go vet ./...

# 构建检查
echo "构建检查..."
go build -v ./...

echo "✨ 星座占卜模块构建完成！"
echo ""
echo "模块信息："
echo "- 名称: 星座占卜运势系统"
echo "- 版本: v1.0.0"  
echo "- 功能: 每日运势、塔罗占卜、星座配对、个人星盘"
echo "- 作者: Go Blog System"
echo ""
echo "使用方法："
echo "1. 访问 /constellation 查看星座占卜主页"
echo "2. 通过API接口进行各种占卜功能调用"
echo "3. 查看占卜历史和统计数据"