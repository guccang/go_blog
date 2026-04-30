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

兼容说明:
  deploy-agent 无参调用时，会回退到历史默认值：
  默认用户 RUN_PUSH_APK_DEFAULT_USER（未设置时为 ztt）
  默认群组 RUN_PUSH_APK_DEFAULT_GROUP
  默认服务 RUN_PUSH_APK_DEFAULT_SERVER（未设置时为 http://blog.guccang.cn:8883）
  默认 token 先取 RUN_PUSH_APK_DEFAULT_TOKEN，再取 assets/app_config.json 中的 receive_token

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

APP_AGENT_SERVER_FROM_ENV="${APP_AGENT_SERVER-}"
APP_AGENT_TOKEN_FROM_ENV="${APP_AGENT_TOKEN-}"
APP_AGENT_SERVER="${APP_AGENT_SERVER_FROM_ENV:-http://127.0.0.1:9002}"
APP_AGENT_TOKEN="${APP_AGENT_TOKEN_FROM_ENV:-}"
TARGET_USER=""
TARGET_GROUP=""
MESSAGE="新的安装包已下发，点击安装"
SKIP_BUILD=0
TARGET_EXPLICIT=0
SERVER_EXPLICIT=0
TOKEN_EXPLICIT=0

read_default_receive_token() {
    local app_config="assets/app_config.json"
    local line
    if [[ ! -f "${app_config}" ]]; then
        return 1
    fi
    while IFS= read -r line; do
        case "$line" in
            *\"receive_token\"*)
                line="${line#*:}"
                line="${line//[[:space:]\",]/}"
                if [[ -n "${line}" ]]; then
                    echo "$line"
                    return 0
                fi
                ;;
        esac
    done < "${app_config}"
    return 1
}

apply_legacy_defaults_for_deploy_agent() {
    local fallback_user fallback_group fallback_server fallback_token

    fallback_user="${RUN_PUSH_APK_DEFAULT_USER:-ztt}"
    fallback_group="${RUN_PUSH_APK_DEFAULT_GROUP:-}"

    if [[ -n "${fallback_user}" && -n "${fallback_group}" ]]; then
        echo "RUN_PUSH_APK_DEFAULT_USER 与 RUN_PUSH_APK_DEFAULT_GROUP 不能同时设置" >&2
        exit 1
    fi

    if [[ -n "${fallback_user}" ]]; then
        TARGET_USER="${fallback_user}"
        echo "未指定 -u/-g，兼容 deploy-agent 无参调用，回退到默认用户: ${TARGET_USER}"
    elif [[ -n "${fallback_group}" ]]; then
        TARGET_GROUP="${fallback_group}"
        echo "未指定 -u/-g，兼容 deploy-agent 无参调用，回退到默认群组: ${TARGET_GROUP}"
    fi

    if [[ -z "${TARGET_USER}" && -z "${TARGET_GROUP}" ]]; then
        return 0
    fi

    if [[ "${SERVER_EXPLICIT}" -eq 0 && -z "${APP_AGENT_SERVER_FROM_ENV}" ]]; then
        fallback_server="${RUN_PUSH_APK_DEFAULT_SERVER:-http://blog.guccang.cn:8883}"
        if [[ -n "${fallback_server}" ]]; then
            APP_AGENT_SERVER="${fallback_server}"
        fi
    fi

    if [[ "${TOKEN_EXPLICIT}" -eq 0 && -z "${APP_AGENT_TOKEN_FROM_ENV}" ]]; then
        fallback_token="${RUN_PUSH_APK_DEFAULT_TOKEN:-}"
        if [[ -z "${fallback_token}" ]]; then
            fallback_token="$(read_default_receive_token || true)"
        fi
        if [[ -n "${fallback_token}" ]]; then
            APP_AGENT_TOKEN="${fallback_token}"
        fi
    fi
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -u|--user)
            TARGET_USER="${2:-}"
            TARGET_EXPLICIT=1
            shift 2
            ;;
        -g|--group)
            TARGET_GROUP="${2:-}"
            TARGET_EXPLICIT=1
            shift 2
            ;;
        -s|--server)
            APP_AGENT_SERVER="${2:-}"
            SERVER_EXPLICIT=1
            shift 2
            ;;
        -t|--token)
            APP_AGENT_TOKEN="${2:-}"
            TOKEN_EXPLICIT=1
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

if [[ "${TARGET_EXPLICIT}" -eq 0 && -z "${TARGET_USER}" && -z "${TARGET_GROUP}" ]]; then
    apply_legacy_defaults_for_deploy_agent
fi

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
