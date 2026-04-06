#!/bin/bash
if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

read_pubspec_version() {
    local pubspec="$1"
    local line
    while IFS= read -r line; do
        case "$line" in
            version:*)
                line="${line#version:}"
                line="${line//[[:space:]]/}"
                echo "$line"
                return 0
                ;;
        esac
    done < "$pubspec"
    return 1
}

to_display_version() {
    local version="$1"
    if [[ "$version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)\+([0-9]+)$ ]]; then
        echo "${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.$((${BASH_REMATCH[3]} + ${BASH_REMATCH[4]}))"
    else
        echo "$version"
    fi
}

echo "=== 开始构建并推送 APK ==="
echo ""
echo ">>> 第一步: 构建 APK <<<"
bash ./build-apk.sh
echo ""

# 读取版本号并转换为显示格式
CURRENT_VERSION="$(read_pubspec_version pubspec.yaml)"
DISPLAY_VERSION="$(to_display_version "$CURRENT_VERSION")"
VERSIONED_APK="build/app/outputs/flutter-apk/app-release-${DISPLAY_VERSION}.apk"

echo ">>> 第二步: 推送 APK <<<"
echo "APK 路径: $VERSIONED_APK"
bash ./push-apk.sh -u ztt -f "$VERSIONED_APK" -s http://blog.guccang.cn:8883 -t 123456
echo ""
echo "=== 构建并推送完成 ==="
