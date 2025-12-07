#!/bin/bash

# zip-files.sh - 打包Go博客项目文件
# 对应Windows的zip-files.bat脚本

# 设置错误处理
set -e

# 获取当前时间戳（格式：yyyy-MM-dd-HH_mm_ss）
TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")

# 压缩文件名
OUTPUT="go_blog-${TIMESTAMP}.zip"

# 检查是否安装了zip命令
if ! command -v zip &> /dev/null; then
    echo "错误：未找到zip命令，请先安装zip工具"
    echo "在Ubuntu/Debian上：sudo apt-get install zip"
    echo "在macOS上：brew install zip"
    exit 1
fi

# 要打包的文件夹和文件（与.bat文件保持一致）
FOLDERS=(
    "pkgs"
    "statics/css"
    "statics/js"
    "templates"
    "main.go"
    "go.mod"
)

# 检查要打包的文件和文件夹是否存在
echo "检查要打包的文件和文件夹..."
for item in "${FOLDERS[@]}"; do
    if [ ! -e "$item" ]; then
        echo "警告：$item 不存在，跳过"
    fi
done

# 执行压缩
echo "正在打包文件到 ${OUTPUT}..."
zip -r "$OUTPUT" "${FOLDERS[@]}"

# 检查压缩是否成功
if [ $? -eq 0 ]; then
    echo "✅ 成功生成压缩文件：${OUTPUT}"
    echo "文件大小：$(du -h "$OUTPUT" | cut -f1)"
else
    echo "❌ 压缩失败"
    exit 1
fi

# 可选：列出压缩包内容
echo ""
echo "压缩包内容："
unzip -l "$OUTPUT" | head -20
echo "...（更多内容请使用 'unzip -l ${OUTPUT}' 查看）"