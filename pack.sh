#!/bin/bash

# 打包 go_blog 项目文件
# 包含: pkgs templates statics main.go go.mod

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 生成日期字符串 格式: YYYYMMDD
DATE=$(date +%Y%m%d_%H%M%S)

# 输出文件名
OUTPUT="go_blog_${DATE}.zip"

# 要打包的文件和目录
FILES="pkgs templates statics main.go go.mod go.sum"

# 检查文件是否存在
for f in $FILES; do
    if [ ! -e "$f" ]; then
        echo "警告: $f 不存在，将跳过"
    fi
done

# 删除旧的同名zip文件
if [ -f "$OUTPUT" ]; then
    echo "删除已存在的 $OUTPUT"
    rm "$OUTPUT"
fi

# 打包
echo "正在打包到 $OUTPUT ..."
zip -r "$OUTPUT" $FILES -x "*.DS_Store" -x "*/__pycache__/*" -x "*.pyc"

# 检查结果
if [ $? -eq 0 ]; then
    echo "✅ 打包完成: $OUTPUT"
    echo "文件大小: $(du -h "$OUTPUT" | cut -f1)"
else
    echo "❌ 打包失败"
    exit 1
fi

scp $OUTPUT root@114.115.214.86:/data/program/go/go_blog
ssh root@114.115.214.86 "cd /data/program/go/go_blog; unzip $OUTPUT;"
