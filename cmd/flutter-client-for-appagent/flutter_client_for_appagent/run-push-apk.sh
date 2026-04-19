#!/bin/bash
if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

usage() {
    cat <<'EOF'
用法:
  ./run-push-apk.sh (-u <to_user> | -g <group_id>) [options]

说明:
  1. 先执行 Flutter release APK 构建
  2. 再调用 app-agent /api/app/upload-apk
  3. 由 app-agent 负责把 APK 同步到 OBS，并下发带 download_ticket 的消息
  4. Flutter 客户端收到后优先通过 obs-agent 下载

必填参数:
  -u, --user        目标用户 ID
  -g, --group       目标群组 ID

可选参数:
  -s, --server      app-agent 地址，默认读取 APP_AGENT_SERVER，否则 http://127.0.0.1:9002
  -t, --token       app-agent receive_token，默认读取 APP_AGENT_TOKEN
  -m, --message     推送文案，默认 "新的安装包已下发，点击安装"
  --skip-build      跳过构建，直接推送已存在的版本化 APK
  -h, --help        显示帮助

示例:
  ./run-push-apk.sh -u ztt -s http://127.0.0.1:9002 -t test-token
  ./run-push-apk.sh -g team-alpha -s http://blog.guccang.cn:8883 -t 123456
EOF
}

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

APP_AGENT_SERVER="${APP_AGENT_SERVER:-http://127.0.0.1:9002}"
APP_AGENT_TOKEN="${APP_AGENT_TOKEN:-}"
TARGET_USER=""
TARGET_GROUP=""
MESSAGE="新的安装包已下发，点击安装"
SKIP_BUILD=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        -u|--user)
            TARGET_USER="${2:-}"
            shift 2
            ;;
        -g|--group)
            TARGET_GROUP="${2:-}"
            shift 2
            ;;
        -s|--server)
            APP_AGENT_SERVER="${2:-}"
            shift 2
            ;;
        -t|--token)
            APP_AGENT_TOKEN="${2:-}"
            shift 2
            ;;
        -m|--message)
            MESSAGE="${2:-}"
            shift 2
            ;;
        --skip-build)
            SKIP_BUILD=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "未知参数: $1" >&2
            usage
            exit 1
            ;;
    esac
done

if [[ -n "${TARGET_USER}" && -n "${TARGET_GROUP}" ]]; then
    echo "不能同时指定 -u/--user 与 -g/--group" >&2
    usage
    exit 1
fi

if [[ -z "${TARGET_USER}" && -z "${TARGET_GROUP}" ]]; then
    echo "必须指定目标用户或群组" >&2
    usage
    exit 1
fi

echo "=== 开始构建并推送 APK ==="
echo "app-agent: ${APP_AGENT_SERVER}"
if [[ -n "${TARGET_USER}" ]]; then
    echo "目标用户: ${TARGET_USER}"
else
    echo "目标群组: ${TARGET_GROUP}"
fi
echo ""

if [[ "${SKIP_BUILD}" -eq 0 ]]; then
    echo ">>> 第一步: 构建 APK <<<"
    bash ./build-apk.sh
    echo ""
else
    echo ">>> 第一步: 跳过构建，复用已有 APK <<<"
    echo ""
fi

CURRENT_VERSION="$(read_pubspec_version pubspec.yaml)"
DISPLAY_VERSION="$(to_display_version "$CURRENT_VERSION")"
VERSIONED_APK="build/app/outputs/flutter-apk/app-release-${DISPLAY_VERSION}.apk"

if [[ ! -f "${VERSIONED_APK}" ]]; then
    echo "版本化 APK 不存在: ${VERSIONED_APK}" >&2
    exit 1
fi

echo ">>> 第二步: 推送 APK 到 app-agent <<<"
echo "APK 路径: ${VERSIONED_APK}"

PUSH_ARGS=(-f "${VERSIONED_APK}" -s "${APP_AGENT_SERVER}" -m "${MESSAGE}")
if [[ -n "${APP_AGENT_TOKEN}" ]]; then
    PUSH_ARGS+=(-t "${APP_AGENT_TOKEN}")
fi
if [[ -n "${TARGET_USER}" ]]; then
    PUSH_ARGS+=(-u "${TARGET_USER}")
else
    PUSH_ARGS+=(-g "${TARGET_GROUP}")
fi

bash ./push-apk.sh "${PUSH_ARGS[@]}"
echo ""
echo "=== 构建并推送完成 ==="
