#!/bin/bash
set -e
echo "=== 开始构建并推送 APK ==="
echo ""
echo ">>> 第一步: 构建 APK <<<"
sh ./build-apk.sh
echo ""

# 读取版本号并转换为显示格式
CURRENT_VERSION=$(grep "^version:" pubspec.yaml | sed 's/version: *//' | tr -d ' ')
if [[ "$CURRENT_VERSION" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)\+([0-9]+)$ ]]; then
    # 格式: major.minor.patch+build -> display: major.minor.(patch+build)
    DISPLAY_VERSION="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.$((${BASH_REMATCH[3]} + ${BASH_REMATCH[4]}))"
else
    DISPLAY_VERSION="$CURRENT_VERSION"
fi
VERSIONED_APK="build/app/outputs/flutter-apk/app-release-${DISPLAY_VERSION}.apk"

echo ">>> 第二步: 推送 APK <<<"
echo "APK 路径: $VERSIONED_APK"
sh ./push-apk.sh -u ztt -f "$VERSIONED_APK" -s http://blog.guccang.cn:8883 -t 123456
echo ""
echo "=== 构建并推送完成 ==="
