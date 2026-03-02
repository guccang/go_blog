#!/bin/bash
set -e
cd "$(dirname "$0")"

echo "========================================"
echo "    开始编译 goblog 所有进程"
echo "========================================"
echo

# 设置变量
BUILD_TIME=$(date '+%Y-%m-%d %H:%M:%S')
BUILD_USER=$(whoami)
GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}
export CGO_ENABLED=0

# 显示配置信息
echo "[配置信息]"
echo "编译时间: $BUILD_TIME"
echo "编译用户: $BUILD_USER"
echo "目标平台: $GOOS/$GOARCH"
echo

echo "[开始编译]"
echo "--------------------------------"

SUCCESS_COUNT=0
FAIL_COUNT=0

# 编译主程序
echo "正在编译 go_blog..."
if go build -ldflags="-s -w" -o go_blog; then
    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    echo "  ✓ go_blog 编译成功"
else
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo "  ✗ go_blog 编译失败"
fi

# 子服务列表
SERVICES=("codegen-agent" "deploy-agent" "gateway" "wechat-agent" "llm-mcp-agent")

for svc in "${SERVICES[@]}"; do
    echo "正在编译 ${svc}..."
    if (cd "./cmd/${svc}" && go build -ldflags="-s -w" -o "${svc}"); then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        echo "  ✓ ${svc} 编译成功"
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo "  ✗ ${svc} 编译失败"
    fi
done

TOTAL=$((SUCCESS_COUNT + FAIL_COUNT))
echo "--------------------------------"
echo
echo "[编译结果]"
echo "成功: ${SUCCESS_COUNT} / ${TOTAL}"
echo "失败: ${FAIL_COUNT} / ${TOTAL}"
echo

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo "✓ 所有进程编译成功！"
else
    echo "✗ 编译过程中出现错误，请检查日志"
fi

echo
echo "========================================"
