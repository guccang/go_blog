#!/bin/bash
# 修复 Hermes Agent DeepSeek API Key 环境变量问题
# 作用：确保 state/hermes/.env 文件中包含正确的 DEEPSEEK_API_KEY

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/hermes-agent.json"
ENV_FILE="$SCRIPT_DIR/state/hermes/.env"
ENV_DIR="$(dirname "$ENV_FILE")"

echo "=== Hermes Agent DeepSeek 环境变量修复工具 ==="
echo ""
echo "工作目录: $SCRIPT_DIR"
echo "配置文件: $CONFIG_FILE"
echo "环境变量文件: $ENV_FILE"
echo ""

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ 错误: 找不到配置文件 $CONFIG_FILE"
    echo "请先创建 hermes-agent.json 配置文件"
    exit 1
fi

echo "✓ 找到配置文件: $CONFIG_FILE"

# 读取配置
PROVIDER=$(jq -r '.provider // "unknown"' "$CONFIG_FILE")
API_KEY=$(jq -r '.api_key // ""' "$CONFIG_FILE")
MODEL=$(jq -r '.model // "unknown"' "$CONFIG_FILE")

echo "  Provider: $PROVIDER"
echo "  Model: $MODEL"
echo "  API Key: ${API_KEY:0:10}... (${#API_KEY} 字符)"
echo ""

# 检查是否是 deepseek provider
if [ "$PROVIDER" != "deepseek" ]; then
    echo "⚠️  警告: 当前 provider 不是 deepseek ($PROVIDER)"
    echo "如果你想使用 deepseek，请修改 hermes-agent.json:"
    echo '  "provider": "deepseek",'
    echo '  "model": "deepseek-v4-flash",'
    echo '  "api_key": "your-deepseek-api-key"'
    echo ""
fi

# 检查 API key 是否存在
if [ -z "$API_KEY" ]; then
    echo "❌ 错误: hermes-agent.json 中没有配置 api_key"
    echo ""
    echo "请在 $CONFIG_FILE 中添加:"
    echo '  "api_key": "your-api-key-here"'
    exit 1
fi

echo "✓ 配置文件中有 API key"
echo ""

# 创建 state/hermes 目录
if [ ! -d "$ENV_DIR" ]; then
    echo "创建目录: $ENV_DIR"
    mkdir -p "$ENV_DIR"
fi

# 生成 .env 文件
echo "生成环境变量文件: $ENV_FILE"
cat > "$ENV_FILE" << EOF
# BEGIN go_blog hermes-agent managed model env
# 此文件由 fix-deepseek-env.sh 自动生成
# 请勿手动编辑 - 修改请编辑 hermes-agent.json 后重新运行此脚本

DEEPSEEK_API_KEY="$API_KEY"
# END go_blog hermes-agent managed model env
EOF

chmod 600 "$ENV_FILE"
echo "✓ 环境变量文件已创建并设置权限为 600"
echo ""

# 验证
echo "=== 验证 ==="
if [ -f "$ENV_FILE" ]; then
    echo "✓ $ENV_FILE 存在"
    if grep -q "DEEPSEEK_API_KEY" "$ENV_FILE"; then
        echo "✓ DEEPSEEK_API_KEY 已写入"
    else
        echo "❌ DEEPSEEK_API_KEY 未找到"
        exit 1
    fi
else
    echo "❌ 环境变量文件创建失败"
    exit 1
fi

echo ""
echo "=== 完成 ==="
echo "✓ DeepSeek API key 已成功配置到 $ENV_FILE"
echo ""
echo "下一步:"
echo "1. 重启 hermes-agent 服务"
echo "2. 检查 Cron job 是否正常运行"
echo ""
